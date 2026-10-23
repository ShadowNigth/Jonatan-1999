package ui

import (
	"embed"
	"io/fs"
)

//go:embed static
var embeddedFiles embed.FS

// StaticFS returns the embedded static assets filesystem rooted at static/.
func StaticFS() (fs.FS, error) {
	return fs.Sub(embeddedFiles, "static")
}
