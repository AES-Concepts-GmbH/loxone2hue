package hue

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sbeyeler/loxone2hue/internal/models"
)

const testEventPayload = `[{"creationtime":"2024-01-01T00:00:00Z","type":"update",` +
	`"data":[{"id":"light-1","type":"light","on":{"on":true}}]}]`

// fakeBridge serves a HUE-style SSE stream and counts how often it was
// connected to. The returned address has no scheme, matching what Configure
// expects as a bridge IP.
func fakeBridge() (addr string, conns *int64, stop func()) {
	var n int64

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eventstream/clip/v2" {
			http.NotFound(w, r)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		atomic.AddInt64(&n, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		for {
			fmt.Fprintf(w, "data: %s\n\n", testEventPayload)
			flusher.Flush()

			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
	}))

	return strings.TrimPrefix(srv.URL, "https://"), &n, srv.Close
}

// waitForConns waits until the counter reaches want.
func waitForConns(conns *int64, want int64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(conns) >= want {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// TestEventStreamStartsAfterConfigure covers pairing through the Web UI: the
// gateway starts with no bridge, and the stream has to come up on its own once
// pairing stores a bridge. Previously the stream was only ever started at boot,
// so feedback to Loxone stayed dead until the next restart.
func TestEventStreamStartsAfterConfigure(t *testing.T) {
	addr, conns, stop := fakeBridge()
	defer stop()

	c := NewClient("", "")
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.RunEventStream(ctx)

	// Unconfigured: the supervisor must idle rather than connect anywhere.
	time.Sleep(200 * time.Millisecond)
	if got := atomic.LoadInt64(conns); got != 0 {
		t.Fatalf("connected %d times while unconfigured, want 0", got)
	}

	// This is what PairBridge does after a successful pairing.
	c.Configure(addr, "test-key")

	select {
	case ev := <-c.Events():
		if ev.ID != "light-1" {
			t.Fatalf("unexpected event id %q", ev.ID)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no event after Configure: stream did not start after pairing")
	}
}

// TestEventStreamSwitchesBridgeOnReconfigure covers restoring a backup that
// carries a different bridge: the running stream has to move to the new one.
func TestEventStreamSwitchesBridgeOnReconfigure(t *testing.T) {
	addrA, connsA, stopA := fakeBridge()
	defer stopA()
	addrB, connsB, stopB := fakeBridge()
	defer stopB()

	c := NewClient("", "")
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.RunEventStream(ctx)

	c.Configure(addrA, "key-a")
	if !waitForConns(connsA, 1, 10*time.Second) {
		t.Fatal("stream never connected to the first bridge")
	}

	// This is what RestoreBackup does.
	c.Configure(addrB, "key-b")
	if !waitForConns(connsB, 1, 10*time.Second) {
		t.Fatal("stream did not move to the restored bridge")
	}

	if _, key := c.endpoint(); key != "key-b" {
		t.Fatalf("application key = %q, want key-b", key)
	}
}

// TestEventStreamBacksOffOnCleanClose guards the reconnect loop: a bridge that
// closes the stream immediately must not spin the supervisor at full speed.
func TestEventStreamBacksOffOnCleanClose(t *testing.T) {
	var conns int64

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&conns, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK) // then return straight away: clean EOF
	}))
	defer srv.Close()

	c := NewClient(strings.TrimPrefix(srv.URL, "https://"), "key")
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.RunEventStream(ctx)

	time.Sleep(2 * time.Second)

	// With a 5s backoff a single attempt is expected; without any backoff this
	// counter runs into the thousands.
	if got := atomic.LoadInt64(&conns); got > 3 {
		t.Fatalf("reconnected %d times in 2s, expected backoff", got)
	}
}

