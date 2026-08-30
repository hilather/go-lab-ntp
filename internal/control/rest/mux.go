package rest

import (
	"strings"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
)

type compiledRoute struct {
	method  string
	path    string
	segs    []pathSeg
	cap     capabilities.Capability
	binding capabilities.RESTBinding
}

type pathSeg struct {
	wild   string
	lit    string
	suffix string
}

func compileRoutes(caps []capabilities.Capability) []compiledRoute {
	var out []compiledRoute
	for _, c := range caps {
		for _, b := range c.REST {
			out = append(out, compiledRoute{
				method:  strings.ToUpper(b.Method),
				path:    b.Path,
				segs:    compilePath(b.Path),
				cap:     c,
				binding: b,
			})
		}
	}
	return out
}

func compilePath(path string) []pathSeg {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil
	}
	raw := strings.Split(path, "/")
	out := make([]pathSeg, len(raw))
	for i, p := range raw {
		open := strings.IndexByte(p, '{')
		close := strings.IndexByte(p, '}')
		if open >= 0 && close > open {
			out[i] = pathSeg{
				lit:    p[:open],
				wild:   p[open+1 : close],
				suffix: p[close+1:],
			}
			continue
		}
		out[i] = pathSeg{lit: p}
	}
	return out
}

func matchRoute(routes []compiledRoute, method, path string) (compiledRoute, map[string]string, bool, bool) {
	method = strings.ToUpper(method)
	pathOK := false
	var pathHit compiledRoute
	for _, rt := range routes {
		params, ok := matchSegs(rt.segs, path)
		if !ok {
			continue
		}
		pathOK = true
		pathHit = rt
		if rt.method == method {
			return rt, params, true, true
		}
	}
	return pathHit, nil, pathOK, false
}

func matchSegs(segs []pathSeg, path string) (map[string]string, bool) {
	path = strings.TrimPrefix(path, "/")
	var parts []string
	if path != "" {
		parts = strings.Split(path, "/")
	}
	if len(parts) != len(segs) {
		return nil, false
	}
	params := make(map[string]string)
	for i, seg := range segs {
		p := parts[i]
		if seg.wild == "" {
			if p != seg.lit {
				return nil, false
			}
			continue
		}
		if !strings.HasPrefix(p, seg.lit) || !strings.HasSuffix(p, seg.suffix) {
			return nil, false
		}
		val := p[len(seg.lit) : len(p)-len(seg.suffix)]
		if val == "" {
			return nil, false
		}
		params[seg.wild] = val
	}
	return params, true
}

func allowedMethods(routes []compiledRoute, path string) string {
	var methods []string
	seen := map[string]bool{}
	for _, rt := range routes {
		if _, ok := matchSegs(rt.segs, path); !ok {
			continue
		}
		if seen[rt.method] {
			continue
		}
		seen[rt.method] = true
		methods = append(methods, rt.method)
	}
	return strings.Join(methods, ", ")
}
