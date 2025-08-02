package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFS embed.FS

// StaticFS provides access to static files (CSS, JS, etc.)
// In embedded mode, files are served from the embedded filesystem
// In development mode, this could be swapped for os.DirFS for live reloading
var StaticFS fs.FS

func init() {
	// Use fs.Sub to mount the static/dist directory at the root level
	// This ensures consistent behavior between embedded and filesystem modes
	var err error
	StaticFS, err = fs.Sub(staticFS, "static/dist")
	if err != nil {
		panic("failed to create static filesystem: " + err.Error())
	}
}

// GetStaticFS returns the static filesystem
// This allows for easy swapping between embedded and filesystem modes
func GetStaticFS() fs.FS {
	return StaticFS
}

// SetStaticFS allows overriding the static filesystem (useful for development)
func SetStaticFS(fsys fs.FS) {
	StaticFS = fsys
}