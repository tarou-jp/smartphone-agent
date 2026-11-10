// Package frontend holds the desktop client's web front end, built by Vite
// into dist (make web). Pinky embeds the build; a checkout that has not
// built it gets a page that says how.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built front end, or nil when it is not built.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}
