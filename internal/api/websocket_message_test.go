package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"github.com/gorilla/websocket"
)

// TestWebSocketCommands drives the WebSocket transport end to end: text and
// JSON commands, SCENE and MOOD against the fake bridge, a STATUS query that
// answers with the light state, and the error replies for garbage and for
// unknown targets.
func TestWebSocketCommands(t *testing.T) {
	bridge := newBridgeRecorder(t)
	hueClient := hue.NewClient("", "")
	hueClient.Configure(bridge.addr(), "test-key")
	if _, err := hueClient.GetLights(); err != nil {
		t.Fatalf("GetLights: %v", err)
	}
	if _, err := hueClient.GetGroups(); err != nil {
		t.Fatalf("GetGroups: %v", err)
	}

	mappings := loxone.NewMappingManager()
	mappings.Load([]models.Mapping{
		{ID: "m1", LoxoneID: "wz_decke", HueID: "light-1", HueType: "light", Enabled: true},
		{ID: "m2", LoxoneID: "wz", HueID: "room-1", HueType: "group", Enabled: true},
		{ID: "m3", LoxoneID: "wz_mood_0", HueID: "room-1", HueType: "group", Enabled: true},
		{ID: "m4", LoxoneID: "wz_mood_1", HueID: "scene-1", HueType: "scene", Enabled: true},
		{ID: "m5", LoxoneID: "abend", HueID: "scene-1", HueType: "scene", Enabled: true},
	})

	hub := NewWebSocketHub(hueClient, mappings, loxone.NewUDPSender(), loxone.NewHTTPSender())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"?type=loxone&id=test", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	send := func(msg string) map[string]interface{} {
		t.Helper()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatalf("write %q: %v", msg, err)
		}
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("no reply to %q: %v", msg, err)
		}
		var reply map[string]interface{}
		if err := json.Unmarshal(data, &reply); err != nil {
			t.Fatalf("reply to %q is not JSON: %s", msg, data)
		}
		return reply
	}

	if r := send("SET wz_decke ON"); r["type"] != "ack" || r["target"] != "wz_decke" {
		t.Errorf("SET text: %v", r)
	}
	if sent, ok := bridge.put("/clip/v2/resource/light/light-1"); !ok || !strings.Contains(sent, `"on":true`) {
		t.Errorf("bridge received %q for SET ON", sent)
	}

	if r := send(`{"type":"command","target":"wz","action":"set","params":{"brightness":30}}`); r["type"] != "ack" {
		t.Errorf("SET json: %v", r)
	}
	if sent, ok := bridge.put("/clip/v2/resource/grouped_light/gl-1"); !ok || !strings.Contains(sent, `"brightness":30`) {
		t.Errorf("bridge received %q for the group command", sent)
	}

	if r := send("SCENE abend"); r["type"] != "ack" {
		t.Errorf("SCENE: %v", r)
	}
	if sent, ok := bridge.put("/clip/v2/resource/scene/scene-1"); !ok || !strings.Contains(sent, `"active"`) {
		t.Errorf("bridge received %q for SCENE", sent)
	}

	if r := send("MOOD wz 1"); r["type"] != "ack" {
		t.Errorf("MOOD 1: %v", r)
	}
	if r := send("MOOD wz 0"); r["type"] != "ack" {
		t.Errorf("MOOD 0: %v", r)
	}
	if sent, _ := bridge.put("/clip/v2/resource/grouped_light/gl-1"); !strings.Contains(sent, `"on":false`) {
		t.Errorf("MOOD 0 did not switch the group off: %q", sent)
	}

	r := send("GET wz_decke STATUS")
	if r["type"] != "status" || r["device"] != "wz_decke" {
		t.Fatalf("STATUS: %v", r)
	}
	if state, ok := r["state"].(map[string]interface{}); !ok || state["on"] != true {
		t.Errorf("STATUS state = %v", r["state"])
	}

	// MOOD 0 on a light mapping switches that light off.
	if r := send("MOOD wz_decke 0"); r["type"] != "ack" {
		t.Errorf("MOOD 0 on a light: %v", r)
	}
	if sent, _ := bridge.put("/clip/v2/resource/light/light-1"); !strings.Contains(sent, `"on":false`) {
		t.Errorf("MOOD 0 did not switch the light off: %q", sent)
	}

	for msg, want := range map[string]string{
		"NOT A COMMAND":  "invalid command format",
		"SET wz_deke ON": "unknown target",
		"SET abend ON":   "cannot be set",
		"SCENE nirgends": "unknown scene",
		"MOOD wz 7":      "no mapping found for mood 7",
		"GET wz STATUS":  "only available for light",
	} {
		r := send(msg)
		if r["type"] != "error" || !strings.Contains(r["message"].(string), want) {
			t.Errorf("%q: reply %v, want error containing %q", msg, r, want)
		}
	}
}

// TestButtonEventCodes pins the numeric codes Loxone receives for button events.
func TestButtonEventCodes(t *testing.T) {
	for event, want := range map[string]int{
		"initial_press": 0, "repeat": 1, "short_release": 2, "long_release": 3, "long_press": 4, "unknown": -1,
	} {
		if got := buttonEventToCode(event); got != want {
			t.Errorf("buttonEventToCode(%q) = %d, want %d", event, got, want)
		}
	}
}

// TestAuthHelpers covers the two small helpers used by the startup log.
func TestAuthHelpers(t *testing.T) {
	withAuth(t, "", "")
	if authStatus() != "disabled" {
		t.Errorf("authStatus = %q, want disabled", authStatus())
	}
	withAuth(t, "admin", "geheim123")
	if authStatus() != "enabled" {
		t.Errorf("authStatus = %q, want enabled", authStatus())
	}
	for host, want := range map[string]bool{"127.0.0.1": true, "::1": true, "LocalHost": true, "0.0.0.0": false, "192.168.1.5": false} {
		if isLoopback(host) != want {
			t.Errorf("isLoopback(%q) = %v, want %v", host, !want, want)
		}
	}
}
