package mcp

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestNoRESTImport(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(path, "internal/control/rest") {
				t.Errorf("%s imports REST %s", name, path)
			}
			if isControlkit(path) && !controlkitFacade(name) {
				t.Errorf("%s imports %s", name, path)
			}
		}
	}
}

// isControlkit reports an import of go-lab-controlkit or one of its packages.
func isControlkit(path string) bool {
	const mod = "github.com/hilather/go-lab-controlkit"
	return path == mod || strings.HasPrefix(path, mod+"/")
}

// controlkitFacade is the ADR 0015 allowlist for this package.
// auth.go is the ratelimit facade. This commit adds no require.
func controlkitFacade(name string) bool {
	return name == "auth.go"
}
