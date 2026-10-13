package api

import (
	"net/http"
	"strings"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/rs/zerolog/log"
)

// minPasswordLength is what PUT /api/auth insists on for a new password.
const minPasswordLength = 8

// AuthStatus tells the Web UI whether a login is required and whether the
// caller already holds one. Reachable without credentials; it reveals only
// the username, which the Basic Auth realm would prompt for anyway.
func (h *Handlers) AuthStatus(w http.ResponseWriter, r *http.Request) {
	auth := config.GetAuth()
	authenticated := !auth.Enabled() || hasValidSession(r)
	if !authenticated {
		if user, pass, ok := r.BasicAuth(); ok {
			authenticated = credentialsMatch(auth, user, pass)
		}
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"enabled":       auth.Enabled(),
		"username":      auth.User(),
		"authenticated": authenticated,
	})
}

// Login checks a username and password from the Web UI's login form and
// issues the session cookie. Throttled like Basic Auth.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req, "invalid request body") {
		return
	}

	auth := config.GetAuth()
	if !auth.Enabled() {
		jsonResponse(w, http.StatusOK, map[string]interface{}{"status": "ok", "enabled": false})
		return
	}

	ip := clientIP(r)
	if wait, blocked := limiter.blocked(ip); blocked {
		tooManyAttempts(w, wait)
		return
	}
	if !credentialsMatch(auth, req.Username, req.Password) {
		limiter.failure(ip)
		log.Warn().Str("ip", ip).Str("user", req.Username).Msg("Login failed")
		errorResponse(w, http.StatusUnauthorized, "wrong username or password")
		return
	}
	limiter.success(ip)

	token, expires := sessions.create()
	setSessionCookie(w, r, token, expires)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"enabled":  true,
		"username": auth.User(),
	})
}

// Logout revokes the caller's session and clears the cookie.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		sessions.revoke(cookie.Value)
	}
	clearSessionCookie(w)
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateAuth sets, changes or disables the login from the settings page.
//
// With a login active, the current password has to be supplied; a wrong one
// counts as a failed login for the throttling. Every other browser session is
// logged out afterwards, the caller receives a fresh one.
func (h *Handlers) UpdateAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username        string `json:"username"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		Disable         bool   `json:"disable"`
	}
	if !decodeJSON(w, r, &req, "invalid request body") {
		return
	}

	current := config.GetAuth()
	if current.Enabled() {
		ip := clientIP(r)
		if wait, blocked := limiter.blocked(ip); blocked {
			tooManyAttempts(w, wait)
			return
		}
		if !current.Verify(req.CurrentPassword) {
			limiter.failure(ip)
			errorResponse(w, http.StatusForbidden, "current password is wrong")
			return
		}
		limiter.success(ip)
	}

	var next config.AuthConfig
	if !req.Disable {
		if len(req.NewPassword) < minPasswordLength {
			errorResponse(w, http.StatusBadRequest, "new password must have at least 8 characters")
			return
		}
		username := strings.TrimSpace(req.Username)
		if username == "" {
			username = current.User()
		}
		next = config.AuthConfig{Username: username, Password: req.NewPassword}
	}

	if err := config.UpdateAuth(next); err != nil {
		errorResponse(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if !saveConfig(w, "login settings", func() { _ = config.UpdateAuth(current) }) {
		return
	}

	// Old sessions belong to the old password.
	sessions.revokeAll()

	if req.Disable {
		clearSessionCookie(w)
		log.Info().Msg("Login protection disabled via settings")
		jsonResponse(w, http.StatusOK, map[string]interface{}{"status": "ok", "enabled": false})
		return
	}

	token, expires := sessions.create()
	setSessionCookie(w, r, token, expires)
	log.Info().Str("user", next.User()).Msg("Login password changed via settings")
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"enabled":  true,
		"username": next.User(),
	})
}
