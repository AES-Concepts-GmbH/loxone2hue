package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
)

// ingressRequest builds a request as the Supervisor sends it.
func ingressRequest(method, path, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	req.RemoteAddr = DefaultIngressProxyIP + ":41234"
	req.Header.Set("X-Remote-User-Name", "stefan")
	req.Header.Set("X-Remote-User-Display-Name", "Stefan Beyeler")
	return req
}

func TestIngressSkipsLoginForSupervisor(t *testing.T) {
	withAuth(t, "admin", "geheim123")
	srv := newAuthTestServer(t)
	ingress := ingressMiddleware(DefaultIngressProxyIP, srv.router)

	rec := httptest.NewRecorder()
	ingress.ServeHTTP(rec, ingressRequest("GET", "/api/config", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("Ingress request: status %d, want 200 without gateway login", rec.Code)
	}

	rec = httptest.NewRecorder()
	ingress.ServeHTTP(rec, ingressRequest("GET", "/api/auth/status", ""))
	var status map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["authenticated"] != true || status["ha_login"] != true || status["ha_user"] != "Stefan Beyeler" {
		t.Errorf("auth status via Ingress: %v", status)
	}
}

func TestIngressPortRefusesOtherClients(t *testing.T) {
	withAuth(t, "", "")
	srv := newAuthTestServer(t)
	ingress := ingressMiddleware(DefaultIngressProxyIP, srv.router)

	req := ingressRequest("GET", "/api/health", "")
	req.RemoteAddr = "10.1.19.20:5000"
	rec := httptest.NewRecorder()
	ingress.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("foreign client on the Ingress port: status %d, want 403", rec.Code)
	}
}

// The main port ignores the Supervisor's headers and address: only the
// Ingress listener marks a request as signed in to Home Assistant.
func TestMainPortIgnoresIngressHeaders(t *testing.T) {
	withAuth(t, "admin", "geheim123")
	srv := newAuthTestServer(t)

	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, ingressRequest("GET", "/api/config", ""))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("forged Ingress request on the main port: status %d, want 401", rec.Code)
	}
}

func TestIngressLoginRequiredSetting(t *testing.T) {
	withAuth(t, "admin", "geheim123")
	srv := newAuthTestServer(t)
	ingress := ingressMiddleware(DefaultIngressProxyIP, srv.router)

	put := func(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// Direct access has to confirm the password.
	direct := httptest.NewRequest("PUT", "/api/auth", strings.NewReader(`{"trust_ingress":false}`))
	direct.SetBasicAuth("admin", "geheim123")
	if rec := put(srv.router, direct); rec.Code != http.StatusForbidden {
		t.Errorf("direct change without current password: status %d, want 403", rec.Code)
	}

	// Through Ingress the Home Assistant sign-in is enough.
	if rec := put(ingress, ingressRequest("PUT", "/api/auth", `{"trust_ingress":false}`)); rec.Code != http.StatusOK {
		t.Fatalf("change via Ingress: status %d %s", rec.Code, rec.Body.String())
	}
	if !config.GetAuth().IngressLoginRequired {
		t.Fatal("ingress_login_required not stored")
	}
	if rec := put(ingress, ingressRequest("GET", "/api/config", "")); rec.Code != http.StatusUnauthorized {
		t.Errorf("Ingress with login required: status %d, want 401", rec.Code)
	}

	// A password change keeps the setting.
	change := httptest.NewRequest("PUT", "/api/auth",
		strings.NewReader(`{"current_password":"geheim123","new_password":"anderes123"}`))
	change.SetBasicAuth("admin", "geheim123")
	if rec := put(srv.router, change); rec.Code != http.StatusOK {
		t.Fatalf("password change: status %d %s", rec.Code, rec.Body.String())
	}
	if !config.GetAuth().IngressLoginRequired {
		t.Error("password change reset ingress_login_required")
	}

	back := httptest.NewRequest("PUT", "/api/auth",
		strings.NewReader(`{"trust_ingress":true,"current_password":"anderes123"}`))
	back.SetBasicAuth("admin", "anderes123")
	if rec := put(srv.router, back); rec.Code != http.StatusOK {
		t.Fatalf("switch back directly: status %d %s", rec.Code, rec.Body.String())
	}
	if !config.GetAuth().TrustsIngress() {
		t.Error("trust_ingress not restored")
	}
}
