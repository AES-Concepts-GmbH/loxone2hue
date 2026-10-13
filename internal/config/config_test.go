package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"gopkg.in/yaml.v3"
)

func loadInto(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	return path
}

// TestSaveLeavesNoTempFileAndKeepsSecretsPrivate covers the rewrite of Save to
// a temp file plus rename. The config carries the HUE application key and the
// Miniserver passwords, so it must not be world readable.
func TestSaveLeavesNoTempFileAndKeepsSecretsPrivate(t *testing.T) {
	path := loadInto(t)
	dir := filepath.Dir(path)

	UpdateHue("192.168.1.5", "secret-key")
	if err := Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config mode = %04o, want 0600", perm)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".config-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var round Config
	if err := yaml.Unmarshal(data, &round); err != nil {
		t.Fatalf("saved config is not valid YAML: %v", err)
	}
	if round.Hue.ApplicationKey != "secret-key" {
		t.Errorf("application_key = %q, want secret-key", round.Hue.ApplicationKey)
	}
}

// TestConcurrentSavesProduceValidConfig hammers Save from several goroutines.
// With the previous read lock plus a direct WriteFile, two savers could write
// over each other and leave a truncated file behind.
func TestConcurrentSavesProduceValidConfig(t *testing.T) {
	path := loadInto(t)

	UpdateHue("10.0.0.1", "key")
	UpdateMappings([]models.Mapping{
		{ID: "a", Name: "Küche", LoxoneID: "kueche", HueID: "1", HueType: "light", Enabled: true},
		{ID: "b", Name: "Bad", LoxoneID: "bad", HueID: "2", HueType: "group", Enabled: true},
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Save(); err != nil {
				t.Errorf("save: %v", err)
			}
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var round Config
	if err := yaml.Unmarshal(data, &round); err != nil {
		t.Fatalf("concurrent saves produced invalid YAML: %v", err)
	}
	if len(round.Mappings) != 2 {
		t.Errorf("mappings = %d, want 2", len(round.Mappings))
	}
}

// TestUpdateLoxonePreservesRedactedPassword covers the round trip the Web UI
// performs: it reads the config with the password redacted and sends the whole
// thing back. An empty password must not wipe the stored one.
func TestUpdateLoxonePreservesRedactedPassword(t *testing.T) {
	loadInto(t)

	UpdateLoxone(LoxoneConfig{Miniservers: []MiniserverConfig{
		{ID: "m1", Name: "MS", IP: "192.168.1.10", HTTPUser: "admin", HTTPPassword: "secret"},
	}})

	// Coming back from the UI: same miniserver, password still the sentinel.
	UpdateLoxone(LoxoneConfig{Miniservers: []MiniserverConfig{
		{ID: "m1", Name: "MS umbenannt", IP: "192.168.1.10", HTTPUser: "admin", HTTPPassword: MaskedPassword},
	}})

	got := GetLoxone().Miniservers
	if len(got) != 1 {
		t.Fatalf("miniservers = %d, want 1", len(got))
	}
	if got[0].HTTPPassword != "secret" {
		t.Errorf("password = %q, want it preserved", got[0].HTTPPassword)
	}
	if got[0].Name != "MS umbenannt" {
		t.Errorf("name = %q, other fields must still update", got[0].Name)
	}

	// An explicitly supplied password wins.
	UpdateLoxone(LoxoneConfig{Miniservers: []MiniserverConfig{
		{ID: "m1", Name: "MS", HTTPPassword: "neu"},
	}})
	if got := GetLoxone().Miniservers[0].HTTPPassword; got != "neu" {
		t.Errorf("password = %q, want neu", got)
	}

	// A removed miniserver must not resurrect its password.
	UpdateLoxone(LoxoneConfig{Miniservers: []MiniserverConfig{}})
	if n := len(GetLoxone().Miniservers); n != 0 {
		t.Errorf("miniservers = %d, want 0", n)
	}
}
