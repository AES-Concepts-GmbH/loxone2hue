package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/sbeyeler/loxone2hue/internal/config"
	"github.com/sbeyeler/loxone2hue/internal/models"
)

// TestBackupListingHidesSecrets covers the backup tab of the Web UI: it only
// needs metadata, yet the listing used to ship every backup in full, HUE
// application key and Miniserver passwords included, on each visit. The
// download endpoint is the one place the full file is meant to leave the
// server.
func TestBackupListingHidesSecrets(t *testing.T) {
	const (
		hueKey     = "very-secret-hue-key"
		msPassword = "miniserver-secret"
	)

	h := newTestHandlers(t)
	config.UpdateHue("10.0.0.5", hueKey)
	config.UpdateLoxone(config.LoxoneConfig{Miniservers: []config.MiniserverConfig{
		{ID: "ms1", Name: "Haus", IP: "10.0.0.9", HTTPEnabled: true, HTTPPassword: msPassword},
	}})
	config.UpdateMappings([]models.Mapping{
		{ID: "a", LoxoneID: "wz", HueID: "1", HueType: "light", Enabled: true},
		{ID: "b", LoxoneID: "kueche", HueID: "2", HueType: "group", Enabled: true},
	})

	assertNoSecrets := func(where, body string) {
		t.Helper()
		if strings.Contains(body, hueKey) {
			t.Errorf("%s leaks the HUE application key: %s", where, body)
		}
		if strings.Contains(body, msPassword) {
			t.Errorf("%s leaks the Miniserver password: %s", where, body)
		}
	}

	rec := httptest.NewRecorder()
	h.CreateBackup(rec, httptest.NewRequest("POST", "/api/backups", strings.NewReader(`{"remark":"vor Umbau"}`)))
	if rec.Code != 201 {
		t.Fatalf("create: status = %d, body %s", rec.Code, rec.Body.String())
	}
	assertNoSecrets("create response", rec.Body.String())

	rec = httptest.NewRecorder()
	h.ListBackups(rec, httptest.NewRequest("GET", "/api/backups", nil))
	if rec.Code != 200 {
		t.Fatalf("list: status = %d, body %s", rec.Code, rec.Body.String())
	}
	list := rec.Body.String()
	assertNoSecrets("listing", list)
	for _, want := range []string{`"remark":"vor Umbau"`, `"mapping_count":2`, `"miniserver_count":1`} {
		if !strings.Contains(list, want) {
			t.Errorf("listing lacks %s: %s", want, list)
		}
	}

	// The download must still be the complete, restorable file.
	var created struct {
		ID string `json:"id"`
	}
	start := strings.Index(list, `"id":"`) + len(`"id":"`)
	created.ID = list[start : start+36]

	rec = httptest.NewRecorder()
	req := mux.SetURLVars(httptest.NewRequest("GET", "/api/backups/"+created.ID+"/download", nil),
		map[string]string{"id": created.ID})
	h.DownloadBackup(rec, req)
	if rec.Code != 200 {
		t.Fatalf("download: status = %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), hueKey) || !strings.Contains(rec.Body.String(), msPassword) {
		t.Error("download no longer contains the full configuration")
	}
}
