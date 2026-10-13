package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
)

func newAuthTestServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(hue.NewClient("", ""), loxone.NewMappingManager(), loxone.NewUDPSender(), loxone.NewHTTPSender())
}

func sessionCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	return nil
}

// TestLoginIssuesSessionAndLogoutRevokesIt covers the Web UI's login form:
// the status endpoint is open, a correct login hands out a HttpOnly,
// SameSite=Strict cookie with an expiry, the cookie authenticates the API and
// the WebSocket, and logout invalidates it server-side.
func TestLoginIssuesSessionAndLogoutRevokesIt(t *testing.T) {
	withAuth(t, "admin", "geheim123")
	srv := newAuthTestServer(t)

	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/status", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":true`) || !strings.Contains(rec.Body.String(), `"authenticated":false`) {
		t.Fatalf("status without login: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"falsch"}`))
	req.Header.Set(uiRequestHeader, "loxone2hue-ui")
	srv.router.ServeHTTP(rec, req)
	if rec.Code != 401 || rec.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("wrong password: status %d, WWW-Authenticate %q (the UI must not get a browser dialog)", rec.Code, rec.Header().Get("WWW-Authenticate"))
	}

	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"geheim123"}`)))
	if rec.Code != 200 {
		t.Fatalf("login: status %d %s", rec.Code, rec.Body.String())
	}
	cookie := sessionCookieFrom(t, rec)
	if cookie == nil {
		t.Fatal("login issued no session cookie")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie flags: HttpOnly=%v SameSite=%v, want HttpOnly and Strict", cookie.HttpOnly, cookie.SameSite)
	}
	if cookie.Expires.Before(time.Now().Add(23*time.Hour)) || cookie.Expires.After(time.Now().Add(25*time.Hour)) {
		t.Errorf("cookie expiry %v, want about 24 h from now", cookie.Expires)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/auth/status", nil)
	req.AddCookie(cookie)
	srv.router.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"authenticated":true`) {
		t.Errorf("status with cookie: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/config", nil)
	req.AddCookie(cookie)
	srv.router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("API with session cookie: status %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(cookie)
	srv.router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("logout: status %d", rec.Code)
	}
	if c := sessionCookieFrom(t, rec); c == nil || c.MaxAge >= 0 {
		t.Error("logout did not clear the cookie")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/config", nil)
	req.AddCookie(cookie)
	srv.router.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("session still valid after logout: status %d", rec.Code)
	}
}

// TestSessionsExpire: a session older than its TTL is refused, and the UI
// request header suppresses the Basic Auth challenge.
func TestSessionsExpire(t *testing.T) {
	withAuth(t, "admin", "geheim123")

	token, _ := sessions.create()
	sessions.mu.Lock()
	sessions.sessions[token] = time.Now().Add(-time.Minute)
	sessions.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/config", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	req.Header.Set(uiRequestHeader, "loxone2hue-ui")
	rec := httptest.NewRecorder()
	protectedHandler().ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("expired session accepted: status %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") != "" {
		t.Error("UI request received a Basic Auth challenge")
	}

	// Plain API clients still get the challenge.
	rec = httptest.NewRecorder()
	protectedHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/config", nil))
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("API client did not receive a Basic Auth challenge")
	}
}

