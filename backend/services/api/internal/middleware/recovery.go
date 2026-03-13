// Package middleware provides HTTP middleware for the API service.
package middleware

import (
	"log"
	"net/http"
)

// Recovery is a middleware that recovers from panics and returns a 500 error.
// This prevents the entire server from crashing due to a panic in a handler.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
