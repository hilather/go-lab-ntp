package web

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

const (
	cacheHTML   = "no-cache"
	cacheHashed = "public, max-age=31536000, immutable"
)

// NewHandler serves hashed assets with an immutable cache and falls back
// to index.html for client routes. Management API paths are never claimed.
func NewHandler(fsys fs.FS) http.Handler {
	if fsys == nil {
		fsys = Files()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if Reserved(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		rel, ok := safeRel(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		serve(w, r, fsys, rel)
	})
}

// Reserved reports management paths the SPA must not capture.
func Reserved(p string) bool {
	p = path.Clean("/" + p)
	switch {
	case p == "/v1" || strings.HasPrefix(p, "/v1/"):
		return true
	case p == "/mcp" || strings.HasPrefix(p, "/mcp/"):
		return true
	case p == "/healthz" || strings.HasPrefix(p, "/healthz/"):
		return true
	case p == "/config" || strings.HasPrefix(p, "/config/"):
		return true
	case p == "/.well-known" || strings.HasPrefix(p, "/.well-known/"):
		return true
	default:
		return false
	}
}

func safeRel(urlPath string) (string, bool) {
	if strings.Contains(urlPath, "..") || strings.Contains(urlPath, "\\") {
		return "", false
	}
	if urlPath == "" || urlPath == "/" {
		return "index.html", true
	}
	if !strings.HasPrefix(urlPath, "/") {
		return "", false
	}
	clean := path.Clean(urlPath)
	if clean == "/" {
		return "index.html", true
	}
	if strings.Contains(clean, "..") {
		return "", false
	}
	return strings.TrimPrefix(clean, "/"), true
}

func hashedAsset(rel string) bool {
	base := path.Base(rel)
	dash := strings.LastIndex(base, "-")
	if dash < 0 {
		return false
	}
	rest := base[dash+1:]
	dot := strings.LastIndex(rest, ".")
	return dot >= 8
}

func serve(w http.ResponseWriter, r *http.Request, fsys fs.FS, rel string) {
	info, err := fs.Stat(fsys, rel)
	if err != nil || info.IsDir() {
		if path.Ext(rel) != "" && path.Ext(rel) != ".html" {
			http.NotFound(w, r)
			return
		}
		serveFile(w, r, fsys, "index.html", false)
		return
	}
	serveFile(w, r, fsys, rel, hashedAsset(rel))
}

func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string, hashed bool) {
	f, err := fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if hashed {
		w.Header().Set("Cache-Control", cacheHashed)
	} else if name == "index.html" || strings.HasSuffix(name, ".html") {
		w.Header().Set("Cache-Control", cacheHTML)
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "asset is not seekable", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, stat.ModTime(), rs)
}
