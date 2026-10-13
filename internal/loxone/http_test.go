package loxone

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
)

// TestHTTPSenderTLSVerification covers the http_verify_tls switch: by default
// a Miniserver's self-signed certificate is accepted (existing installations
// keep working), with the switch on the request is refused and the target is
// paused instead of credentials being sent to an unverified peer.
func TestHTTPSenderTLSVerification(t *testing.T) {
	var hits int64
	var lastAuth atomic.Value
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		lastAuth.Store(r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	s := NewHTTPSender()
	defer s.Close()
	s.Configure([]config.MiniserverConfig{
		{ID: "lax", Name: "lax", HTTPEnabled: true, HTTPURL: srv.URL, HTTPUser: "admin", HTTPPassword: "geheim"},
		{ID: "strict", Name: "strict", HTTPEnabled: true, HTTPURL: srv.URL, HTTPUser: "admin", HTTPPassword: "geheim", HTTPVerifyTLS: true},
	})

	s.Send("lax", "wz", "on", 1)
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("default target: %d requests, want 1 (self-signed certificate must be accepted)", got)
	}
	if auth, _ := lastAuth.Load().(string); !strings.HasPrefix(auth, "Basic ") {
		t.Errorf("Authorization = %q, want Basic Auth", auth)
	}

	s.Send("strict", "wz", "on", 1)
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("verifying target reached the server despite an untrusted certificate (%d requests)", got)
	}
	s.mu.Lock()
	paused := time.Now().Before(s.targets["strict"].downUntil)
	s.mu.Unlock()
	if !paused {
		t.Error("verifying target was not paused after the TLS failure")
	}
}
