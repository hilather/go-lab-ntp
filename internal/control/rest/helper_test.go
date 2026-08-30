package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/model"
)

const testToken = "0123456789abcdef0123456789abcdef"

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func bootTestApp(t *testing.T) *app.App {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "labntp.yaml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	return svc
}

func newTestServer(t *testing.T) (*Server, *app.App) {
	t.Helper()
	svc := bootTestApp(t)
	s, err := New(Config{
		Service:    svc,
		RatePerSec: -1,
		Auth:       auth.Static(testToken, "admin", model.RoleAdministrator),
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, svc
}

func doJSON(t *testing.T, s *Server, method, path, body string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w.Result()
}

func decodeMap(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	defer func() { _ = r.Body.Close() }()
	var m map[string]any
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}
