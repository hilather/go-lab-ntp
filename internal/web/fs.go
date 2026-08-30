package web

import (
	"embed"
	"io/fs"
)

//go:embed all:stub
var stub embed.FS

//go:embed all:dist
var dist embed.FS

// Files returns the production Vite tree when web-embed copied index.html
// into dist/, otherwise the committed stub used by go test before web-build.
func Files() fs.FS {
	if sub, err := fs.Sub(dist, "dist"); err == nil {
		if _, err := fs.Stat(sub, "index.html"); err == nil {
			return sub
		}
	}
	sub, err := fs.Sub(stub, "stub")
	if err != nil {
		return stub
	}
	return sub
}