// TestLoginLockout: from the fifth failure on, an address is blocked for 30 s
// with a Retry-After header, on the login form and on Basic Auth alike, and
// the correct password is refused as well while the block lasts. A success
// clears the counter.
func TestLoginLockout(t *testing.T) {
	withAuth(t, "admin", "geheim123")
	srv := newAuthTestServer(t)

	attempt := func(password, ip string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"`+password+`"}`))
		req.RemoteAddr = ip + ":40000"
		srv.router.ServeHTTP(rec, req)
		return rec
	}

	for i := 1; i <= lockoutThreshold; i++ {
		if rec := attempt("falsch", "10.1.1.1"); rec.Code != 401 {
			t.Fatalf("attempt %d: status %d, want 401", i, rec.Code)
		}
	}
	rec := attempt("geheim123", "10.1.1.1")
	if rec.Code != 429 {
		t.Fatalf("blocked address: status %d, want 429 (%s)", rec.Code, rec.Body.String())
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" {
		t.Error("429 without Retry-After")
	}

	// Basic Auth from the same address is blocked too.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/config", nil)
	req.RemoteAddr = "10.1.1.1:40001"
	req.SetBasicAuth("admin", "geheim123")
	srv.router.ServeHTTP(rec, req)
	if rec.Code != 429 {
		t.Errorf("Basic Auth from blocked address: status %d, want 429", rec.Code)
	}

	// Another address is unaffected, and its success clears its own record.
	if rec := attempt("falsch", "10.1.1.2"); rec.Code != 401 {
		t.Errorf("other address: status %d, want 401", rec.Code)
	}
	if rec := attempt("geheim123", "10.1.1.2"); rec.Code != 200 {
		t.Errorf("other address login: status %d, want 200", rec.Code)
	}
	if _, blocked := limiter.blocked("10.1.1.2"); blocked {
		t.Error("record not cleared after a successful login")
	}

	// The block doubles with further failures and is capped.
	limiter.mu.Lock()
	e := limiter.entries["10.1.1.1"]
	e.failures = 100
	limiter.mu.Unlock()
	limiter.failure("10.1.1.1")
	if wait, _ := limiter.blocked("10.1.1.1"); wait > lockoutMax {
		t.Errorf("block %v exceeds the cap %v", wait, lockoutMax)
	}
}

// TestUpdateAuthFromSettings covers the settings page: enabling a login on an
// open installation, changing it with the current password, refusing a wrong
// current password or a short new one, and switching the login off again.
func TestUpdateAuthFromSettings(t *testing.T) {
	withAuth(t, "", "")
	srv := newAuthTestServer(t)

	put := func(body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/auth", strings.NewReader(body))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		srv.router.ServeHTTP(rec, req)
		return rec
	}

	if rec := put(`{"new_password":"kurz"}`, nil); rec.Code != 400 {
		t.Errorf("short password: status %d, want 400", rec.Code)
	}

	rec := put(`{"username":"stefan","new_password":"geheim123"}`, nil)
	if rec.Code != 200 {
		t.Fatalf("enable login: status %d %s", rec.Code, rec.Body.String())
	}
	cookie := sessionCookieFrom(t, rec)
	if cookie == nil {
		t.Fatal("enabling the login issued no session for the caller")
	}
	auth := config.GetAuth()
	if !auth.Enabled() || auth.User() != "stefan" || auth.Password != "" || !strings.HasPrefix(auth.PasswordHash, "$2") {
		t.Errorf("stored auth = %+v, want bcrypt hash for stefan and no plain text", auth)
	}

	// Without a session the API is now closed.
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest("GET", "/api/config", nil))
	if rec.Code != 401 {
		t.Errorf("API open after enabling login: status %d", rec.Code)
	}

	if rec := put(`{"current_password":"falsch","new_password":"nochgeheimer"}`, cookie); rec.Code != 403 {
		t.Errorf("wrong current password: status %d, want 403", rec.Code)
	}

	rec = put(`{"current_password":"geheim123","new_password":"nochgeheimer"}`, cookie)
	if rec.Code != 200 {
		t.Fatalf("change password: status %d %s", rec.Code, rec.Body.String())
	}
	fresh := sessionCookieFrom(t, rec)
	if fresh == nil || fresh.Value == cookie.Value {
		t.Error("password change did not issue a fresh session")
	}
	// The previous session is gone, the new one works.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/config", nil)
	req.AddCookie(cookie)
	srv.router.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("old session survived the password change: status %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/config", nil)
	req.AddCookie(fresh)
	srv.router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("fresh session rejected: status %d", rec.Code)
	}
	if !config.GetAuth().Verify("nochgeheimer") || config.GetAuth().Verify("geheim123") {
		t.Error("stored hash does not match the new password")
	}

	rec = put(`{"current_password":"nochgeheimer","disable":true}`, fresh)
	if rec.Code != 200 {
		t.Fatalf("disable: status %d %s", rec.Code, rec.Body.String())
	}
	if config.GetAuth().Enabled() {
		t.Error("login still enabled after disable")
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["enabled"] != false {
		t.Errorf("disable response = %v", body)
	}
}

// TestBridgeAddressesMustBePrivate: the bridge test and the pairing open
// connections to the supplied address, so only LAN addresses are accepted.
func TestBridgeAddressesMustBePrivate(t *testing.T) {
	for host, want := range map[string]bool{
		"192.168.1.20":    true,
		"10.0.0.5:443":    true,
		"172.16.4.1":      true,
		"127.0.0.1:1":     true,
		"localhost":       true,
		"169.254.10.10":   true,
		"[fd00::1]":       true,
		"8.8.8.8":         false,
		"1.1.1.1:443":     false,
		"2001:4860::8888": false,
		"":                false,
	} {
		if got := isPrivateHost(host); got != want {
			t.Errorf("isPrivateHost(%q) = %v, want %v", host, got, want)
		}
	}

	h := newTestHandlers(t)
	for _, path := range []string{"/api/bridge/test", "/api/bridge/pair"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, strings.NewReader(`{"bridge_ip":"8.8.8.8"}`))
		if path == "/api/bridge/test" {
			h.TestBridgeConnection(rec, req)
		} else {
			h.PairBridge(rec, req)
		}
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "local network") {
			t.Errorf("%s with public address: status %d %s, want 400", path, rec.Code, rec.Body.String())
		}
	}
}
