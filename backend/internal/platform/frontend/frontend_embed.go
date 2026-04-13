//go:build embed

// Package frontend provides HTTP handlers for frontend assets in embedded production mode.
package frontend

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:assets
var embeddedAssets embed.FS

// Handler returns an HTTP handler that serves the embedded SPA.
// This is used in production mode when built with the embed tag.
func Handler() http.Handler {
	// Strip the "build" prefix since the embed directive includes it
	assets, err := fs.Sub(embeddedAssets, "build")
	if err != nil {
		// Fallback: serve index.html directly from root using http.FS adapter
		return SPAHandler(http.FS(embeddedAssets))
	}
	return SPAHandler(http.FS(assets))
}
