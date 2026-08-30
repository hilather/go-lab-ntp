package rest

import (
	"net/http"
	"path"
	"strings"
)

// tryUI serves the embedded SPA after native routing misses. rest must not
// import internal/web; cmd/labntp wires Config.UI.
func (s *Server) tryUI(w http.ResponseWriter, r *http.Request, instance string) bool {
	if s.cfg.UI == nil || reservedManagementPath(r.URL.Path) {
		return false
	}
	if s.cfg.UIEnabled != nil && !s.cfg.UIEnabled() {
		return false
	}
	if err := s.rate.allow(r.RemoteAddr); err != nil {
		s.writeProblem(w, r, instance, err)
		return true
	}
	s.cfg.UI.ServeHTTP(w, r)
	return true
}

// reservedManagementPath keeps /v1 and MCP off the SPA fallback so a
// missing API route stays problem+json instead of index.html.
func reservedManagementPath(p string) bool {
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
