//go:build !embed

// Package frontend provides HTTP handlers for frontend assets in dev mode.
package frontend

import (
	"net/http"
)

// Handler returns a handler that proxies requests to the Vite dev server.
// This is used in development mode when the frontend is run separately via `npm run dev`.
func Handler() http.Handler {
	return DevHandler()
}
