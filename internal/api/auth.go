package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
)

const sessionCookieName = "loxone2hue_session"

// sessionToken is generated once per process. It is handed out as a cookie
// after a successful Basic Auth challenge and invalidated by a restart.
//
// The cookie exists because of the WebSocket: browsers do not attach Basic Auth
// credentials to a WebSocket handshake, so a Basic-only setup would leave the
// dashboard without live updates. Cookies are sent on the handshake, and
// SameSite=Strict keeps them off cross-site requests — which also closes the
// CSRF path through GET /ws?cmd=.
var sessionToken = mustRandomToken()

func mustRandomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("failed to generate session token: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

// isAuthExempt reports whether a path is reachable without credentials.
func isAuthExempt(path string) bool {
	// The container health check runs without credentials and the response
	// carries no configuration detail.
	return path == "/api/health"
}

// authMiddleware guards every route when a password is configured.
//
// With no password set it is a pass-through, so existing installations and the
// Home Assistant Ingress setup keep working exactly as before.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := config.GetAuth()
		if !auth.Enabled() || isAuthExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// CORS preflight carries no credentials by definition.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if hasValidSession(r) {
			next.ServeHTTP(w, r)
			return
		}

		user, pass, ok := r.BasicAuth()
		if ok && credentialsMatch(auth, user, pass) {
			// Hand out a session so the WebSocket handshake authenticates too.
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    sessionToken,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="Loxone2HUE", charset="UTF-8"`)
		errorResponse(w, http.StatusUnauthorized, "authentication required")
	})
}

// hasValidSession reports whether the request carries the session cookie.
func hasValidSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(sessionToken)) == 1
}

// credentialsMatch compares user and password without leaking timing.
func credentialsMatch(auth config.AuthConfig, user, pass string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(auth.User())) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(auth.Password)) == 1
	return userOK && passOK
}

// authStatus describes the current protection state for the UI and the logs.
func authStatus() string {
	if config.GetAuth().Enabled() {
		return "enabled"
	}
	return "disabled"
}

// isLoopback reports whether the server only listens on the loopback interface.
func isLoopback(host string) bool {
	return host == "127.0.0.1" || host == "::1" || strings.EqualFold(host, "localhost")
}
