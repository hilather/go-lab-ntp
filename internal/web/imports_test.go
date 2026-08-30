package web

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestNoForbiddenImports(t *testing.T) {
	t.Parallel()
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	forbidden := []string{
		"github.com/hilather/go-lab-ntp/internal/app",
		"github.com/hilather/go-lab-ntp/internal/control",
		"github.com/hilather/go-lab-ntp/internal/ntpserver",
		"net/smtp",
	}
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
			for _, bad := range forbidden {
				if path == bad || strings.HasPrefix(path, bad+"/") {
					t.Errorf("%s imports forbidden package %s", name, path)
				}
			}
		}
	}
}
