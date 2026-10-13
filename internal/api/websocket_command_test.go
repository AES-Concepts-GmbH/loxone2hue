package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

// TestHTTPCommandAnswersHonestly covers the shared command executor through
// the GET /ws?cmd= path. Before the two transports were merged, a SET on a
// scene mapping did nothing and still answered "ok", a typo in the Loxone ID
// was sent to the bridge as a light ID, and STATUS existed only on the
// WebSocket side.
func TestHTTPCommandAnswersHonestly(t *testing.T) {
	if _, err := config.Load(t.TempDir() + "/config.yaml"); err != nil {
		t.Fatalf("load config: %v", err)
	}

	bridgeAddr, stopBridge := fakeBridge(t, `[]`)
	defer stopBridge()

	hueClient := hue.NewClient("", "")
	hueClient.Configure(bridgeAddr, "test-key")
	if _, err := hueClient.GetLights(); err != nil {
		t.Fatalf("GetLights: %v", err)
	}

	mappings := loxone.NewMappingManager()
	mappings.Load([]models.Mapping{
		{ID: "m1", LoxoneID: "wz_decke", HueID: "light-1", HueType: "light", Enabled: true},
		{ID: "m2", LoxoneID: "abend", HueID: "scene-1", HueType: "scene", Enabled: true},
		{ID: "m3", LoxoneID: "wz", HueID: "room-1", HueType: "group", Enabled: true},
	})

	hub := NewWebSocketHub(hueClient, mappings, loxone.NewUDPSender(), loxone.NewHTTPSender())

	run := func(cmd string) (int, map[string]interface{}) {
		t.Helper()
		rec := httptest.NewRecorder()
		hub.handleHTTPCommand(rec, httptest.NewRequest("GET", "/ws", nil), cmd)
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: response is not JSON: %s", cmd, rec.Body.String())
		}
		return rec.Code, body
	}

	// Typo in the Loxone ID: 404, not a bridge call with a bogus light ID.
	if code, body := run("SET wz_deke ON"); code != http.StatusNotFound {
		t.Errorf("unknown target: status = %d, body %v, want 404", code, body)
	}

	// SET on a scene mapping used to be a silent no-op with status ok.
	if code, body := run("SET abend ON"); code != http.StatusBadRequest {
		t.Errorf("set on scene mapping: status = %d, body %v, want 400", code, body)
	}

	// Unknown mood: 404.
	if code, _ := run("MOOD wz 7"); code != http.StatusNotFound {
		t.Errorf("unknown mood: status = %d, want 404", code)
	}

	// STATUS now works over HTTP too and carries the converted LightState.
	code, body := run("GET wz_decke STATUS")
	if code != http.StatusOK {
		t.Fatalf("status query: status = %d, body %v", code, body)
	}
	state, ok := body["state"].(map[string]interface{})
	if !ok {
		t.Fatalf("status query: no state object in %v", body)
	}
	if on, ok := state["on"].(bool); !ok || !on {
		t.Errorf("status query: state.on = %v, want true", state["on"])
	}
	if body["hue_id"] != "light-1" || body["hue_type"] != "light" {
		t.Errorf("status query: resolved to %v/%v, want light-1/light", body["hue_id"], body["hue_type"])
	}

	// STATUS on a group mapping is a client error, not a bridge error.
	if code, _ := run("GET wz STATUS"); code != http.StatusBadRequest {
		t.Errorf("status on group: status = %d, want 400", code)
	}
}

// TestCacheReadsAreCopies pins the data race fixed in 4.1: a handler that
// JSON-encodes a light while an event updates the cache must not share the
// object. Run with -race.
func TestCacheReadsAreCopies(t *testing.T) {
	bridgeAddr, stopBridge := fakeBridge(t, `[]`)
	defer stopBridge()

	hueClient := hue.NewClient("", "")
	hueClient.Configure(bridgeAddr, "test-key")
	lights, err := hueClient.GetLights()
	if err != nil {
		t.Fatalf("GetLights: %v", err)
	}

	lights[0].State.On = false
	lights[0].Name = "changed by caller"

	snap, ok := hueClient.LightSnapshot("light-1")
	if !ok {
		t.Fatal("light missing from cache")
	}
	if !snap.State.On || snap.Name != "WZ Decke" {
		t.Errorf("cache was modified through the returned object: %+v", snap)
	}

	light, err := hueClient.GetLight("light-1")
	if err != nil {
		t.Fatalf("GetLight: %v", err)
	}
	if light == lights[0] {
		t.Error("GetLight returned the same pointer as GetLights")
	}
}
