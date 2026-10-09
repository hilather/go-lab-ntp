package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestCharacterizeMCPAuthText(t *testing.T) {
	s, _ := newTestServer(t)
	basic := callMCP(t, s, "Basic YWRtaW46c2VjcmV0")
	assertRPC(t, basic, http.StatusUnauthorized, -32001, "MCP accepts bearer tokens only")
	assertRPC(t, callMCP(t, s, ""), http.StatusUnauthorized, -32001, "authentication required")
	assertRPC(t, callMCP(t, s, "Bearer nope"), http.StatusUnauthorized, -32001, "authentication required")
	assertRPC(t, callMCP(t, s, "Token abc"), http.StatusUnauthorized, -32001, "authentication required")

	nilSrv := newMCPAuth(t, bootTestApp(t), nil)
	assertRPC(t, callMCP(t, nilSrv, "Bearer "+testBearerToken), http.StatusUnauthorized, -32001, "authentication required")

	origin := newMCPOrigin(t, []string{"https://lab.example"})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ntp_version_get","arguments":{}}}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", ProtocolVersion)
	req.Header.Set("Origin", "https://evil.example")
	req.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	origin.Handler().ServeHTTP(w, req)
	assertRPC(t, w.Result(), http.StatusForbidden, -32003, "origin is not allowed")

	emptyHost := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ntp_version_get","arguments":{}}}`))
	emptyHost.Header.Set("Authorization", "Bearer "+testBearerToken)
	emptyHost.Header.Set("Content-Type", "application/json")
	emptyHost.Header.Set("Accept", "application/json, text/event-stream")
	emptyHost.Header.Set("Mcp-Protocol-Version", ProtocolVersion)
	emptyHost.Header.Set("Origin", "http://")
	emptyHost.RemoteAddr = "192.0.2.1:1234"
	ew := httptest.NewRecorder()
	origin.Handler().ServeHTTP(ew, emptyHost)
	assertRPC(t, ew.Result(), http.StatusForbidden, -32003, "origin is not allowed")
}

func TestCharacterizeUnmappedToolAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	viewer := app.Actor{ID: "viewer", Class: auth.ClassToken, Role: model.RoleViewer, Scopes: auth.DefaultScopes(model.RoleViewer), Transport: "mcp"}
	if err := s.authorizeTool(viewer, "ntp_not_a_tool"); err != nil {
		t.Fatalf("unmapped tool must be allowed: %v", err)
	}
	if err := s.authorizeTool(viewer, "ntp_state_get"); err != nil {
		t.Fatalf("read tool: %v", err)
	}
	err := s.authorizeTool(viewer, "ntp_state_reset")
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeForbidden || de.Message != "missing scope "+model.ScopeNTPAdmin {
		t.Fatalf("admin tool: %v", err)
	}
}

func TestCharacterizeFirstCapOnly(t *testing.T) {
	s, _ := newTestServer(t)
	viewer := app.Actor{ID: "viewer", Class: auth.ClassToken, Role: model.RoleViewer, Scopes: []string{model.ScopeNTPRead}, Transport: "mcp"}
	for _, name := range capabilities.Tools() {
		caps := capabilities.LookupTool(name)
		if len(caps) != 1 {
			t.Fatalf("%s caps %d", name, len(caps))
		}
		err := s.authorizeTool(viewer, name)
		want := ""
		if len(caps[0].RequiredScopes) > 0 {
			want = caps[0].RequiredScopes[0]
		}
		if auth.HasScope(viewer.Scopes, want) {
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			continue
		}
		de, ok := domainerr.As(err)
		if !ok || de.Message != "missing scope "+want {
			t.Fatalf("%s err=%v want missing scope %s", name, err, want)
		}
	}
}

func newMCPAuth(t *testing.T, svc *app.App, verifier *auth.Verifier) *Server {
	t.Helper()
	s, err := New(Config{Service: svc, RatePerSec: -1, AllowLegacyClients: true, Auth: verifier})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func newMCPOrigin(t *testing.T, origins []string) *Server {
	t.Helper()
	svc := bootTestApp(t)
	s, err := New(Config{
		Service:        svc,
		RatePerSec:     -1,
		Auth:           auth.Static(testBearerToken, "admin", model.RoleAdministrator),
		AllowedOrigins: origins,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func callMCP(t *testing.T, s *Server, authorization string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ntp_version_get","arguments":{}}}`))
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", ProtocolVersion)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w.Result()
}

func assertRPC(t *testing.T, res *http.Response, status, code int, message string) {
	t.Helper()
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != status {
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var doc struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Error.Code != code || doc.Error.Message != message {
		t.Fatalf("rpc %+v body %s", doc.Error, b)
	}
	if status == http.StatusUnauthorized {
		if got := res.Header.Get("WWW-Authenticate"); got != `Bearer realm="labntp"` {
			t.Fatalf("www-authenticate %q body %s", got, b)
		}
	}
}
