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
	ingress := isIngressRequest(r)
	viaHA := ingress && auth.TrustsIngress()
	authenticated := !auth.Enabled() || viaHA || hasValidSession(r)
	status := map[string]interface{}{
		"enabled":       auth.Enabled(),
		"configured":    auth.Configured(),
		"username":      auth.User(),
		"authenticated": authenticated,
		"ingress":       ingress,
		"ha_login":      viaHA,
	}
	if ingress {
		status["ha_user"] = ingressUser(r)
	}
	jsonResponse(w, http.StatusOK, status)
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

// UpdateAuth sets or changes the password and switches the login on or off
// from the settings page.
//
//   - {"new_password", "username"}: sets or changes the password. A first
//     password switches the protection on; changing it keeps the current
//     on/off state.
//   - {"enabled": false}: switches the protection off and keeps the
//     credentials ("disable": true is the older spelling).
//   - {"enabled": true}: switches it on again with the stored password.
//
// While the protection is on, the current password has to be supplied; a
// wrong one counts as a failed login for the throttling. Switching on with
// the stored password also checks it, so nobody locks themselves out with a
// password they no longer know. Every browser session is logged out
// afterwards; with the protection on, the caller receives a fresh one.
func (h *Handlers) UpdateAuth(w http.ResponseWriter, r *http.Request) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	var req struct {
		Username        string `json:"username"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		Enabled         *bool  `json:"enabled"`
		Disable         bool   `json:"disable"`
		TrustIngress    *bool  `json:"trust_ingress"`
	}
	if !decodeJSON(w, r, &req, "invalid request body") {
		return
	}
	if req.TrustIngress != nil && req.NewPassword == "" && req.Enabled == nil && !req.Disable {
		updateIngressTrust(w, r, *req.TrustIngress, req.CurrentPassword)
		return
	}
	if req.Disable {
		off := false
		req.Enabled = &off
	}

	current := config.GetAuth()
	toggleOnly := req.NewPassword == "" && req.Enabled != nil
	switchOn := toggleOnly && *req.Enabled

	if switchOn && !current.Configured() {
		errorResponse(w, http.StatusBadRequest, "no password set: set a password to enable the login")
		return
	}
	if current.Enabled() || switchOn {
		if !verifyCurrentPassword(w, r, current, req.CurrentPassword) {
			return
		}
	}

	next := current
	if toggleOnly {
		next.Disabled = !*req.Enabled
	} else {
		if len(req.NewPassword) < minPasswordLength {
			errorResponse(w, http.StatusBadRequest, "new password must have at least 8 characters")
			return
		}
		hash, err := config.HashPassword(req.NewPassword)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "password cannot be hashed")
			return
		}
		username := strings.TrimSpace(req.Username)
		if username == "" {
			username = current.User()
		}
		// A first password switches the protection on; a change keeps the state.
		disabled := current.Configured() && current.Disabled
		if req.Enabled != nil {
			disabled = !*req.Enabled
		}
		next = config.AuthConfig{
			Username:             username,
			PasswordHash:         hash,
			Disabled:             disabled,
			IngressLoginRequired: current.IngressLoginRequired,
		}
	}

	candidate := config.Snapshot()
	candidate.Auth = next
	if !commitConfig(w, candidate) {
		return
	}

	// Old sessions belong to the old password or the old state.
	sessions.revokeAll()

	switch {
	case toggleOnly && next.Enabled():
		log.Info().Str("user", next.User()).Msg("Login protection enabled via settings")
	case toggleOnly:
		log.Info().Msg("Login protection disabled via settings (credentials kept)")
	default:
		log.Info().Str("user", next.User()).Bool("enabled", next.Enabled()).Msg("Login password changed via settings")
	}

	if !next.Enabled() {
		clearSessionCookie(w)
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"status":     "ok",
			"enabled":    false,
			"configured": next.Configured(),
			"username":   next.User(),
		})
		return
	}

	token, expires := sessions.create()
	setSessionCookie(w, r, token, expires)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":     "ok",
		"enabled":    true,
		"configured": true,
		"username":   next.User(),
	})
}

// updateIngressTrust switches whether the Home Assistant sign-in replaces the
// gateway login in the sidebar. A request through Ingress is signed in to
// Home Assistant and needs no password; a direct one has to confirm the
// password while the protection is on. Sessions stay valid: the change does
// not touch the password.
func updateIngressTrust(w http.ResponseWriter, r *http.Request, trust bool, currentPassword string) {
	current := config.GetAuth()
	if current.Enabled() && !isIngressRequest(r) {
		if !verifyCurrentPassword(w, r, current, currentPassword) {
			return
		}
	}

	next := current
	next.IngressLoginRequired = !trust
	candidate := config.Snapshot()
	candidate.Auth = next
	if !commitConfig(w, candidate) {
		return
	}
	log.Info().Bool("trust_ingress", trust).Msg("Home Assistant sign-in for Ingress changed via settings")
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"enabled":       next.Enabled(),
		"configured":    next.Configured(),
		"username":      next.User(),
		"trust_ingress": next.TrustsIngress(),
	})
}

// verifyCurrentPassword checks the password confirming a change, throttled
// like a login. It answers the request itself when the check fails.
func verifyCurrentPassword(w http.ResponseWriter, r *http.Request, auth config.AuthConfig, password string) bool {
	ip := clientIP(r)
	if wait, blocked := limiter.blocked(ip); blocked {
		tooManyAttempts(w, wait)
		return false
	}
	if !auth.Verify(password) {
		limiter.failure(ip)
		errorResponse(w, http.StatusForbidden, "current password is wrong")
		return false
	}
	limiter.success(ip)
	return true
}
