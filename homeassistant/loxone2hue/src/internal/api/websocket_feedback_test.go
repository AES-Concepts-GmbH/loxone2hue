package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sbeyeler/loxone2hue/internal/config"
	"github.com/sbeyeler/loxone2hue/internal/hue"
	"github.com/sbeyeler/loxone2hue/internal/loxone"
	"github.com/sbeyeler/loxone2hue/internal/models"
)

// fakeBridge serves the REST resources the client caches plus a one-shot SSE
// stream carrying events. It mirrors a room "Wohnzimmer" with one light and
// its grouped_light service.
func fakeBridge(t *testing.T, events string) (addr string, stop func()) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/clip/v2/resource/light", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"light-1","type":"light","owner":{"rid":"dev-1","rtype":"device"},`+
			`"metadata":{"name":"WZ Decke"},"on":{"on":true},"dimming":{"brightness":80}}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/room", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"room-1","type":"room","metadata":{"name":"Wohnzimmer"},`+
			`"children":[{"rid":"dev-1","rtype":"device"}],"services":[{"rid":"gl-1","rtype":"grouped_light"}]}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/zone", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	})
	mux.HandleFunc("/clip/v2/resource/grouped_light", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"gl-1","type":"grouped_light","owner":{"rid":"room-1","rtype":"room"},`+
			`"on":{"on":true},"dimming":{"brightness":80}}]}`)
	})
	mux.HandleFunc("/eventstream/clip/v2", func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", events)
		flusher.Flush()
		<-r.Context().Done()
	})

	srv := httptest.NewTLSServer(mux)
	return strings.TrimPrefix(srv.URL, "https://"), srv.Close
}

// slowMiniserver records HTTP feedback requests and answers each one only
// after delay, standing in for a Miniserver on a congested link.
type slowMiniserver struct {
	mu    sync.Mutex
	paths []string
	delay time.Duration
}

func (m *slowMiniserver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	time.Sleep(m.delay)
	m.mu.Lock()
	m.paths = append(m.paths, r.URL.Path)
	m.mu.Unlock()
}

func (m *slowMiniserver) snapshot() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.paths...)
}

// TestBridgeEventsReachUIWithoutWaitingForLoxone covers three regressions at
// once, all on the single event pipeline between bridge and consumers:
//
//   - the Web UI receives the converted LightState (on is a bool), not the raw
//     CLIP event where on was an always-truthy object;
//   - a room's state comes from its grouped_light event and is pushed as
//     group_update, instead of a single lamp's state being reported as the
//     room's;
//   - HTTP feedback to a slow Miniserver no longer delays the UI messages,
//     because it runs on its own worker.
//
// Run with -race.
func TestBridgeEventsReachUIWithoutWaitingForLoxone(t *testing.T) {
	const feedbackDelay = 800 * time.Millisecond

	if _, err := config.Load(t.TempDir() + "/config.yaml"); err != nil {
		t.Fatalf("load config: %v", err)
	}

	// One SSE event: the lamp and, as the bridge does, the room's grouped_light.
	bridgeAddr, stopBridge := fakeBridge(t, `[{"creationtime":"2024-01-01T00:00:00Z","type":"update","data":[`+
		`{"id":"light-1","type":"light","on":{"on":false}},`+
		`{"id":"gl-1","type":"grouped_light","owner":{"rid":"room-1","rtype":"room"},"on":{"on":false}}]}]`)
	defer stopBridge()

	ms := &slowMiniserver{delay: feedbackDelay}
	msSrv := httptest.NewServer(ms)
	defer msSrv.Close()

	hueClient := hue.NewClient("", "")
	hueClient.Configure(bridgeAddr, "test-key")
	if _, err := hueClient.GetLights(); err != nil {
		t.Fatalf("GetLights: %v", err)
	}
	if _, err := hueClient.GetGroups(); err != nil {
		t.Fatalf("GetGroups: %v", err)
	}

	mappings := loxone.NewMappingManager()
	mappings.Load([]models.Mapping{
		{ID: "m1", LoxoneID: "wz_decke", HueID: "light-1", HueType: "light", Enabled: true, MiniserverID: "ms1"},
		{ID: "m2", LoxoneID: "wohnzimmer", HueID: "room-1", HueType: "group", Enabled: true, MiniserverID: "ms1"},
	})

	httpSender := loxone.NewHTTPSender()
	defer httpSender.Close()
	httpSender.Configure([]config.MiniserverConfig{
		{ID: "ms1", Name: "Test", HTTPEnabled: true, HTTPURL: msSrv.URL},
	})

	hub := NewWebSocketHub(hueClient, mappings, loxone.NewUDPSender(), httpSender)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	wsSrv := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer wsSrv.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(wsSrv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Only now start the stream, so the client above is registered when the
	// events arrive.
	go hueClient.RunEventStream(ctx)

	var (
		gotStatus, gotGroup bool
		first               time.Time
	)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for !(gotStatus && gotGroup) {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read (status=%v group=%v): %v", gotStatus, gotGroup, err)
		}
		if first.IsZero() {
			first = time.Now()
		}

		var msg struct {
			Type   string          `json:"type"`
			Device string          `json:"device"`
			State  json.RawMessage `json:"state"`
			Group  *models.Group   `json:"group"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}

		switch msg.Type {
		case "status":
			var state models.LightState
			if err := json.Unmarshal(msg.State, &state); err != nil {
				t.Fatalf("status state is not a LightState: %s", msg.State)
			}
			if msg.Device != "light-1" || state.On {
				t.Errorf("status = %s, want light-1 off", raw)
			}
			if state.Brightness != 80 {
				t.Errorf("status brightness = %v, want the cached 80", state.Brightness)
			}
			gotStatus = true
		case "group_update":
			if msg.Group == nil || msg.Group.ID != "room-1" || msg.Group.State.AnyOn {
				t.Errorf("group_update = %s, want room-1 with any_on false", raw)
			}
			gotGroup = true
		}
	}

	// Both messages must arrive back to back. With feedback on the event
	// goroutine the second one waited for at least one slow HTTP round trip.
	if spread := time.Since(first); spread > feedbackDelay/2 {
		t.Errorf("UI messages spread over %v, feedback is blocking the event loop", spread)
	}

	// The worker still delivers: one update per mapping, and the lamp's event
	// no longer fans out to the room.
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) && len(ms.snapshot()) < 2 {
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(feedbackDelay + 200*time.Millisecond) // catch any extra send

	got := ms.snapshot()
	want := map[string]bool{"/dev/sps/io/wz_decke_on/0": true, "/dev/sps/io/wohnzimmer_on/0": true}
	if len(got) != len(want) {
		t.Fatalf("feedback requests = %v, want exactly %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected feedback request %s", p)
		}
	}
}

// TestQueueFeedbackNeverBlocks pins the contract forwardHueEvents relies on:
// with no worker draining the queue, queueing returns immediately and the
// overflow is counted instead of stalling the bridge's event channel.
func TestQueueFeedbackNeverBlocks(t *testing.T) {
	hub := newTestHub()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < feedbackQueueSize+10; i++ {
			hub.queueFeedback(hue.Event{Type: "light", ID: "light-1"})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("queueFeedback blocked on a full queue")
	}

	if got := hub.DroppedFeedback(); got != 10 {
		t.Errorf("DroppedFeedback = %d, want 10", got)
	}
}
