// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"net/http"

	"go.uber.org/zap"
)

// Recovery is a middleware that recovers from panics and returns a 500 error.
// This prevents the entire server from crashing due to a panic in a handler.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger, _ := zap.NewProduction()
				defer logger.Sync()

				logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
