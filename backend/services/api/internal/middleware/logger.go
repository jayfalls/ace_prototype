// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Logger is a middleware that logs incoming HTTP requests.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}

		// Get request ID if present
		reqID := chi.URLParam(r, "request_id")
		if reqID == "" {
			reqID = "-"
		}

		log.Printf(
			"[%s] %s %s %s %s %s",
			r.RemoteAddr,
			r.Method,
			scheme+"://"+r.Host+r.RequestURI,
			r.UserAgent(),
			reqID,
			time.Since(start),
		)

		next.ServeHTTP(w, r)
	})
}
