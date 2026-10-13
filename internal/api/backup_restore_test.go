package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"github.com/gorilla/mux"
)

// TestBackupRestoreRoundTrip: a backup taken before a change brings bridge,
// Miniservers and mappings back, both in the config and in the running
// mapping manager, and the restored config is written to disk.
func TestBackupRestoreRoundTrip(t *testing.T) {
	h := newTestHandlers(t)
	server := config.GetServer()
	defer config.RestoreFromBackup(server, "", "", config.LoxoneConfig{Miniservers: []config.MiniserverConfig{}},
		config.Snapshot().Logging, nil)

	config.UpdateHue("10.0.0.5", "key-before")
	config.UpdateLoxone(config.LoxoneConfig{Miniservers: []config.MiniserverConfig{
		{ID: "ms1", Name: "Haus", IP: "10.0.0.9", Port: 7777, UDPEnabled: true},
	}})
	before := []models.Mapping{
		{ID: "a", LoxoneID: "wz", HueID: "1", HueType: "light", Enabled: true, MiniserverID: "ms1"},
		{ID: "b", LoxoneID: "kueche", HueID: "2", HueType: "group", Enabled: true, MiniserverID: "ms1"},
	}
	config.UpdateMappings(before)
	h.mappingManager.Load(before)

	rec := httptest.NewRecorder()
	h.CreateBackup(rec, httptest.NewRequest("POST", "/api/backups", strings.NewReader(`{"remark":"vorher"}`)))
	if rec.Code != 201 {
		t.Fatalf("create: status = %d, body %s", rec.Code, rec.Body.String())
	}
	var created BackupSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.MappingCount != 2 || created.MiniserverCount != 1 || created.Remark != "vorher" {
		t.Errorf("summary = %+v", created)
	}

	// Now break things.
	config.UpdateHue("10.0.0.6", "key-after")
	config.UpdateLoxone(config.LoxoneConfig{Miniservers: []config.MiniserverConfig{}})
	after := []models.Mapping{{ID: "c", LoxoneID: "neu", HueID: "3", HueType: "light", Enabled: true}}
	config.UpdateMappings(after)
	h.mappingManager.Load(after)

	rec = httptest.NewRecorder()
	req := mux.SetURLVars(httptest.NewRequest("POST", "/api/backups/"+created.ID+"/restore", nil),
		map[string]string{"id": created.ID})
	h.RestoreBackup(rec, req)
	if rec.Code != 200 {
		t.Fatalf("restore: status = %d, body %s", rec.Code, rec.Body.String())
	}

	if hue := config.GetHue(); hue.BridgeIP != "10.0.0.5" || hue.ApplicationKey != "key-before" {
		t.Errorf("hue after restore = %+v, want 10.0.0.5/key-before", hue)
	}
	if ms := config.GetLoxone().Miniservers; len(ms) != 1 || ms[0].ID != "ms1" {
		t.Errorf("miniservers after restore = %+v", ms)
	}
	if got := config.GetMappings(); len(got) != 2 || got[0].LoxoneID != "wz" {
		t.Errorf("mappings after restore = %+v", got)
	}
	if h.mappingManager.GetByLoxoneID("kueche") == nil || h.mappingManager.GetByLoxoneID("neu") != nil {
		t.Error("mapping manager was not reloaded from the backup")
	}
	if h.hueClient.BridgeIP() != "10.0.0.5" {
		t.Errorf("hue client points at %q after restore, want 10.0.0.5", h.hueClient.BridgeIP())
	}

	// The restored state must survive a restart.
	data, err := os.ReadFile(config.GetPath())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(data), "key-before") || strings.Contains(string(data), "neu") {
		t.Errorf("saved config does not reflect the restore:\n%s", data)
	}
}

// TestBackupIDsAreValidated: the backup id ends up in a filename, so anything
// but a UUID is refused before it can leave the backups directory, and an
// uploaded file's id is replaced rather than trusted.
func TestBackupIDsAreValidated(t *testing.T) {
	h := newTestHandlers(t)

	for _, id := range []string{"../../etc/passwd", "backup-x", "", "123"} {
		for name, handler := range map[string]func(*httptest.ResponseRecorder, string){
			"restore":  func(rec *httptest.ResponseRecorder, id string) { h.RestoreBackup(rec, idReq("POST", id)) },
			"download": func(rec *httptest.ResponseRecorder, id string) { h.DownloadBackup(rec, idReq("GET", id)) },
			"delete":   func(rec *httptest.ResponseRecorder, id string) { h.DeleteBackup(rec, idReq("DELETE", id)) },
		} {
			rec := httptest.NewRecorder()
			handler(rec, id)
			if rec.Code != 400 {
				t.Errorf("%s with id %q: status = %d, want 400", name, id, rec.Code)
			}
		}
	}

	// A well-formed but unknown id is a 404, not a 400.
	rec := httptest.NewRecorder()
	h.DownloadBackup(rec, idReq("GET", "00000000-0000-4000-8000-000000000000"))
	if rec.Code != 404 {
		t.Errorf("unknown id: status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.UploadBackup(rec, httptest.NewRequest("POST", "/api/backups/upload",
		strings.NewReader(`{"id":"../evil","version":"1.0","config":{"mappings":[]}}`)))
	if rec.Code != 201 {
		t.Fatalf("upload: status = %d, body %s", rec.Code, rec.Body.String())
	}
	var uploaded BackupSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if uploaded.ID == "../evil" || len(uploaded.ID) != 36 {
		t.Errorf("uploaded backup kept the client-supplied id %q", uploaded.ID)
	}
	entries, err := os.ReadDir(backupsDir())
	if err != nil {
		t.Fatalf("read backups dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "backup-"+uploaded.ID+".json" {
		t.Errorf("backups dir holds %v, want exactly backup-%s.json", entries, uploaded.ID)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(backupsDir()), "evil")); err == nil {
		t.Error("upload wrote outside the backups directory")
	}

	// Upload without config data is refused.
	rec = httptest.NewRecorder()
	h.UploadBackup(rec, httptest.NewRequest("POST", "/api/backups/upload", strings.NewReader(`{"version":"1.0"}`)))
	if rec.Code != 400 {
		t.Errorf("upload without mappings: status = %d, want 400", rec.Code)
	}
}

func idReq(method, id string) *http.Request {
	return mux.SetURLVars(httptest.NewRequest(method, "/api/backups/"+id, nil), map[string]string{"id": id})
}
