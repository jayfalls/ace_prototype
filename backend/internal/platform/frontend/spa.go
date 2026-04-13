// Package frontend provides HTTP handlers for frontend assets in dev and embedded modes.
package frontend

import (
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
)

// SPAHandler returns an HTTP handler that serves a Single Page Application.
// It implements the routing rules from FSD §3.5:
//
//  1. Request path has a file extension → serve file from embedded FS with
//     appropriate Content-Type and cache headers
//  2. Request path starts with /_app/ or /@vite/ → serve from embedded FS
//     (Vite internal assets)
//  3. All other paths → serve index.html with Content-Type: text/html; charset=utf-8
//
// Asset caching headers:
//   - /_app/immutable/* → public, max-age=31536000, immutable
//   - Other files with extension → public, max-age=3600
//   - index.html → no-cache
func SPAHandler(assets http.FileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path

		// Remove leading slash for file system operations
		filePath := strings.TrimPrefix(reqPath, "/")

		// Check if request is for a file asset (has extension)
		hasExt := path.Ext(filePath) != ""

		// Handle immutable assets (Vite's _app/immutable directory)
		if strings.HasPrefix(reqPath, "/_app/immutable/") {
			serveAsset(w, r, assets, filePath, "public, max-age=31536000, immutable")
			return
		}

		// Handle Vite internal assets
		if strings.HasPrefix(reqPath, "/_app/") || strings.HasPrefix(reqPath, "/@vite/") {
			serveAsset(w, r, assets, filePath, "public, max-age=3600")
			return
		}

		// Check if path has a file extension
		if hasExt && path.Ext(filePath) != "" {
			serveAsset(w, r, assets, filePath, "public, max-age=3600")
			return
		}

		// All other paths → serve index.html (SPA routing)
		serveIndex(w, r, assets)
	})
}

// serveAsset serves a file from the embedded filesystem with caching headers.
func serveAsset(w http.ResponseWriter, r *http.Request, assets http.FileSystem, filePath, cacheControl string) {
	f, err := assets.Open(filePath)
	if err != nil {
		// If file not found, try serving index.html
		serveIndex(w, r, assets)
		return
	}
	defer f.Close()

	// Get content type from mime.TypeByExtension
	contentType := mime.TypeByExtension(path.Ext(filePath))
	if contentType == "" {
		// Fallback to application/octet-stream for unknown types
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusOK)

	io.Copy(w, f)
}

// serveIndex serves the index.html file for SPA routing.
func serveIndex(w http.ResponseWriter, r *http.Request, assets http.FileSystem) {
	f, err := assets.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found in embedded assets", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	io.Copy(w, f)
}
