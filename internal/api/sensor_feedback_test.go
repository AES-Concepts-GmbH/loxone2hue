package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/sbeyeler/loxone2hue/internal/config"
	"github.com/sbeyeler/loxone2hue/internal/hue"
	"github.com/sbeyeler/loxone2hue/internal/loxone"
	"github.com/sbeyeler/loxone2hue/internal/models"
)

// TestSensorFeedbackForRoomSensors covers the Loxone side of the aggregated
// sensors: a grouped_motion event reaches the Miniserver under the same
// "motion" property as a device sensor, and light readings go out twice, as
// the bridge's raw light_level and as lux.
func TestSensorFeedbackForRoomSensors(t *testing.T) {
	if _, err := config.Load(t.TempDir() + "/config.yaml"); err != nil {
		t.Fatalf("load config: %v", err)
	}

	var mu sync.Mutex
	var paths []string
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
	}))
	defer ms.Close()

	mappings := loxone.NewMappingManager()
	mappings.Load([]models.Mapping{
		{ID: "m1", LoxoneID: "wz_bewegung", HueID: "gm-room", HueType: "sensor", Enabled: true, MiniserverID: "ms1"},
		{ID: "m2", LoxoneID: "wz_licht", HueID: "gl-room", HueType: "sensor", Enabled: true, MiniserverID: "ms1"},
	})

	httpSender := loxone.NewHTTPSender()
	defer httpSender.Close()
	httpSender.Configure([]config.MiniserverConfig{{ID: "ms1", Name: "Test", HTTPEnabled: true, HTTPURL: ms.URL}})

	hub := NewWebSocketHub(hue.NewClient("", ""), mappings, loxone.NewUDPSender(), httpSender)

	eventFrom := func(t *testing.T, raw string) hue.Event {
		t.Helper()
		var item hue.EventItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			t.Fatalf("decode event item: %v", err)
		}
		return hue.Event{Type: item.Type, ID: item.ID, Data: item}
	}

	hub.sendFeedback(eventFrom(t, `{"id":"gm-room","type":"grouped_motion",`+
		`"motion":{"motion_report":{"changed":"2026-10-12T21:44:39.707Z","motion":true}}}`))
	hub.sendFeedback(eventFrom(t, `{"id":"gl-room","type":"grouped_light_level",`+
		`"light":{"light_level_report":{"changed":"2026-10-12T21:44:39.707Z","light_level":20001}}}`))

	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"/dev/sps/io/wz_bewegung_motion/1",
		"/dev/sps/io/wz_licht_light_level/20001",
		"/dev/sps/io/wz_licht_lux/100",
	}
	if len(paths) != len(want) {
		t.Fatalf("feedback requests = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("request %d = %s, want %s", i, paths[i], want[i])
		}
	}
}
