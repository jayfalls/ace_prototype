package ace

import (
	"embed"
	"io/fs"
)

//go:embed all:web/build
var embeddedWeb embed.FS

func WebFS() (fs.FS, error) {
	return fs.Sub(embeddedWeb, "web/build")
}
