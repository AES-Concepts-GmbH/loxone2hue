package hue

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGetSensorsDoesNotBlockCacheReaders pins the lock scope of GetSensors.
// It used to hold the write lock across all seven sensor requests, so every
// cache reader — including the ones the event stream calls per incoming event —
// stalled for the full duration of the refresh.
func TestGetSensorsDoesNotBlockCacheReaders(t *testing.T) {
	const delay = 200 * time.Millisecond

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := NewClient(strings.TrimPrefix(srv.URL, "https://"), "key")
	defer c.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := c.GetSensors(); err != nil {
			t.Errorf("GetSensors: %v", err)
		}
	}()

	// Let the device lookup finish so GetSensors is inside the endpoint loop.
	time.Sleep(delay + 50*time.Millisecond)

	start := time.Now()
	c.GetSensorName("some-sensor")
	blocked := time.Since(start)

	<-done

	// A cache read is a map lookup; anything near the request delay means it
	// waited on the refresh.
	if blocked > delay/2 {
		t.Errorf("cache read blocked for %v while GetSensors was fetching", blocked)
	}
}

// TestGetSensorsPopulatesCache guards against the lock rework dropping the
// cache update entirely.
func TestGetSensorsPopulatesCache(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clip/v2/resource/device":
			w.Write([]byte(`{"data":[{"id":"dev-1","metadata":{"name":"Flur"}}]}`))
		case "/clip/v2/resource/motion":
			w.Write([]byte(`{"data":[{"id":"sensor-1","owner":{"rid":"dev-1","rtype":"device"},` +
				`"enabled":true,"motion":{"motion":true,"motion_valid":true}}]}`))
		default:
			w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer srv.Close()

	c := NewClient(strings.TrimPrefix(srv.URL, "https://"), "key")
	defer c.Close()

	sensors, err := c.GetSensors()
	if err != nil {
		t.Fatalf("GetSensors: %v", err)
	}
	if len(sensors) != 1 {
		t.Fatalf("sensors = %d, want 1", len(sensors))
	}

	cached := c.GetSensor("sensor-1")
	if cached == nil {
		t.Fatal("sensor was not written to the cache")
	}
	if want := "Flur Motion"; cached.Name != want {
		t.Errorf("name = %q, want %q", cached.Name, want)
	}
	if got := c.GetSensorName("sensor-1"); got != "Flur Motion" {
		t.Errorf("GetSensorName = %q", got)
	}
}

// TestGetSensorsIncludesRoomAndHomeSensors covers the bridge's aggregated
// sensors: grouped_motion and grouped_light_level are owned by a room or by
// bridge_home instead of a device, so their names come from the room list
// (fetched here, not from the group cache, which may still be empty) and
// their scope says what they report on.
func TestGetSensorsIncludesRoomAndHomeSensors(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clip/v2/resource/device":
			w.Write([]byte(`{"data":[{"id":"dev-1","metadata":{"name":"Flur Sensor"}}]}`))
		case "/clip/v2/resource/room":
			w.Write([]byte(`{"data":[{"id":"room-1","metadata":{"name":"Wohnzimmer"}}]}`))
		case "/clip/v2/resource/motion":
			w.Write([]byte(`{"data":[{"id":"motion-1","owner":{"rid":"dev-1","rtype":"device"},` +
				`"enabled":true,"motion":{"motion":false,"motion_valid":true}}]}`))
		case "/clip/v2/resource/grouped_motion":
			w.Write([]byte(`{"data":[` +
				`{"id":"gm-room","owner":{"rid":"room-1","rtype":"room"},"enabled":true,` +
				`"motion":{"motion_report":{"changed":"2026-10-12T21:44:39.707Z","motion":true}}},` +
				`{"id":"gm-home","owner":{"rid":"home-1","rtype":"bridge_home"},"enabled":true,` +
				`"motion":{"motion_report":{"changed":"2026-10-12T21:44:39.707Z","motion":false}}}]}`))
		case "/clip/v2/resource/grouped_light_level":
			w.Write([]byte(`{"data":[{"id":"gl-room","owner":{"rid":"room-1","rtype":"room"},"enabled":true,` +
				`"light":{"light_level_report":{"changed":"2026-10-12T21:44:39.707Z","light_level":20001}}}]}`))
		default:
			// Older bridges do not know the grouped types; everything else is empty.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors":[{"description":"Not Found"}]}`))
		}
	}))
	defer srv.Close()

	c := NewClient(strings.TrimPrefix(srv.URL, "https://"), "key")
	defer c.Close()

	sensors, err := c.GetSensors()
	if err != nil {
		t.Fatalf("GetSensors: %v", err)
	}
	byID := map[string]int{}
	for i, s := range sensors {
		byID[s.ID] = i
	}
	if len(sensors) != 4 {
		t.Fatalf("sensors = %d, want 4 (%v)", len(sensors), byID)
	}

	room := sensors[byID["gm-room"]]
	if room.Name != "Wohnzimmer Bewegung" || room.Owner != "Wohnzimmer" || room.Scope != "room" {
		t.Errorf("room motion = %q/%q/%q, want Wohnzimmer Bewegung / Wohnzimmer / room", room.Name, room.Owner, room.Scope)
	}
	if room.State.Motion == nil || !*room.State.Motion || room.State.LastUpdated == "" {
		t.Errorf("room motion state not taken from motion_report: %+v", room.State)
	}

	home := sensors[byID["gm-home"]]
	if home.Name != "Zuhause Bewegung" || home.Scope != "home" {
		t.Errorf("home motion = %q/%q, want Zuhause Bewegung / home", home.Name, home.Scope)
	}

	light := sensors[byID["gl-room"]]
	if light.Name != "Wohnzimmer Helligkeit" || light.Scope != "room" {
		t.Errorf("room light = %q/%q, want Wohnzimmer Helligkeit / room", light.Name, light.Scope)
	}
	if light.State.LightLevel == nil || *light.State.LightLevel != 20001 {
		t.Errorf("light_level = %v, want raw 20001", light.State.LightLevel)
	}
	if light.State.Lux == nil || *light.State.Lux != 100 {
		t.Errorf("lux = %v, want 100 (10^((20001-1)/10000))", light.State.Lux)
	}

	device := sensors[byID["motion-1"]]
	if device.Name != "Flur Sensor Motion" || device.Scope != "device" {
		t.Errorf("device motion = %q/%q, want Flur Sensor Motion / device", device.Name, device.Scope)
	}
}
