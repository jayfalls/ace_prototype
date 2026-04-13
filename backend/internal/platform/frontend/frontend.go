// Package frontend provides HTTP handlers for frontend assets in dev and embedded modes.
package frontend

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// DevProxy returns a handler that proxies requests to the Vite dev server.
// This is used in development mode when the frontend is run separately.
func DevProxy(viteURL string) http.Handler {
	target, err := url.Parse(viteURL)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Invalid Vite URL: "+viteURL, http.StatusInternalServerError)
		})
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Frontend not available in dev mode - run npm run dev in frontend/", http.StatusServiceUnavailable)
	}

	return proxy
}

// DevModeHandler returns a placeholder handler for dev mode when Vite is not running.
func DevModeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("frontend not available in dev mode - run npm run dev in frontend/"))
	})
}
