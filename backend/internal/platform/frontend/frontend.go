// Package frontend provides HTTP handlers for frontend assets in dev and embedded modes.
package frontend

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

const defaultViteURL = "http://localhost:5173"

// DevHandler returns a handler that proxies requests to the Vite dev server.
// This is used in development mode when the frontend is run separately via `npm run dev`.
// API paths are excluded from proxying: the Vite dev server proxies /api/* back to the backend,
// so proxying API paths creates an infinite loop when the backend doesn't match a route.
func DevHandler() http.Handler {
	target, err := url.Parse(defaultViteURL)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Invalid Vite URL: "+defaultViteURL, http.StatusInternalServerError)
		})
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Vite dev server not reachable at localhost:5173. Run 'npm run dev' in the frontend directory.", http.StatusBadGateway)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not proxy API or health paths to Vite — Vite proxies these back to the backend,
		// creating an infinite loop when the backend has no matching route.
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/health/") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}
