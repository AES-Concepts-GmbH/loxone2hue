package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
)

const sessionCookieName = "loxone2hue_session"

// uiRequestHeader marks requests from the bundled Web UI. They get a plain
// 401 instead of a Basic Auth challenge, so the browser shows the UI's login
// form rather than its own credentials dialog.
const uiRequestHeader = "X-Requested-With"

// ---------------------------------------------------------------------------
// Sessions
//
// A session cookie exists because of the WebSocket: browsers do not attach
// Basic Auth credentials to a WebSocket handshake, so a Basic-only setup
// would leave the dashboard without live updates. Cookies are sent on the
// handshake, and SameSite=Strict keeps them off cross-site requests — which
// also closes the CSRF path through GET /ws?cmd=.
//
// Sessions are issued only by POST /api/auth/login, never as a side effect of
// a Basic-Auth request: Loxone sends its credentials with every command and
// would otherwise create a new session per command.

const (
	sessionTTL  = 24 * time.Hour
	maxSessions = 1000
)

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time // token -> expiry
}

var sessions = &sessionStore{sessions: make(map[string]time.Time)}

func mustRandomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("failed to generate session token: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

// create issues a new session and returns its token and expiry.
func (s *sessionStore) create() (string, time.Time) {
	token := mustRandomToken()
	expires := time.Now().Add(sessionTTL)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	if len(s.sessions) >= maxSessions {
		// Evict the session closest to expiry rather than refusing the login.
		var oldest string
		var oldestExp time.Time
		for t, exp := range s.sessions {
			if oldest == "" || exp.Before(oldestExp) {
				oldest, oldestExp = t, exp
			}
		}
		delete(s.sessions, oldest)
	}
	s.sessions[token] = expires
	return token, expires
}

// valid reports whether token names a live session.
func (s *sessionStore) valid(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.sessions, token)
		return false
	}
	return true
}

func (s *sessionStore) revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// revokeAll logs every browser out, used after a password change.
func (s *sessionStore) revokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = make(map[string]time.Time)
}

func (s *sessionStore) pruneLocked() {
	now := time.Now()
	for t, exp := range s.sessions {
		if now.After(exp) {
			delete(s.sessions, t)
		}
	}
}

// hasValidSession reports whether the request carries a live session cookie.
func hasValidSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return sessions.valid(cookie.Value)
}

// setSessionCookie hands a freshly created session to the browser.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearSessionCookie tells the browser to drop the session cookie.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ---------------------------------------------------------------------------
// Login throttling
//
// Failed logins are counted per client address. From the fifth failure on the
// address is blocked for 30 s, doubling with every further failure up to an
// hour; a successful login clears the record. Behind a reverse proxy all
// clients share one address, which then shares one counter.

const (
	lockoutThreshold = 5
	lockoutBase      = 30 * time.Second
	lockoutMax       = time.Hour
)

type loginAttempts struct {
	failures     int
	blockedUntil time.Time
	lastSeen     time.Time
}

type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginAttempts
}

var limiter = newLoginLimiter()

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{entries: make(map[string]*loginAttempts)}
}

// blocked reports how long the address still has to wait, if at all.
func (l *loginLimiter) blocked(ip string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok {
		return 0, false
	}
	if wait := time.Until(e.blockedUntil); wait > 0 {
		return wait, true
	}
	return 0, false
}

// failure records a failed login and extends the block when the threshold is reached.
func (l *loginLimiter) failure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked()
	e, ok := l.entries[ip]
	if !ok {
		e = &loginAttempts{}
		l.entries[ip] = e
	}
	e.failures++
	e.lastSeen = time.Now()
	if e.failures >= lockoutThreshold {
		wait := lockoutBase << uint(e.failures-lockoutThreshold)
		if wait > lockoutMax || wait <= 0 {
			wait = lockoutMax
		}
		e.blockedUntil = time.Now().Add(wait)
	}
}

// success clears the record of an address.
func (l *loginLimiter) success(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

// pruneLocked drops records nobody has touched for the maximum block time.
func (l *loginLimiter) pruneLocked() {
	if len(l.entries) < 1000 {
		return
	}
	cutoff := time.Now().Add(-lockoutMax)
	for ip, e := range l.entries {
		if e.lastSeen.Before(cutoff) && time.Now().After(e.blockedUntil) {
			delete(l.entries, ip)
		}
	}
}

// clientIP returns the remote address without the port.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// tooManyAttempts answers a blocked address.
func tooManyAttempts(w http.ResponseWriter, wait time.Duration) {
	secs := int(wait.Seconds()) + 1
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	errorResponse(w, http.StatusTooManyRequests,
		"too many failed logins, try again in "+strconv.Itoa(secs)+" seconds")
}

// ---------------------------------------------------------------------------
// Credential check

// verifiedCredentials remembers which user:password pairs matched which
// password hash, so Loxone, which sends its credentials with every command,
// pays the bcrypt cost once instead of per request. Keyed by a digest, and
// the stored hash makes entries stale as soon as the password changes.
var verifiedCredentials sync.Map

func credentialKey(user, pass string) [32]byte {
	return sha256.Sum256([]byte(user + "\x00" + pass))
}

// credentialsMatch compares user and password against the configuration.
func credentialsMatch(auth config.AuthConfig, user, pass string) bool {
	if subtle.ConstantTimeCompare([]byte(user), []byte(auth.User())) != 1 {
		return false
	}
	key := credentialKey(user, pass)
	if hash, ok := verifiedCredentials.Load(key); ok && hash == auth.PasswordHash && auth.PasswordHash != "" {
		return true
	}
	if !auth.Verify(pass) {
		return false
	}
	if auth.PasswordHash != "" {
		verifiedCredentials.Store(key, auth.PasswordHash)
	}
	return true
}

// ---------------------------------------------------------------------------
// Middleware

// isAuthExempt reports whether a path is reachable without credentials.
func isAuthExempt(path string) bool {
	switch path {
	case "/api/health":
		// The container health check runs without credentials and the
		// response carries no configuration detail.
		return true
	case "/api/auth/status", "/api/auth/login":
		// The login form needs to know whether a login is required, and the
		// login itself checks the credentials in its handler.
		return true
	}
	return false
}

// authMiddleware guards every route when a password is configured.
//
// With no password set it is a pass-through, so existing installations and the
// Home Assistant Ingress setup keep working exactly as before. Browsers hold
// a session cookie from the login form; API clients and Loxone send Basic
// Auth on each request.
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

		if user, pass, ok := r.BasicAuth(); ok {
			if wait, blocked := limiter.blocked(clientIP(r)); blocked {
				tooManyAttempts(w, wait)
				return
			}
			if credentialsMatch(auth, user, pass) {
				limiter.success(clientIP(r))
				next.ServeHTTP(w, r)
				return
			}
			limiter.failure(clientIP(r))
		}

		if r.Header.Get(uiRequestHeader) == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Loxone2HUE", charset="UTF-8"`)
		}
		errorResponse(w, http.StatusUnauthorized, "authentication required")
	})
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
