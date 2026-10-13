package api

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

// TestExportVirtualInputs covers the Virtual UDP Input template: one block of
// five command recognitions per mapped light or group, none for scenes, mood
// children or disabled mappings, the UDP port of the Miniserver, and the
// framing Loxone Config insists on (UTF-8 BOM, CRLF, escaped attributes).
func TestExportVirtualInputs(t *testing.T) {
	h := newTestHandlers(t)
	config.UpdateLoxone(config.LoxoneConfig{Miniservers: []config.MiniserverConfig{
		{ID: "ms1", Name: "Haus", IP: "10.0.0.9", Port: 7778, UDPEnabled: true},
	}})
	config.UpdateMappings([]models.Mapping{
		{ID: "a", Name: `K&L "Zimmer"`, LoxoneID: "kl_zimmer", HueID: "1", HueType: "light", Enabled: true},
		{ID: "b", Name: "Wohnzimmer", LoxoneID: "wz", HueID: "room-1", HueType: "group", Enabled: true},
		{ID: "c", Name: "Abend", LoxoneID: "abend", HueID: "scene-1", HueType: "scene", Enabled: true},
		{ID: "d", Name: "Wohnzimmer Mood 1", LoxoneID: "wz_mood_1", HueID: "scene-2", HueType: "scene", Enabled: true},
		{ID: "e", Name: "Alt", LoxoneID: "alt", HueID: "2", HueType: "light", Enabled: false},
	})

	rec := httptest.NewRecorder()
	h.ExportVirtualInputs(rec, httptest.NewRequest("GET", "/api/export/inputs", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.HasPrefix(body, "\xEF\xBB\xBF<?xml") {
		t.Error("export lacks the UTF-8 BOM Loxone Config requires")
	}
	if strings.Count(body, "\r\n") < 3 || strings.Contains(strings.ReplaceAll(body, "\r\n", ""), "\n") {
		t.Error("export must use CRLF line endings throughout")
	}
	if !strings.Contains(body, `Port="7778"`) {
		t.Error("export does not use the Miniserver's UDP port")
	}
	if !strings.Contains(body, `Title="K&amp;L &quot;Zimmer&quot; On"`) {
		t.Error("mapping name is not XML-escaped")
	}
	for _, id := range []string{"kl_zimmer", "wz"} {
		for _, prop := range []string{"on", "bri", "ct", "color_x", "color_y"} {
			if !strings.Contains(body, fmt.Sprintf(`Check="%s/%s:\v"`, id, prop)) {
				t.Errorf("missing command recognition %s/%s", id, prop)
			}
		}
	}
	for _, id := range []string{"abend", "wz_mood_1", "alt"} {
		if strings.Contains(body, id+"/on") {
			t.Errorf("export contains %s, which is a scene, mood child or disabled mapping", id)
		}
	}
	if got := strings.Count(body, "<VirtualInUdpCmd "); got != 10 {
		t.Errorf("command count = %d, want 10 (2 devices x 5 properties)", got)
	}
}

// TestExportVirtualOutputs covers the Virtual HTTP Output template: the
// address comes from the request host plus the configured server port, every
// light and group gets a dimmer (Br) and a switch (On/Off), and the
// "_mood_<n>" scene mappings of a group collapse into one MOOD command.
func TestExportVirtualOutputs(t *testing.T) {
	h := newTestHandlers(t)
	config.UpdateMappings([]models.Mapping{
		{ID: "a", Name: "Decke", LoxoneID: "wz_decke", HueID: "1", HueType: "light", Enabled: true, Description: "Hauptlicht"},
		{ID: "b", Name: "Wohnzimmer", LoxoneID: "wz", HueID: "room-1", HueType: "group", Enabled: true},
		{ID: "c", Name: "Wohnzimmer - Aus", LoxoneID: "wz_mood_0", HueID: "room-1", HueType: "group", Enabled: true},
		{ID: "d", Name: "Wohnzimmer - Lesen", LoxoneID: "wz_mood_1", HueID: "scene-1", HueType: "scene", Enabled: true},
		{ID: "e", Name: "Wohnzimmer - Kino", LoxoneID: "wz_mood_2", HueID: "scene-2", HueType: "scene", Enabled: true},
		// Mood children without a base mapping still get a MOOD command.
		{ID: "f", Name: "Küche - Kochen", LoxoneID: "kueche_mood_1", HueID: "scene-3", HueType: "scene", Enabled: true},
		{ID: "g", Name: "Abend", LoxoneID: "abend", HueID: "scene-4", HueType: "scene", Enabled: true},
		{ID: "h", Name: "Alt", LoxoneID: "alt", HueID: "2", HueType: "light", Enabled: false},
	})

	req := httptest.NewRequest("GET", "/api/export/outputs", nil)
	req.Host = "gateway.local:3000" // the UI may be proxied; the gateway port is what counts
	rec := httptest.NewRecorder()
	h.ExportVirtualOutputs(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	wantAddr := fmt.Sprintf(`Address="http://gateway.local:%d"`, config.GetServer().Port)
	if !strings.Contains(body, wantAddr) {
		t.Errorf("export lacks %s", wantAddr)
	}

	for _, want := range []string{
		`Title="Decke (Br)" Comment="Hauptlicht" CmdOnMethod="GET" CmdOn="/ws?cmd=SET wz_decke BRI &lt;v&gt;"`,
		`Title="Decke (On/Off)" Comment="Hauptlicht" CmdOnMethod="GET" CmdOn="/ws?cmd=SET wz_decke ON"`,
		`CmdOff="/ws?cmd=SET wz_decke OFF"`,
		`Title="Wohnzimmer (Br)"`,
		`Title="Wohnzimmer (On/Off)"`,
		`Title="Wohnzimmer (Mood)" Comment="Mood: 0=Aus, 1-2=Szene" CmdOnMethod="GET" CmdOn="/ws?cmd=MOOD wz &lt;v&gt;"`,
		`Title="Kueche (Mood)" Comment="Mood: 0=Aus, 1-1=Szene"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("export lacks %s\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"wz_mood_", "kueche_mood_", "abend", `Title="Alt`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("export contains %q: scenes and disabled mappings have no output command", unwanted)
		}
	}
	if got := strings.Count(body, "<VirtualOutCmd "); got != 6 {
		t.Errorf("command count = %d, want 6 (2 moods + 2 x (Br, On/Off))", got)
	}
}

// TestSanitizeName pins the Loxone ID derived from a HUE name. The Web UI
// mirrors this function, so the two must not drift apart.
func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"WZ Decke":          "wz_decke",
		"Küche Süd-Ost":     "kueche_sued_ost",
		"Büro (Stefan) 2.0": "buero_stefan_20",
		"Straße":            "strasse",
		"ÄÖÜ":               "aeoeue",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
