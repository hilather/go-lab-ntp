package main

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestNoControlkitImport fails if any Go file in cmd/labntp, test or not,
// imports go-lab-controlkit. This package has no facade allow entry.
func TestNoControlkitImport(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if isControlkit(path) {
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
