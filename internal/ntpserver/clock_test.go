package ntpserver

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/ntpwire"
)

func TestNoClockSetSyscalls(t *testing.T) {
	root := moduleRoot(t)
	forbiddenSel := map[string]bool{
		"Settimeofday": true,
		"ClockSettime": true,
		"Adjtimex":     true,
		"ClockAdjtime": true,
		"Adjtime":      true,
	}
	forbiddenExec := map[string]bool{
		"date": true, "hwclock": true, "chronyc": true, "ntpd": true, "timedatectl": true,
	}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "vendor", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if forbiddenSel[x.Sel.Name] {
					t.Errorf("%s: forbidden selector %s", rel, x.Sel.Name)
				}
			case *ast.CallExpr:
				fun, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if fun.Sel.Name != "Command" && fun.Sel.Name != "CommandContext" {
					return true
				}
				// exec.Command / CommandContext string-literal args
				id, ok := fun.X.(*ast.Ident)
				if !ok || id.Name != "exec" {
					return true
				}
				start := 0
				if fun.Sel.Name == "CommandContext" {
					start = 1
				}
				if start >= len(x.Args) {
					return true
				}
				lit, ok := x.Args[start].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				val := strings.Trim(lit.Value, `"`)
				base := val
				if i := strings.LastIndex(val, "/"); i >= 0 {
					base = val[i+1:]
				}
				if forbiddenExec[base] {
					t.Errorf("%s: forbidden exec %s", rel, val)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHostClockUnchanged(t *testing.T) {
	rt0, mono0, err := readClocks()
	if err != nil {
		t.Skip(err)
	}
	srv, addr := startServer(t, testdata(t, "config/valid/defaults.yaml"), "")
	defer shutdown(t, srv)
	c, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	req := ntpwire.Encode(ntpwire.Packet{VN: 4, Mode: ntpwire.ModeClient, XmtTime: ntpwire.FromTime(time.Now())})
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	for i := 0; i < 1000; i++ {
		if _, err := c.Write(req); err != nil {
			t.Fatal(err)
		}
	}
	buf := make([]byte, 64)
	_ = c.SetDeadline(time.Now().Add(200 * time.Millisecond))
	for {
		if _, err := c.Read(buf); err != nil {
			break
		}
	}
	rt1, mono1, err := readClocks()
	if err != nil {
		t.Fatal(err)
	}
	dRT := rt1 - rt0
	dMono := mono1 - mono0
	delta := dRT - dMono
	if delta < 0 {
		delta = -delta
	}
	if delta > 50*time.Millisecond {
		t.Fatalf("|ΔREALTIME-ΔMONOTONIC|=%s (rt %s mono %s)", delta, dRT, dMono)
	}
}
