// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"net/http"
	"strings"
)

// CORS creates a CORS middleware with configurable allowed origins.
//
// The middleware handles:
// - Preflight requests (OPTIONS)
// - Setting appropriate CORS headers on responses
//
// Configuration:
// - allowedOrigins: List of allowed origins. Use ["*"] to allow all origins.
// - allowedMethods: HTTP methods allowed for cross-origin requests
// - allowedHeaders: Headers that can be sent in cross-origin requests
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			originAllowed := false
			for _, allowed := range allowedOrigins {
				if allowed == "*" {
					originAllowed = true
					break
				}
				if strings.EqualFold(allowed, origin) {
					originAllowed = true
					break
				}
			}

			// Set CORS headers for allowed origins
			if originAllowed {
				if origin != "" && origin != "*" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else if allowedOrigins[0] == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Request-ID")
				w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
