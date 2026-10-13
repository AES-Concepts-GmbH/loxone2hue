package hue

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

// apiBridge is a fake CLIP v2 bridge that records PUTs and lets a test set
// the pairing answer.
type apiBridge struct {
	srv          *httptest.Server
	mu           sync.Mutex
	puts         map[string]string
	pairResponse string
	lightList    string
}

func newAPIBridge(t *testing.T) *apiBridge {
	t.Helper()
	b := &apiBridge{
		puts:         make(map[string]string),
		pairResponse: `[{"success":{"username":"paired-key"}}]`,
		lightList: `{"data":[{"id":"light-1","metadata":{"name":"Decke","archetype":"ceiling"},"product_data":{"model_id":"LCA001","product_name":"Hue color"},` +
			`"on":{"on":true},"dimming":{"brightness":75.5},"color_temperature":{"mirek":250,"mirek_valid":true},"color":{"xy":{"x":0.3,"y":0.3},"gamut_type":"C"}},` +
			`{"id":"light-2","metadata":{"name":"Steckdose","archetype":"plug"},"on":{"on":false},"color_temperature":{"mirek":0,"mirek_valid":false}}]}`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		fmt.Fprint(w, b.pairResponse)
	})
	mux.HandleFunc("/clip/v2/resource/bridge", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"bridge-1","bridge_id":"001788FFFE000001"}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/light", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, b.lightList)
	})
	mux.HandleFunc("/clip/v2/resource/light/light-3", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"light-3","metadata":{"name":"Neu"},"on":{"on":true},"dimming":{"brightness":10}}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/light/light-9", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	})
	mux.HandleFunc("/clip/v2/resource/grouped_light", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"gl-1","owner":{"rid":"room-1","rtype":"room"}},{"id":"gl-2","owner":{"rid":"zone-1","rtype":"zone"}}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/scene", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"scene-1","metadata":{"name":"Abend"},"group":{"rid":"room-1","rtype":"room"}}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			b.mu.Lock()
			b.puts[r.URL.Path] = string(body)
			b.mu.Unlock()
			fmt.Fprint(w, `{"data":[]}`)
			return
		}
		http.Error(w, `{"errors":[{"description":"not found"}]}`, http.StatusNotFound)
	})
	b.srv = httptest.NewTLSServer(mux)
	t.Cleanup(b.srv.Close)
	return b
}

func (b *apiBridge) client() *Client {
	return NewClient(strings.TrimPrefix(b.srv.URL, "https://"), "key")
}

func (b *apiBridge) put(path string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.puts[path]
}

// TestPair covers the three answers a bridge gives to the pairing call.
func TestPair(t *testing.T) {
	b := newAPIBridge(t)
	c := NewClient(strings.TrimPrefix(b.srv.URL, "https://"), "")
	if c.IsConfigured() {
		t.Fatal("client without key reports configured")
	}

	key, err := c.Pair("Loxone2HUE", "test")
	if err != nil || key != "paired-key" {
		t.Fatalf("Pair = %q, %v", key, err)
	}
	if !c.IsConfigured() {
		t.Error("client not configured after pairing")
	}

	b.mu.Lock()
	b.pairResponse = `[{"error":{"type":101,"description":"link button not pressed"}}]`
	b.mu.Unlock()
	if _, err := c.Pair("Loxone2HUE", "test"); err == nil || !strings.Contains(err.Error(), "link button") {
		t.Errorf("error 101 = %v, want link button hint", err)
	}

	b.mu.Lock()
	b.pairResponse = `{"not":"a list"}`
	b.mu.Unlock()
	if _, err := c.Pair("Loxone2HUE", "test"); err == nil {
		t.Error("garbage response accepted")
	}

	b.mu.Lock()
	b.pairResponse = `[{"error":"string instead of object"}]`
	b.mu.Unlock()
	if _, err := c.Pair("Loxone2HUE", "test"); err == nil || !strings.Contains(err.Error(), "pairing error") {
		t.Errorf("malformed error object = %v", err)
	}

	// Connection refused: nothing listens on port 1.
	c2 := NewClient("127.0.0.1:1", "")
	if _, err := c2.Pair("Loxone2HUE", "test"); err == nil || !strings.Contains(err.Error(), "connection to bridge failed") {
		t.Errorf("unreachable bridge = %v", err)
	}
}

