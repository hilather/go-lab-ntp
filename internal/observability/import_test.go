package observability

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestNoControlWebImports(t *testing.T) {
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
			if strings.Contains(path, "internal/control") || strings.Contains(path, "internal/web") {
				t.Errorf("%s imports %s", name, path)
			}
			if strings.Contains(path, "github.com/prometheus") {
				t.Errorf("%s imports prometheus client %s", name, path)
			}
		}
	}
}
