package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/gorilla/websocket"
)

// withAuth configures a password for the duration of a test.
func withAuth(t *testing.T, username, password string) {
	t.Helper()

	if _, err := config.Load(filepath.Join(t.TempDir(), "config.yaml")); err != nil {
		t.Fatalf("load config: %v", err)
	}
	config.UpdateAuth(config.AuthConfig{Username: username, Password: password})
	t.Cleanup(func() { config.UpdateAuth(config.AuthConfig{}) })
}

func protectedHandler() http.Handler {
	return authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("secret"))
	}))
}

func TestAuthDisabledLetsEverythingThrough(t *testing.T) {
	withAuth(t, "", "")

	rec := httptest.NewRecorder()
	protectedHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/config", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when no password is configured", rec.Code)
	}
}

func TestAuthRejectsMissingAndWrongCredentials(t *testing.T) {
	withAuth(t, "admin", "geheim")

	cases := []struct {
		name       string
		user, pass string
		useBasic   bool
	}{
		{"ohne Anmeldung", "", "", false},
		{"falsches Passwort", "admin", "falsch", true},
		{"falscher Benutzer", "root", "geheim", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/config", nil)
			if tc.useBasic {
				req.SetBasicAuth(tc.user, tc.pass)
			}

			rec := httptest.NewRecorder()
			protectedHandler().ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Error("handler ran despite failed authentication")
			}
		})
	}
}

func TestAuthAcceptsCorrectCredentialsAndIssuesSession(t *testing.T) {
	withAuth(t, "admin", "geheim")

	req := httptest.NewRequest("GET", "/api/config", nil)
	req.SetBasicAuth("admin", "geheim")

	rec := httptest.NewRecorder()
	protectedHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			session = c
		}
	}
	if session == nil {
		t.Fatal("no session cookie issued")
	}
	if !session.HttpOnly {
		t.Error("session cookie is not HttpOnly")
	}
	// SameSite=Strict is what stops a foreign page from driving GET /ws?cmd=.
	if session.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", session.SameSite)
	}

	// The cookie alone must authenticate the next request.
	follow := httptest.NewRequest("GET", "/api/config", nil)
	follow.AddCookie(session)

	rec2 := httptest.NewRecorder()
	protectedHandler().ServeHTTP(rec2, follow)
	if rec2.Code != http.StatusOK {
		t.Errorf("session cookie was not accepted: status %d", rec2.Code)
	}
}

func TestAuthRejectsForgedSessionCookie(t *testing.T) {
	withAuth(t, "admin", "geheim")

	req := httptest.NewRequest("GET", "/api/config", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "geraten"})

	rec := httptest.NewRecorder()
	protectedHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for a forged cookie", rec.Code)
	}
}

// TestHealthStaysOpen keeps the container health check working.
func TestHealthStaysOpen(t *testing.T) {
	withAuth(t, "admin", "geheim")

	rec := httptest.NewRecorder()
	protectedHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 — the Docker HEALTHCHECK has no credentials", rec.Code)
	}
}

// TestWebSocketAuthentication is the reason the session cookie exists: browsers
// do not attach basic auth to a WebSocket handshake, so the dashboard would
// lose live updates if the cookie were not accepted here.
func TestWebSocketAuthentication(t *testing.T) {
	withAuth(t, "admin", "geheim")

	hub := newTestHub()
	srv := httptest.NewServer(authMiddleware(http.HandlerFunc(hub.HandleWebSocket)))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	t.Run("ohne Anmeldung abgewiesen", func(t *testing.T) {
		conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
		if err == nil {
			conn.Close()
			t.Fatal("unauthenticated WebSocket handshake succeeded")
		}
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %v, want 401", resp)
		}
	})

	t.Run("mit Session-Cookie akzeptiert", func(t *testing.T) {
		header := http.Header{}
		header.Set("Cookie", sessionCookieName+"="+sessionToken)

		conn, _, err := websocket.DefaultDialer.Dial(url, header)
		if err != nil {
			t.Fatalf("handshake with session cookie failed: %v", err)
		}
		conn.Close()
	})

	t.Run("mit Basic Auth akzeptiert", func(t *testing.T) {
		header := http.Header{}
		header.Set("Authorization", "Basic YWRtaW46Z2VoZWlt") // admin:geheim

		conn, _, err := websocket.DefaultDialer.Dial(url, header)
		if err != nil {
			t.Fatalf("handshake with basic auth failed: %v", err)
		}
		conn.Close()
	})
}

// TestCommandEndpointRequiresAuth covers the CSRF-prone GET command path.
func TestCommandEndpointRequiresAuth(t *testing.T) {
	withAuth(t, "admin", "geheim")

	hub := newTestHub()
	handler := authMiddleware(http.HandlerFunc(hub.HandleWebSocket))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/ws?cmd=SET+wohnzimmer+ON", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 — /ws?cmd= must not run unauthenticated", rec.Code)
	}
}
