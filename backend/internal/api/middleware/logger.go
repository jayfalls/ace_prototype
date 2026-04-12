// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Logger is a middleware that logs incoming HTTP requests using structured logging.
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

		// Create a structured logger
		logger, _ := zap.NewProduction()
		defer logger.Sync()

		logger.Info("HTTP request",
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("method", r.Method),
			zap.String("url", scheme+"://"+r.Host+r.RequestURI),
			zap.String("user_agent", r.UserAgent()),
			zap.String("request_id", reqID),
			zap.Duration("duration", time.Since(start)),
		)

		next.ServeHTTP(w, r)
	})
}
