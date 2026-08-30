package rest

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestNoMCPImport(t *testing.T) {
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
			if strings.Contains(path, "internal/control/mcp") {
				t.Errorf("%s imports MCP %s", name, path)
			}
			if path == "github.com/hilather/go-lab-ntp/internal/web" || strings.HasPrefix(path, "github.com/hilather/go-lab-ntp/internal/web/") {
				t.Errorf("%s production file imports internal/web %s", name, path)
			}
		}
	}
}
