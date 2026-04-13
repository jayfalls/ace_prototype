// Package frontend provides HTTP handlers for frontend assets in dev and embedded modes.
package frontend

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

const defaultViteURL = "http://localhost:5173"

// DevHandler returns a handler that proxies requests to the Vite dev server.
// This is used in development mode when the frontend is run separately via `npm run dev`.
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

	return proxy
}
