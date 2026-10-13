package config

import (
	"path/filepath"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

// TestSnapshotAndGettersReturnCopies: the getters hand out copies, so a
// caller cannot change the live configuration by accident, and
// RestoreFromBackup replaces everything the backup carries.
func TestSnapshotAndGettersReturnCopies(t *testing.T) {
	path := loadInto(t)
	if got := GetPath(); got != path {
		t.Errorf("GetPath = %q, want %q", got, path)
	}

	SetVersion("9.9.9")
	if GetVersion() != "9.9.9" {
		t.Errorf("GetVersion = %q", GetVersion())
	}

	RestoreFromBackup(
		ServerConfig{Port: 9090, Host: "127.0.0.1"},
		"10.0.0.5", "key",
		LoxoneConfig{Miniservers: []MiniserverConfig{{ID: "ms1", Name: "Haus", IP: "10.0.0.9", Port: 7777, UDPEnabled: true}}},
		LoggingConfig{Level: "debug", Format: "console"},
		[]models.Mapping{{ID: "a", LoxoneID: "wz", HueID: "1", HueType: "light", Enabled: true}},
	)

	if s := GetServer(); s.Port != 9090 || s.Host != "127.0.0.1" {
		t.Errorf("GetServer = %+v", s)
	}
	if h := GetHue(); h.BridgeIP != "10.0.0.5" || h.ApplicationKey != "key" {
		t.Errorf("GetHue = %+v", h)
	}

	snap := Snapshot()
	if snap.Logging.Level != "debug" || len(snap.Mappings) != 1 || len(snap.Loxone.Miniservers) != 1 {
		t.Errorf("Snapshot = %+v", snap)
	}

	// Mutating the copies must not reach the live config.
	snap.Mappings[0].LoxoneID = "changed"
	snap.Loxone.Miniservers[0].Name = "changed"
	lox := GetLoxone()
	lox.Miniservers[0].IP = "changed"
	maps := GetMappings()
	maps[0].HueID = "changed"

	live := Snapshot()
	if live.Mappings[0].LoxoneID != "wz" || live.Mappings[0].HueID != "1" {
		t.Errorf("mapping changed through a copy: %+v", live.Mappings[0])
	}
	if live.Loxone.Miniservers[0].Name != "Haus" || live.Loxone.Miniservers[0].IP != "10.0.0.9" {
		t.Errorf("miniserver changed through a copy: %+v", live.Loxone.Miniservers[0])
	}

	// A fresh load of a non-existent file keeps defaults for the server.
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err != nil {
		t.Fatalf("load missing: %v", err)
	}
}
