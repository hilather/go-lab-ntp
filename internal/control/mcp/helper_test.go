package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/model"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const testBearerToken = "0123456789abcdef0123456789abcdef"

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
		Service:            svc,
		RatePerSec:         -1,
		AllowLegacyClients: true,
		Auth:               auth.Static(testBearerToken, "admin", model.RoleAdministrator),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, svc
}

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set(headerAuthorization, "Bearer "+b.token)
	base := b.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

func startHTTP(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func connectClient(t *testing.T, ts *httptest.Server) *sdk.ClientSession {
	t.Helper()
	return connectClientAuth(t, ts, testBearerToken)
}

func connectClientAuth(t *testing.T, ts *httptest.Server, token string) *sdk.ClientSession {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "labntp-test", Version: "dev"}, nil)
	session, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint:             ts.URL,
		DisableStandaloneSSE: true,
		HTTPClient:           &http.Client{Transport: bearerRoundTripper{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}
