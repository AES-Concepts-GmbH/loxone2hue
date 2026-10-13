package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
)

// TestRequestBodyLimit: an oversized upload is refused with 413 before any
// handler buffers it, while a normal-sized request still reaches the handler.
func TestRequestBodyLimit(t *testing.T) {
	if _, err := config.Load(t.TempDir() + "/config.yaml"); err != nil {
		t.Fatalf("load config: %v", err)
	}
	config.UpdateAuth(config.AuthConfig{})
	config.UpdateMappings(nil) // the config singleton outlives earlier tests

	srv := NewServer(hue.NewClient("", ""), loxone.NewMappingManager(), loxone.NewUDPSender(), loxone.NewHTTPSender())

	// A syntactically valid backup padded beyond the limit with a long remark.
	huge := `{"version":"1.0","remark":"` + strings.Repeat("x", maxRequestBody) + `","config":{"mappings":[]}}`
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest("POST", "/api/backups/upload", strings.NewReader(huge)))
	if rec.Code != 413 {
		t.Errorf("oversized upload: status = %d, want 413 (body %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest("POST", "/api/mappings",
		strings.NewReader(strings.Repeat(" ", 4096)+`{"loxone_id":"wz","hue_id":"1","hue_type":"light"}`)))
	if rec.Code != 201 {
		t.Errorf("normal request: status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}

	// Oversized on the raw-body path (CreateMapping reads the body itself).
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest("POST", "/api/mappings",
		strings.NewReader(`{"loxone_id":"k","hue_id":"2","hue_type":"light","description":"`+strings.Repeat("y", maxRequestBody)+`"}`)))
	if rec.Code != 413 {
		t.Errorf("oversized mapping: status = %d, want 413 (body %s)", rec.Code, rec.Body.String())
	}
}
