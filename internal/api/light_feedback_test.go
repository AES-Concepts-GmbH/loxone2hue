package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

// udpListener collects the datagrams a Miniserver would receive.
type udpListener struct {
	conn *net.UDPConn
	mu   sync.Mutex
	msgs []string
}

func listenUDP(t *testing.T) *udpListener {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	l := &udpListener{conn: conn}
	go func() {
		buf := make([]byte, 1024)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			l.mu.Lock()
			l.msgs = append(l.msgs, string(buf[:n]))
			l.mu.Unlock()
		}
	}()
	t.Cleanup(func() { conn.Close() })
	return l
}

func (l *udpListener) port() int { return l.conn.LocalAddr().(*net.UDPAddr).Port }

// wait returns the messages once n have arrived or the deadline passed.
func (l *udpListener) wait(n int) []string {
	deadline := time.Now().Add(2 * time.Second)
	for {
		l.mu.Lock()
		got := append([]string(nil), l.msgs...)
		l.mu.Unlock()
		if len(got) >= n || time.Now().After(deadline) {
			sort.Strings(got)
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func lightEvent(t *testing.T, raw string) hue.Event {
	t.Helper()
	var item hue.EventItem
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatalf("decode event item: %v", err)
	}
	return hue.Event{Type: item.Type, ID: item.ID, Data: item}
}

// TestLightAndGroupFeedback covers the actuator side of the Loxone feedback:
// a light event goes out per changed property in the UDP ("<id>/<prop>:<v>")
// and HTTP ("/dev/sps/io/<id>_<prop>/<v>") formats, a room's grouped_light
// event lands on the room mapping, mappings with a protocol switched off are
// skipped on that protocol, and scene/mood mappings never receive feedback.
func TestLightAndGroupFeedback(t *testing.T) {
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
	if _, err := hueClient.GetGroups(); err != nil {
		t.Fatalf("GetGroups: %v", err)
	}

	udp := listenUDP(t)

	var mu sync.Mutex
	var httpPaths []string
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		httpPaths = append(httpPaths, r.URL.Path)
		mu.Unlock()
	}))
	defer ms.Close()

	noUDP := false
	mappings := loxone.NewMappingManager()
	mappings.Load([]models.Mapping{
		{ID: "m1", LoxoneID: "wz_decke", HueID: "light-1", HueType: "light", Enabled: true, MiniserverID: "ms1"},
		// The room: HTTP only.
		{ID: "m2", LoxoneID: "wz", HueID: "room-1", HueType: "group", Enabled: true, MiniserverID: "ms1", FeedbackUDP: &noUDP},
		// Mood 0 of the room points at the same group but must stay silent.
		{ID: "m3", LoxoneID: "wz_mood_0", HueID: "room-1", HueType: "group", Enabled: true, MiniserverID: "ms1"},
		{ID: "m4", LoxoneID: "abend", HueID: "scene-1", HueType: "scene", Enabled: true, MiniserverID: "ms1"},
	})

	udpSender := loxone.NewUDPSender()
	defer udpSender.Close()
	udpSender.Configure([]config.MiniserverConfig{{ID: "ms1", Name: "Test", IP: "127.0.0.1", Port: udp.port(), UDPEnabled: true}})
	httpSender := loxone.NewHTTPSender()
	defer httpSender.Close()
	httpSender.Configure([]config.MiniserverConfig{{ID: "ms1", Name: "Test", HTTPEnabled: true, HTTPURL: ms.URL}})

	hub := NewWebSocketHub(hueClient, mappings, udpSender, httpSender)

	hub.sendFeedback(lightEvent(t, `{"id":"light-1","type":"light","on":{"on":true},"dimming":{"brightness":55.4},`+
		`"color_temperature":{"mirek":300},"color":{"xy":{"x":0.45,"y":0.4}}}`))
	hub.sendFeedback(lightEvent(t, `{"id":"gl-1","type":"grouped_light","on":{"on":false}}`))
	// Unknown light, no send_all target: nothing goes out.
	hub.sendFeedback(lightEvent(t, `{"id":"light-9","type":"light","on":{"on":true}}`))

	wantUDP := []string{
		"wz_decke/bri:55",
		"wz_decke/color_x:0.4500",
		"wz_decke/color_y:0.4000",
		"wz_decke/ct:300",
		"wz_decke/on:1",
	}
	if got := udp.wait(len(wantUDP)); !equalStrings(got, wantUDP) {
		t.Errorf("UDP messages = %v, want %v", got, wantUDP)
	}

	mu.Lock()
	gotHTTP := append([]string(nil), httpPaths...)
	mu.Unlock()
	sort.Strings(gotHTTP)
	wantHTTP := []string{
		"/dev/sps/io/wz_decke_bri/55",
		"/dev/sps/io/wz_decke_color_x/0.4500",
		"/dev/sps/io/wz_decke_color_y/0.4000",
		"/dev/sps/io/wz_decke_ct/300",
		"/dev/sps/io/wz_decke_on/1",
		"/dev/sps/io/wz_on/0",
	}
	if !equalStrings(gotHTTP, wantHTTP) {
		t.Errorf("HTTP requests = %v, want %v", gotHTTP, wantHTTP)
	}
}

// TestFeedbackSendAllUsesDeviceName: with send_all on a Miniserver, unmapped
// lights report under their sanitized HUE name, the same ID the XML export
// generates for them.
func TestFeedbackSendAllUsesDeviceName(t *testing.T) {
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

	udp := listenUDP(t)
	udpSender := loxone.NewUDPSender()
	defer udpSender.Close()
	udpSender.Configure([]config.MiniserverConfig{{ID: "ms1", Name: "Test", IP: "127.0.0.1", Port: udp.port(), UDPEnabled: true, SendAll: true}})

	hub := NewWebSocketHub(hueClient, loxone.NewMappingManager(), udpSender, loxone.NewHTTPSender())
	hub.sendFeedback(lightEvent(t, `{"id":"light-1","type":"light","on":{"on":true}}`))

	// "WZ Decke" -> wz_decke
	if got := udp.wait(1); !equalStrings(got, []string{"wz_decke/on:1"}) {
		t.Errorf("UDP messages = %v, want [wz_decke/on:1]", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
