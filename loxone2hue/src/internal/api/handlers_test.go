package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sbeyeler/loxone2hue/internal/config"
	"github.com/sbeyeler/loxone2hue/internal/hue"
	"github.com/sbeyeler/loxone2hue/internal/loxone"
)

func newTestHandlers(t *testing.T) *Handlers {
	t.Helper()

	if _, err := config.Load(filepath.Join(t.TempDir(), "config.yaml")); err != nil {
		t.Fatalf("load config: %v", err)
	}
	config.UpdateMappings(nil)

	return NewHandlers(hue.NewClient("", ""), loxone.NewMappingManager(),
		loxone.NewUDPSender(), loxone.NewHTTPSender())
}

// TestCreateMappingRespectsExplicitEnabled covers the flag that used to be
// overwritten unconditionally: a client could not create a disabled mapping.
func TestCreateMappingRespectsExplicitEnabled(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"ohne Feld", `{"loxone_id":"a","hue_id":"1","hue_type":"light"}`, true},
		{"explizit true", `{"loxone_id":"b","hue_id":"2","hue_type":"light","enabled":true}`, true},
		{"explizit false", `{"loxone_id":"c","hue_id":"3","hue_type":"light","enabled":false}`, false},
	}

	h := newTestHandlers(t)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.CreateMapping(rec, httptest.NewRequest("POST", "/api/mappings", strings.NewReader(tc.body)))

			if rec.Code != 201 {
				t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
			}

			var got struct {
				Enabled bool `json:"enabled"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Enabled != tc.want {
				t.Errorf("enabled = %v, want %v", got.Enabled, tc.want)
			}
		})
	}
}

// TestImportMappingsCountsSkipped pins the previously always-zero counter.
func TestImportMappingsCountsSkipped(t *testing.T) {
	h := newTestHandlers(t)

	body := `{"mode":"replace","backup":{"version":"1.0","mappings":[
		{"loxone_id":"gut","hue_id":"1","hue_type":"light","enabled":true},
		{"loxone_id":"","hue_id":"2","hue_type":"light"},
		{"loxone_id":"ohne_hue","hue_id":"","hue_type":"light"}
	]}}`

	rec := httptest.NewRecorder()
	h.ImportMappings(rec, httptest.NewRequest("POST", "/api/mappings/import", strings.NewReader(body)))

	if rec.Code != 200 {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var got struct {
		Imported int `json:"imported"`
		Skipped  int `json:"skipped"`
		Total    int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Imported != 1 || got.Skipped != 2 || got.Total != 1 {
		t.Errorf("imported=%d skipped=%d total=%d, want 1/2/1", got.Imported, got.Skipped, got.Total)
	}
}

// TestPairBridgeFailureKeepsCurrentBridge covers a typo in the pairing dialog:
// the shared client used to be repointed before the attempt, so a failed
// pairing broke every REST call against the bridge that was actually working.
func TestPairBridgeFailureKeepsCurrentBridge(t *testing.T) {
	if _, err := config.Load(filepath.Join(t.TempDir(), "config.yaml")); err != nil {
		t.Fatalf("load config: %v", err)
	}

	client := hue.NewClient("192.0.2.1", "working-key")
	h := NewHandlers(client, loxone.NewMappingManager(), loxone.NewUDPSender(), loxone.NewHTTPSender())

	// Nothing listens on port 1, so the connection is refused immediately.
	rec := httptest.NewRecorder()
	h.PairBridge(rec, httptest.NewRequest("POST", "/api/bridge/pair",
		strings.NewReader(`{"bridge_ip":"127.0.0.1:1"}`)))

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if got := client.BridgeIP(); got != "192.0.2.1" {
		t.Errorf("bridge IP after failed pairing = %q, want unchanged 192.0.2.1", got)
	}
	if !client.IsConfigured() {
		t.Error("client lost its configuration after a failed pairing")
	}
}
