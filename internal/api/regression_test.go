package api

import (
	"context"
	"fmt"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/gorilla/websocket"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAuthStatusDoesNotVerifyBasicPasswords(t *testing.T) {
	withAuth(t, "admin", "geheim123")
	s := newAuthTestServer(t)
	r := httptest.NewRequest("GET", "/api/auth/status", nil)
	r.SetBasicAuth("admin", "geheim123")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), `"authenticated":false`) {
		t.Fatal(w.Body.String())
	}
}
func TestConcurrentMappingCreatesArePreserved(t *testing.T) {
	h := newTestHandlers(t)
	config.UpdateMappings(nil)
	var wg sync.WaitGroup
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			h.CreateMapping(w, httptest.NewRequest("POST", "/api/mappings", strings.NewReader(fmt.Sprintf(`{"loxone_id":"l%d","hue_id":"h%d","hue_type":"light"}`, i, i))))
			if w.Code != 201 {
				t.Errorf("create: %d %s", w.Code, w.Body.String())
			}
		}(i)
	}
	wg.Wait()
	if n := len(config.GetMappings()); n != 15 {
		t.Fatalf("got %d mappings", n)
	}
}
func TestCreateRejectsInvalidMapping(t *testing.T) {
	h := newTestHandlers(t)
	w := httptest.NewRecorder()
	h.CreateMapping(w, httptest.NewRequest("POST", "/api/mappings", strings.NewReader(`{"hue_type":"invalid"}`)))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func TestRevokedSocketCannotExecuteCommands(t *testing.T) {
	withAuth(t, "admin", "geheim123")
	s := newAuthTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.wsHub.Run(ctx)
	server := httptest.NewServer(s.router)
	defer server.Close()
	token, _ := sessions.create()
	headers := make(map[string][]string)
	headers["Cookie"] = []string{sessionCookieName + "=" + token}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sessions.revoke(token)
	if err := conn.WriteMessage(websocket.TextMessage, []byte("STATUS unknown")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("revoked socket received a command response")
	}
}
func TestImportDeduplicatesNewTargets(t *testing.T) {
	h := newTestHandlers(t)
	w := httptest.NewRecorder()
	h.ImportMappings(w, httptest.NewRequest("POST", "/api/mappings/import", strings.NewReader(`{"mode":"merge","backup":{"version":"1","mappings":[{"loxone_id":"a","hue_id":"1","hue_type":"light"},{"loxone_id":"a","hue_id":"2","hue_type":"light"}]}}`)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if m := config.GetMappings(); len(m) != 1 || m[0].HueID != "2" {
		t.Fatal(m)
	}
}

func TestSettingsAndImportRollbackOnSaveFailure(t *testing.T) {
	h := newTestHandlers(t)
	previous := config.GetLoxone()
	breakConfigStorage(t)
	w := httptest.NewRecorder()
	h.UpdateConfig(w, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"loxone":{"miniservers":[{"id":"unsaved","udp_enabled":true,"ip":"127.0.0.1","port":7777}]}}`)))
	if w.Code != 500 {
		t.Fatal(w.Body.String())
	}
	if len(config.GetLoxone().Miniservers) != len(previous.Miniservers) {
		t.Fatal("settings were not rolled back")
	}
	w = httptest.NewRecorder()
	h.ImportMappings(w, httptest.NewRequest("POST", "/api/mappings/import", strings.NewReader(`{"mode":"replace","backup":{"version":"1","mappings":[{"loxone_id":"a","hue_id":"1","hue_type":"light","enabled":true}]}}`)))
	if w.Code != 500 {
		t.Fatal(w.Body.String())
	}
	if len(config.GetMappings()) != 0 || h.mappingManager.GetByLoxoneID("a") != nil {
		t.Fatal("import was not rolled back")
	}
}
