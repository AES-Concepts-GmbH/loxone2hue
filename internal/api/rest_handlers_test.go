package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/logging"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

// bridgeRecorder is a fuller fake bridge than fakeBridge: it also answers the
// bridge info, scenes, every sensor endpoint, the pairing call and records
// every PUT so tests can check what the gateway sent.
type bridgeRecorder struct {
	srv  *httptest.Server
	mu   sync.Mutex
	puts map[string]string // path -> last body
	// pairResponse is what POST /api answers.
	pairResponse string
}

func newBridgeRecorder(t *testing.T) *bridgeRecorder {
	t.Helper()
	rec := &bridgeRecorder{puts: make(map[string]string), pairResponse: `[{"success":{"username":"new-app-key"}}]`}

	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		fmt.Fprint(w, rec.pairResponse)
	})
	mux.HandleFunc("/clip/v2/resource/bridge", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"bridge-1","bridge_id":"001788FFFE000001"}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/light", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"light-1","type":"light","owner":{"rid":"dev-1","rtype":"device"},`+
			`"metadata":{"name":"WZ Decke","archetype":"ceiling_round"},"product_data":{"model_id":"LCA001","product_name":"Hue color lamp"},`+
			`"on":{"on":true},"dimming":{"brightness":80},"color_temperature":{"mirek":300,"mirek_valid":true},`+
			`"color":{"xy":{"x":0.4,"y":0.4},"gamut_type":"C"}}]}`)
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
	mux.HandleFunc("/clip/v2/resource/scene", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"scene-1","type":"scene","metadata":{"name":"Abend"},"group":{"rid":"room-1","rtype":"room"}}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/device", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"dev-1","metadata":{"name":"Flur Sensor"}}]}`)
	})
	mux.HandleFunc("/clip/v2/resource/motion", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"sensor-1","owner":{"rid":"dev-1","rtype":"device"},"enabled":true,`+
			`"motion":{"motion_report":{"changed":"t","motion":true}}}]}`)
	})
	// Everything else: sensor types this bridge does not have (empty lists)
	// and the PUTs for lights, grouped lights and scenes.
	mux.HandleFunc("/clip/v2/resource/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			rec.mu.Lock()
			rec.puts[r.URL.Path] = string(body)
			rec.mu.Unlock()
		}
		fmt.Fprint(w, `{"data":[]}`)
	})
	mux.HandleFunc("/eventstream/clip/v2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		<-r.Context().Done()
	})

	rec.srv = httptest.NewTLSServer(mux)
	t.Cleanup(rec.srv.Close)
	return rec
}

func (b *bridgeRecorder) addr() string { return strings.TrimPrefix(b.srv.URL, "https://") }

func (b *bridgeRecorder) put(path string) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	body, ok := b.puts[path]
	return body, ok
}

// newRESTServer wires a Server to the fake bridge with the login disabled.
func newRESTServer(t *testing.T, bridge *bridgeRecorder, mappings []models.Mapping) *Server {
	t.Helper()
	if _, err := config.Load(t.TempDir() + "/config.yaml"); err != nil {
		t.Fatalf("load config: %v", err)
	}
	_ = config.UpdateAuth(config.AuthConfig{})
	config.UpdateMappings(mappings)
	config.UpdateLoxone(config.LoxoneConfig{Miniservers: []config.MiniserverConfig{}})

	hueClient := hue.NewClient("", "")
	if bridge != nil {
		hueClient.Configure(bridge.addr(), "test-key")
		config.UpdateHue(bridge.addr(), "test-key")
	} else {
		config.UpdateHue("", "") // the config singleton outlives earlier tests
	}
	mm := loxone.NewMappingManager()
	mm.Load(mappings)
	return NewServer(hueClient, mm, loxone.NewUDPSender(), loxone.NewHTTPSender())
}

func do(t *testing.T, srv *Server, method, path, body string) (int, map[string]interface{}, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest(method, path, r))
	var parsed map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	return rec.Code, parsed, rec.Body.String()
}

// TestRESTReadsAndWritesAgainstBridge covers the thin handlers that pass
// through to the Hue client: lights, groups, scenes and sensors are read from
// the bridge and cached, a 404 for unknown IDs, and the PUTs reach the bridge
// in CLIP v2 form.
func TestRESTReadsAndWritesAgainstBridge(t *testing.T) {
	bridge := newBridgeRecorder(t)
	srv := newRESTServer(t, bridge, nil)

	code, body, raw := do(t, srv, "GET", "/api/bridge", "")
	if code != 200 || body["configured"] != true || body["info"] == nil {
		t.Errorf("GET /api/bridge: %d %s", code, raw)
	}

	code, body, raw = do(t, srv, "GET", "/api/devices", "")
	if code != 200 || len(body["devices"].([]interface{})) != 1 {
		t.Fatalf("GET /api/devices: %d %s", code, raw)
	}
	light := body["devices"].([]interface{})[0].(map[string]interface{})
	caps := light["capabilities"].(map[string]interface{})
	if light["name"] != "WZ Decke" || caps["supports_color"] != true || caps["supports_color_temp"] != true || caps["supports_dimming"] != true {
		t.Errorf("light conversion = %v", light)
	}

	code, body, raw = do(t, srv, "GET", "/api/devices/light-1", "")
	if code != 200 || body["id"] != "light-1" {
		t.Errorf("GET /api/devices/light-1: %d %s", code, raw)
	}
	// Unknown light: the bridge answers with an empty list, the API with 404.
	if code, _, raw = do(t, srv, "GET", "/api/devices/light-9", ""); code != 404 {
		t.Errorf("GET unknown light: %d %s", code, raw)
	}

	if code, _, raw = do(t, srv, "PUT", "/api/devices/light-1", `{"on":true,"brightness":50,"color_temp":300}`); code != 200 {
		t.Errorf("PUT /api/devices/light-1: %d %s", code, raw)
	}
	if sent, ok := bridge.put("/clip/v2/resource/light/light-1"); !ok || !strings.Contains(sent, `"brightness":50`) || !strings.Contains(sent, `"mirek":300`) {
		t.Errorf("bridge received %q for the light PUT", sent)
	}
	if code, _, _ = do(t, srv, "PUT", "/api/devices/light-1", `{nope`); code != 400 {
		t.Errorf("invalid JSON: %d, want 400", code)
	}

	code, body, raw = do(t, srv, "GET", "/api/groups", "")
	if code != 200 || len(body["groups"].([]interface{})) != 1 {
		t.Fatalf("GET /api/groups: %d %s", code, raw)
	}
	code, body, raw = do(t, srv, "GET", "/api/groups/room-1", "")
	if code != 200 || body["name"] != "Wohnzimmer" || body["grouped_light_id"] != "gl-1" {
		t.Errorf("GET /api/groups/room-1: %d %s", code, raw)
	}
	if code, _, _ = do(t, srv, "GET", "/api/groups/room-9", ""); code != 404 {
		t.Errorf("GET unknown group: %d, want 404", code)
	}
	if code, _, raw = do(t, srv, "PUT", "/api/groups/room-1", `{"on":false}`); code != 200 {
		t.Errorf("PUT /api/groups/room-1: %d %s", code, raw)
	}
	if sent, ok := bridge.put("/clip/v2/resource/grouped_light/gl-1"); !ok || !strings.Contains(sent, `"on":false`) {
		t.Errorf("bridge received %q for the group PUT (room resolved to its grouped_light)", sent)
	}

	code, body, raw = do(t, srv, "GET", "/api/scenes", "")
	if code != 200 || len(body["scenes"].([]interface{})) != 1 {
		t.Fatalf("GET /api/scenes: %d %s", code, raw)
	}
	scene := body["scenes"].([]interface{})[0].(map[string]interface{})
	if scene["name"] != "Abend" || scene["group_id"] != "room-1" || scene["type"] != "room" {
		t.Errorf("scene conversion = %v", scene)
	}
	if code, _, raw = do(t, srv, "POST", "/api/scenes/scene-1/activate", ""); code != 200 {
		t.Errorf("activate scene: %d %s", code, raw)
	}
	if sent, ok := bridge.put("/clip/v2/resource/scene/scene-1"); !ok || !strings.Contains(sent, `"action":"active"`) {
		t.Errorf("bridge received %q for the scene activation", sent)
	}

	code, body, raw = do(t, srv, "GET", "/api/sensors", "")
	if code != 200 || len(body["sensors"].([]interface{})) != 1 {
		t.Fatalf("GET /api/sensors: %d %s", code, raw)
	}
	sensor := body["sensors"].([]interface{})[0].(map[string]interface{})
	if sensor["name"] != "Flur Sensor Motion" || sensor["scope"] != "device" {
		t.Errorf("sensor = %v", sensor)
	}

	code, body, raw = do(t, srv, "GET", "/api/health", "")
	if code != 200 || body["status"] != "healthy" || body["hue_configured"] != true {
		t.Errorf("GET /api/health: %d %s", code, raw)
	}
}

// TestRESTWithoutBridge: with no bridge configured the bridge endpoint says
// so instead of failing, and the device endpoints report the error.
func TestRESTWithoutBridge(t *testing.T) {
	srv := newRESTServer(t, nil, nil)

	code, body, raw := do(t, srv, "GET", "/api/bridge", "")
	if code != 200 || body["configured"] != false {
		t.Errorf("GET /api/bridge: %d %s", code, raw)
	}
	if code, _, _ := do(t, srv, "GET", "/api/devices", ""); code != 500 {
		t.Errorf("GET /api/devices without bridge: %d, want 500", code)
	}
	code, body, raw = do(t, srv, "GET", "/api/health", "")
	if code != 200 || body["hue_configured"] != false {
		t.Errorf("GET /api/health: %d %s", code, raw)
	}
}

// TestPairBridgeStoresKeyAndReportsLinkButton covers the pairing dialog.
func TestPairBridgeStoresKeyAndReportsLinkButton(t *testing.T) {
	bridge := newBridgeRecorder(t)
	srv := newRESTServer(t, nil, nil)

	bridge.mu.Lock()
	bridge.pairResponse = `[{"error":{"type":101,"description":"link button not pressed"}}]`
	bridge.mu.Unlock()
	code, _, raw := do(t, srv, "POST", "/api/bridge/pair", `{"bridge_ip":"`+bridge.addr()+`"}`)
	if code != 400 || !strings.Contains(raw, "link button") {
		t.Errorf("pair without link button: %d %s", code, raw)
	}
	if config.GetHue().ApplicationKey != "" {
		t.Error("failed pairing stored a key")
	}

	bridge.mu.Lock()
	bridge.pairResponse = `[{"success":{"username":"new-app-key"}}]`
	bridge.mu.Unlock()
	code, body, raw := do(t, srv, "POST", "/api/bridge/pair", `{"bridge_ip":"`+bridge.addr()+`"}`)
	if code != 200 || body["application_key"] != "new-app-key" {
		t.Fatalf("pair: %d %s", code, raw)
	}
	if h := config.GetHue(); h.BridgeIP != bridge.addr() || h.ApplicationKey != "new-app-key" {
		t.Errorf("config after pairing = %+v", h)
	}
	if !srv.hueClient.IsConfigured() {
		t.Error("shared client not pointed at the paired bridge")
	}
	if code, _, _ := do(t, srv, "POST", "/api/bridge/pair", `{}`); code != 400 {
		t.Errorf("pair without bridge_ip: %d, want 400", code)
	}
}

// TestBridgeConnectionTestReportsEachStep: the diagnostic runs all four
// probes against a LAN address and reports each outcome instead of failing
// the request when one of them is refused.
func TestBridgeConnectionTestReportsEachStep(t *testing.T) {
	srv := newRESTServer(t, nil, nil)
	code, body, raw := do(t, srv, "POST", "/api/bridge/test", `{"bridge_ip":"127.0.0.1"}`)
	if code != 200 {
		t.Fatalf("bridge test: %d %s", code, raw)
	}
	for _, key := range []string{"dns_lookup", "tcp_443", "tcp_80", "https_request"} {
		step, ok := body[key].(map[string]interface{})
		if !ok {
			t.Errorf("result lacks %s: %s", key, raw)
			continue
		}
		if _, ok := step["success"]; !ok {
			t.Errorf("%s has no success flag: %v", key, step)
		}
	}
	if dns := body["dns_lookup"].(map[string]interface{}); dns["success"] != true {
		t.Errorf("dns_lookup for 127.0.0.1 = %v", dns)
	}
	if code, _, _ := do(t, srv, "POST", "/api/bridge/test", `{}`); code != 400 {
		t.Errorf("bridge test without bridge_ip: %d, want 400", code)
	}
}

// TestMappingsListExportAndLogs covers the remaining read-only endpoints.
func TestMappingsListExportAndLogs(t *testing.T) {
	srv := newRESTServer(t, nil, []models.Mapping{
		{ID: "a", LoxoneID: "wz", HueID: "1", HueType: "light", Enabled: true},
	})

	code, body, raw := do(t, srv, "GET", "/api/mappings", "")
	if code != 200 || len(body["mappings"].([]interface{})) != 1 {
		t.Errorf("GET /api/mappings: %d %s", code, raw)
	}

	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest("GET", "/api/mappings/export", nil))
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("export: %d %v", rec.Code, rec.Header())
	}
	var exported MappingsBackup
	if err := json.Unmarshal(rec.Body.Bytes(), &exported); err != nil || exported.Version != "1.0" || len(exported.Mappings) != 1 {
		t.Errorf("export body = %s (%v)", rec.Body.String(), err)
	}

	logging.Buffer.Write([]byte(`{"level":"warn","message":"rest-handlers-test marker","time":"2026-10-13T22:00:00Z"}`))
	code, body, raw = do(t, srv, "GET", "/api/logs?level=warn&search=rest-handlers-test&limit=5", "")
	if code != 200 {
		t.Fatalf("GET /api/logs: %d %s", code, raw)
	}
	entries := body["entries"].([]interface{})
	if len(entries) != 1 || entries[0].(map[string]interface{})["message"] != "rest-handlers-test marker" {
		t.Errorf("log entries = %v", entries)
	}
}

// TestUpdateConfigReconfiguresSendersAndKeepsPassword covers PUT /api/config:
// the Miniserver list is replaced, a masked password keeps the stored one,
// and the UDP sender is live for the new Miniserver.
func TestUpdateConfigReconfiguresSendersAndKeepsPassword(t *testing.T) {
	srv := newRESTServer(t, nil, nil)
	config.UpdateLoxone(config.LoxoneConfig{Miniservers: []config.MiniserverConfig{
		{ID: "ms1", Name: "Haus", IP: "127.0.0.1", Port: 7777, HTTPEnabled: true, HTTPURL: "http://127.0.0.1:1", HTTPPassword: "geheim"},
	}})

	body := `{"loxone":{"miniservers":[{"id":"ms1","name":"Haus neu","ip":"127.0.0.1","port":7778,"udp_enabled":true,` +
		`"http_enabled":true,"http_url":"http://127.0.0.1:1","http_user":"admin","http_password":"` + config.MaskedPassword + `"}]}}`
	code, _, raw := do(t, srv, "PUT", "/api/config", body)
	if code != 200 {
		t.Fatalf("PUT /api/config: %d %s", code, raw)
	}
	ms := config.GetLoxone().Miniservers
	if len(ms) != 1 || ms[0].Name != "Haus neu" || ms[0].Port != 7778 || ms[0].HTTPPassword != "geheim" {
		t.Errorf("miniservers after update = %+v (password must be the stored one)", ms)
	}
	if !srv.udpSender.IsEnabledFor("ms1") || !srv.httpSender.IsEnabled() {
		t.Error("senders were not reconfigured")
	}

	code, body2, raw := do(t, srv, "GET", "/api/config", "")
	if code != 200 {
		t.Fatalf("GET /api/config: %d %s", code, raw)
	}
	got := body2["loxone"].(map[string]interface{})["miniservers"].([]interface{})[0].(map[string]interface{})
	if got["http_password"] != config.MaskedPassword {
		t.Errorf("GET /api/config exposes the password: %v", got["http_password"])
	}
	if code, _, _ := do(t, srv, "PUT", "/api/config", `{bad`); code != 400 {
		t.Errorf("invalid body: %d, want 400", code)
	}
}

// TestUDPTestEndpoint covers the "Testen" button of the UDP panel: input
// validation per property, the check that UDP is enabled for the Miniserver,
// and the datagram that goes out.
func TestUDPTestEndpoint(t *testing.T) {
	srv := newRESTServer(t, nil, nil)

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer conn.Close()
	port := conn.LocalAddr().(*net.UDPAddr).Port

	for name, body := range map[string]string{
		"ohne loxone_id":    `{"property":"on","value":"1","miniserver_id":"ms1"}`,
		"ohne miniserver":   `{"loxone_id":"wz","property":"on","value":"1"}`,
		"on ungueltig":      `{"loxone_id":"wz","property":"on","value":"2","miniserver_id":"ms1"}`,
		"bri ungueltig":     `{"loxone_id":"wz","property":"bri","value":"101","miniserver_id":"ms1"}`,
		"ct ungueltig":      `{"loxone_id":"wz","property":"ct","value":"100","miniserver_id":"ms1"}`,
		"color_x ungueltig": `{"loxone_id":"wz","property":"color_x","value":"1.5","miniserver_id":"ms1"}`,
		"unbekannt":         `{"loxone_id":"wz","property":"hue","value":"1","miniserver_id":"ms1"}`,
		"udp nicht aktiv":   `{"loxone_id":"wz","property":"on","value":"1","miniserver_id":"ms1"}`,
	} {
		if code, _, raw := do(t, srv, "POST", "/api/udp/test", body); code != 400 {
			t.Errorf("%s: %d %s, want 400", name, code, raw)
		}
	}

	srv.udpSender.Configure([]config.MiniserverConfig{{ID: "ms1", Name: "Haus", IP: "127.0.0.1", Port: port, UDPEnabled: true}})
	code, body, raw := do(t, srv, "POST", "/api/udp/test", `{"loxone_id":"wz","property":"bri","value":"42","miniserver_id":"ms1"}`)
	if code != 200 || body["message"] != "wz/bri:42" {
		t.Fatalf("udp test: %d %s", code, raw)
	}
	buf := make([]byte, 64)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil || string(buf[:n]) != "wz/bri:42" {
		t.Errorf("datagram = %q, %v; want wz/bri:42", buf[:n], err)
	}
}