// TestGroupedLightEventUpdatesGroup covers room feedback: the bridge reports
// the aggregated state of a room on its grouped_light service, and that event
// has to land on the room in the cache. Previously grouped_light events were
// ignored and the room's state was derived from whichever single lamp
// changed last.
func TestGroupedLightEventUpdatesGroup(t *testing.T) {
	c := NewClient("", "")

	room := convertHueRoom(hueRoom{
		ID: "room-1",
		Services: []struct {
			RID   string `json:"rid"`
			RType string `json:"rtype"`
		}{{RID: "gl-1", RType: "grouped_light"}},
	}, nil)
	room.Name = "Wohnzimmer"
	room.State.AnyOn = true
	room.State.Brightness = 80

	c.mu.Lock()
	c.storeGroupLocked(room)
	c.mu.Unlock()

	if got := c.GroupIDForGroupedLight("gl-1"); got != "room-1" {
		t.Fatalf("GroupIDForGroupedLight = %q, want room-1", got)
	}

	c.processEvent(`[{"creationtime":"2024-01-01T00:00:00Z","type":"update",` +
		`"data":[{"id":"gl-1","type":"grouped_light","on":{"on":false},"dimming":{"brightness":10}}]}]`)

	snap, ok := c.GroupSnapshot("room-1")
	if !ok {
		t.Fatal("room missing from cache")
	}
	if snap.State.AnyOn {
		t.Error("any_on still true after grouped_light off event")
	}
	if snap.State.Brightness != 10 {
		t.Errorf("brightness = %v, want 10", snap.State.Brightness)
	}

	select {
	case ev := <-c.Events():
		if ev.Type != "grouped_light" || ev.ID != "gl-1" {
			t.Errorf("event = %s/%s, want grouped_light/gl-1", ev.Type, ev.ID)
		}
	default:
		t.Error("grouped_light event was not forwarded")
	}
}

// TestDroppedEventsAreCounted pins the previously silent "channel full, skip"
// path: lost events are now visible through the counter and the health
// endpoint.
func TestDroppedEventsAreCounted(t *testing.T) {
	c := NewClient("", "")

	// Nobody reads Events(), so everything beyond the buffer is dropped.
	for i := 0; i < cap(c.eventChan)+5; i++ {
		c.processEvent(testEventPayload)
	}

	if got := c.DroppedEvents(); got != 5 {
		t.Errorf("DroppedEvents = %d, want 5", got)
	}
}

// TestGroupedSensorEventsUpdateCache makes sure the aggregated room sensors
// take the same event path as their per-device counterparts: a grouped_motion
// event flips the cached motion flag, a grouped_light_level event stores the
// raw level and its lux value.
func TestGroupedSensorEventsUpdateCache(t *testing.T) {
	c := NewClient("", "")

	off := false
	c.mu.Lock()
	c.sensors["gm-room"] = &models.Sensor{ID: "gm-room", Type: "grouped_motion", State: models.SensorState{Motion: &off, Enabled: true}}
	c.sensors["gl-room"] = &models.Sensor{ID: "gl-room", Type: "grouped_light_level", State: models.SensorState{Enabled: true}}
	c.mu.Unlock()

	c.processEvent(`[{"creationtime":"2026-10-12T21:44:39Z","type":"update","data":[` +
		`{"id":"gm-room","type":"grouped_motion","owner":{"rid":"room-1","rtype":"room"},` +
		`"motion":{"motion_report":{"changed":"2026-10-12T21:44:39.707Z","motion":true}}},` +
		`{"id":"gl-room","type":"grouped_light_level","owner":{"rid":"room-1","rtype":"room"},` +
		`"light":{"light_level_report":{"changed":"2026-10-12T21:44:39.707Z","light_level":30001}}}]}]`)

	motion := c.GetSensor("gm-room")
	if motion == nil || motion.State.Motion == nil || !*motion.State.Motion {
		t.Errorf("grouped_motion event did not set motion: %+v", motion)
	}
	light := c.GetSensor("gl-room")
	if light == nil || light.State.LightLevel == nil || *light.State.LightLevel != 30001 || light.State.Lux == nil || *light.State.Lux != 1000 {
		t.Errorf("grouped_light_level event did not set level/lux: %+v", light)
	}

	// Both must have been forwarded as sensor events.
	got := 0
	for i := 0; i < 2; i++ {
		select {
		case ev := <-c.Events():
			if !IsSensorType(ev.Type) {
				t.Errorf("event type %s is not recognised as a sensor", ev.Type)
			}
			got++
		default:
		}
	}
	if got != 2 {
		t.Errorf("forwarded %d events, want 2", got)
	}
}
