package server

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

// mutatingAuth requires `Authorization: Bearer <token>` on every non-GET /api/ route.
// Same-origin browser requests (Origin/Referer host == request Host) pass so the
// served WebUI keeps working without a token round-trip; cross-site browser posts
// carry a foreign Origin and are rejected. ACE is single-owner and loopback-bound.
func (s *Server) mutatingAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/") {
			if !bearerOK(r, s.authToken) && !sameOrigin(r) {
				writeError(w, http.StatusUnauthorized, "bearer token required for mutating routes")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func bearerOK(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		o = r.Header.Get("Referer")
	}
	if o == "" {
		return false
	}
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}

// corsMiddleware reflects the Origin only when it matches the request host
// (same-origin WebUI). No wildcard: cross-origin browser access gets nothing.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sameOrigin(r) {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
