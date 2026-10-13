package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadMigratesLegacyConfig covers the two format changes a config file
// from an early release carries: the single "miniserver_ip"/"udp_feedback"
// block became the miniservers list, and mappings used to be written without
// yaml tags (loxoneid, hueid, huetype). Both must load into the current
// structures, and every mapping gets the migrated Miniserver assigned.
func TestLoadMigratesLegacyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `server:
  port: 8080
  host: 0.0.0.0
hue:
  bridge_ip: 192.168.1.20
  application_key: alt
loxone:
  enabled: true
  miniserver_ip: 192.168.1.10
  udp_feedback:
    enabled: true
    port: 7778
    send_all: true
mappings:
  - id: m1
    name: Decke
    loxoneid: wz_decke
    hueid: light-1
    huetype: light
    enabled: true
  - id: m2
    name: Abend
    loxoneid: abend
    hueid: scene-1
    huetype: scene
    enabled: true
`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}

	ms := GetLoxone().Miniservers
	if len(ms) != 1 {
		t.Fatalf("miniservers = %+v, want exactly one migrated entry", ms)
	}
	if ms[0].IP != "192.168.1.10" || ms[0].Port != 7778 || !ms[0].UDPEnabled || !ms[0].SendAll || ms[0].ID == "" {
		t.Errorf("migrated miniserver = %+v", ms[0])
	}

	mappings := GetMappings()
	if len(mappings) != 2 {
		t.Fatalf("mappings = %+v, want 2", mappings)
	}
	want := []struct{ loxoneID, hueID, hueType string }{
		{"wz_decke", "light-1", "light"},
		{"abend", "scene-1", "scene"},
	}
	for i, w := range want {
		m := mappings[i]
		if m.LoxoneID != w.loxoneID || m.HueID != w.hueID || m.HueType != w.hueType {
			t.Errorf("mapping %d = %+v, want %+v", i, m, w)
		}
		if m.MiniserverID != ms[0].ID {
			t.Errorf("mapping %d miniserver = %q, want the migrated %q", i, m.MiniserverID, ms[0].ID)
		}
	}

	// Saving writes the current format; loading that again must not migrate
	// a second time or lose anything.
	if err := Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, old := range []string{"miniserver_ip", "udp_feedback", "loxoneid:", "hueid:", "huetype:"} {
		if contains(string(data), old) {
			t.Errorf("saved config still contains legacy key %q:\n%s", old, data)
		}
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if again := GetLoxone().Miniservers; len(again) != 1 || again[0].ID != ms[0].ID {
		t.Errorf("miniservers after reload = %+v, want the same single entry", again)
	}
	if again := GetMappings(); len(again) != 2 || again[0].LoxoneID != "wz_decke" {
		t.Errorf("mappings after reload = %+v", again)
	}
}

// TestLoadKeepsCurrentFormat: a current config with a miniservers list is
// taken as is, even when a stale legacy key is still around.
func TestLoadKeepsCurrentFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	current := `loxone:
  miniserver_ip: 10.0.0.1
  miniservers:
    - id: ms-a
      name: Haus
      ip: 10.0.0.2
      port: 7777
      udp_enabled: true
mappings:
  - id: m1
    loxone_id: wz
    hue_id: room-1
    hue_type: group
    enabled: true
`
	if err := os.WriteFile(path, []byte(current), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	ms := GetLoxone().Miniservers
	if len(ms) != 1 || ms[0].ID != "ms-a" || ms[0].IP != "10.0.0.2" {
		t.Errorf("miniservers = %+v, want the configured ms-a untouched", ms)
	}
	if m := GetMappings(); len(m) != 1 || m[0].LoxoneID != "wz" || m[0].MiniserverID != "ms-a" {
		t.Errorf("mappings = %+v", m)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