// TestLightsRoundTrip covers GetLights/GetLight/SetLightState and the light
// conversion: capabilities follow the fields the bridge reports, an invalid
// mirek does not count as colour temperature support, snapshots are copies.
func TestLightsRoundTrip(t *testing.T) {
	b := newAPIBridge(t)
	c := b.client()

	info, err := c.GetBridgeInfo()
	if err != nil || info["data"] == nil {
		t.Errorf("GetBridgeInfo = %v, %v", info, err)
	}

	lights, err := c.GetLights()
	if err != nil || len(lights) != 2 {
		t.Fatalf("GetLights = %d lights, %v", len(lights), err)
	}
	colour, plug := lights[0], lights[1]
	if colour.Name != "Decke" || colour.ProductName != "Hue color" || colour.State.Brightness != 75.5 || colour.State.ColorTemp != 250 {
		t.Errorf("colour light = %+v", colour)
	}
	if !colour.Capabilities.SupportsColor || !colour.Capabilities.SupportsColorTemp || !colour.Capabilities.SupportsDimming {
		t.Errorf("colour capabilities = %+v", colour.Capabilities)
	}
	if colour.State.Color == nil || colour.State.Color.Gamut != "C" {
		t.Errorf("colour xy = %+v", colour.State.Color)
	}
	if plug.Capabilities.SupportsDimming || plug.Capabilities.SupportsColor || plug.Capabilities.SupportsColorTemp {
		t.Errorf("plug capabilities = %+v, want none", plug.Capabilities)
	}

	// Cached light: no request. Unknown light: fetched, converted and cached.
	if l, err := c.GetLight("light-1"); err != nil || l.Name != "Decke" {
		t.Errorf("GetLight cached = %+v, %v", l, err)
	}
	if l, err := c.GetLight("light-3"); err != nil || l.Name != "Neu" || l.State.Brightness != 10 {
		t.Errorf("GetLight uncached = %+v, %v", l, err)
	}
	if snap, ok := c.LightSnapshot("light-3"); !ok || snap.Name != "Neu" {
		t.Error("uncached light was not stored")
	}
	if _, err := c.GetLight("light-9"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("GetLight unknown = %v, want not found", err)
	}

	// Snapshots are copies: changing one must not change the cache.
	snap, _ := c.LightSnapshot("light-1")
	snap.State.Color.XY[0] = 0.9
	snap.Name = "changed"
	again, _ := c.LightSnapshot("light-1")
	if again.Name != "Decke" || again.State.Color.XY[0] != 0.3 {
		t.Errorf("cache changed through a snapshot: %+v", again)
	}
	if _, ok := c.LightSnapshot("nope"); ok {
		t.Error("snapshot of unknown light reported ok")
	}

	on, bri, ct := true, 40.0, 300
	if err := c.SetLightState("light-1", models.DeviceCommand{On: &on, Brightness: &bri, ColorTemp: &ct, Color: &models.Color{XY: [2]float64{0.5, 0.4}}}); err != nil {
		t.Fatalf("SetLightState: %v", err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(b.put("/clip/v2/resource/light/light-1")), &body); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	if body["on"].(map[string]interface{})["on"] != true || body["dimming"].(map[string]interface{})["brightness"] != 40.0 ||
		body["color_temperature"].(map[string]interface{})["mirek"] != 300.0 || body["color"] == nil {
		t.Errorf("PUT body = %v", body)
	}
}

// TestGroupsScenesAndLookups covers SetGroupState with the grouped_light
// lookup fallback, scene listing and activation, and the name lookups.
func TestGroupsScenesAndLookups(t *testing.T) {
	b := newAPIBridge(t)
	c := b.client()

	// No room in the cache: the grouped_light is looked up on the bridge and
	// remembered.
	off := false
	if err := c.SetGroupState("room-1", models.DeviceCommand{On: &off}); err != nil {
		t.Fatalf("SetGroupState: %v", err)
	}
	if got := b.put("/clip/v2/resource/grouped_light/gl-1"); !strings.Contains(got, `"on":false`) {
		t.Errorf("grouped_light PUT = %q", got)
	}
	if c.GroupIDForGroupedLight("gl-1") != "room-1" {
		t.Error("lookup result was not cached")
	}
	if err := c.SetGroupState("room-9", models.DeviceCommand{On: &off}); err == nil || !strings.Contains(err.Error(), "grouped_light not found") {
		t.Errorf("unknown group = %v", err)
	}

	scenes, err := c.GetScenes()
	if err != nil || len(scenes) != 1 || scenes[0].Name != "Abend" || scenes[0].GroupID != "room-1" || scenes[0].Type != "room" {
		t.Fatalf("GetScenes = %+v, %v", scenes, err)
	}
	if err := c.ActivateScene("scene-1"); err != nil {
		t.Fatalf("ActivateScene: %v", err)
	}
	if got := b.put("/clip/v2/resource/scene/scene-1"); !strings.Contains(got, `"action":"active"`) {
		t.Errorf("scene PUT = %q", got)
	}

	if c.GetGroupName("room-1") != "" {
		t.Error("room name known without GetGroups")
	}
	c.mu.Lock()
	c.storeGroupLocked(&models.Group{ID: "room-1", Name: "Wohnzimmer", GroupedLightID: "gl-1"})
	c.mu.Unlock()
	if c.GetGroupName("room-1") != "Wohnzimmer" {
		t.Errorf("GetGroupName = %q", c.GetGroupName("room-1"))
	}
	snap, ok := c.GroupSnapshot("room-1")
	if !ok || snap.Name != "Wohnzimmer" {
		t.Errorf("GroupSnapshot = %+v, %v", snap, ok)
	}
	if _, ok := c.GroupSnapshot("nope"); ok {
		t.Error("snapshot of unknown group reported ok")
	}

	// Setters used during pairing.
	c.SetBridgeIP("10.0.0.7")
	c.SetApplicationKey("")
	if c.BridgeIP() != "10.0.0.7" || c.IsConfigured() {
		t.Errorf("after SetBridgeIP/SetApplicationKey: ip=%q configured=%v", c.BridgeIP(), c.IsConfigured())
	}
}
