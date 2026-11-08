package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
)

// ---------------------------------------------------------------------------
// Home Assistant Ingress
//
// The add-on serves the sidebar panel on a port of its own. The Supervisor
// proxies to it only after Home Assistant has signed the user in, always from
// the fixed address 172.30.32.2, and names the signed-in user in the
// X-Remote-User-* headers. Requests on that port from any other address are
// refused, so the port cannot be used to get around the login.
//
// The main port keeps its login: the Miniserver and direct browser access
// still need the gateway credentials.

// DefaultIngressProxyIP is the Home Assistant Supervisor's address on the
// internal hassio network; every Ingress request comes from it.
const DefaultIngressProxyIP = "172.30.32.2"

type ingressContextKey struct{}

type ingressInfo struct {
	user string
}

// ingressMiddleware wraps the handler of the Ingress listener. It refuses
// every client but the Supervisor and marks the request as coming through
// Ingress.
func ingressMiddleware(proxyIP string, next http.Handler) http.Handler {
	var warnOnce sync.Once
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip != proxyIP {
			warnOnce.Do(func() {
				log.Warn().Str("ip", ip).Str("expected", proxyIP).
					Msg("Refused a request on the Ingress port that did not come from the Home Assistant Supervisor")
			})
			errorResponse(w, http.StatusForbidden, "this port only serves Home Assistant Ingress")
			return
		}

		user := r.Header.Get("X-Remote-User-Display-Name")
		if user == "" {
			user = r.Header.Get("X-Remote-User-Name")
		}
		ctx := context.WithValue(r.Context(), ingressContextKey{}, ingressInfo{user: user})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isIngressRequest reports whether the request came through Home Assistant
// Ingress, as established by ingressMiddleware. Headers alone never count.
func isIngressRequest(r *http.Request) bool {
	_, ok := r.Context().Value(ingressContextKey{}).(ingressInfo)
	return ok
}

// ingressUser returns the Home Assistant user named by the Supervisor, if any.
func ingressUser(r *http.Request) string {
	info, _ := r.Context().Value(ingressContextKey{}).(ingressInfo)
	return info.user
}
