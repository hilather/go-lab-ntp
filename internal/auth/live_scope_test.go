package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// TestLiveScopeTablePinsOperatorWithoutAdmin loads viewer, operator, and
// administrator through FromSpec and authenticates each the way REST and MCP
// do. The operator principal must not carry ntp.admin, and AuthorizeScopes
// on the export capability (GET /v1/state:export and ntp_state_export) must
// refuse it. DefaultScopes stays; frozen characterization tests call it.
// It must keep matching ntpScopeTable for every role.
func TestLiveScopeTablePinsOperatorWithoutAdmin(t *testing.T) {
	table := ntpScopeTable()
	want := map[string][]string{
		model.RoleViewer:        {model.ScopeNTPRead},
		model.RoleOperator:      {model.ScopeNTPRead, model.ScopeNTPWrite},
		model.RoleAdministrator: {model.ScopeNTPRead, model.ScopeNTPWrite, model.ScopeNTPAdmin, model.ScopeNTPAuditRead},
	}
	if len(table.Roles) != len(want) {
		t.Fatalf("live roles = %v, want %d", table.Roles, len(want))
	}
	for role, scopes := range want {
		got, ok := table.Roles[role]
		if !ok || !slices.Equal(got, scopes) {
			t.Fatalf("live %s = %v (ok=%v), want %v", role, got, ok, scopes)
		}
		if !slices.Equal(DefaultScopes(role), got) {
			t.Fatalf("DefaultScopes(%s) = %v, live table = %v", role, DefaultScopes(role), got)
		}
	}
	for _, role := range []string{"reader", " administrator", "operator ", "Administrator"} {
		if got := DefaultScopes(role); got != nil {
			t.Fatalf("DefaultScopes(%q) = %v, want nil", role, got)
		}
		if scopes, ok := table.Roles[role]; ok || scopes != nil {
			t.Fatalf("live table %q = %v ok=%v, want absent", role, scopes, ok)
		}
	}

	dir := t.TempDir()
	const (
		viewerSecret = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		opSecret     = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		adminSecret  = "cccccccccccccccccccccccccccccccc"
	)
	v, err := FromSpec(model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{
			{ID: "viewer", Role: model.RoleViewer, SecretFile: writeLiveSecret(t, dir, "viewer", viewerSecret)},
			{ID: "operator", Role: model.RoleOperator, SecretFile: writeLiveSecret(t, dir, "operator", opSecret)},
			{ID: "admin", Role: model.RoleAdministrator, SecretFile: writeLiveSecret(t, dir, "admin", adminSecret)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	restExport, ok := capabilities.LookupREST(http.MethodGet, "/v1/state:export")
	if !ok {
		t.Fatal("REST export capability")
	}
	mcpExport := capabilities.LookupTool("ntp_state_export")
	if len(mcpExport) != 1 || !slices.Equal(mcpExport[0].RequiredScopes, restExport.RequiredScopes) {
		t.Fatalf("MCP export scopes %v, REST %v", mcpExport, restExport.RequiredScopes)
	}
	if !slices.Equal(restExport.RequiredScopes, []string{model.ScopeNTPAdmin}) {
		t.Fatalf("export required scopes %v", restExport.RequiredScopes)
	}

	viewer := authenticateLive(t, v, viewerSecret)
	operator := authenticateLive(t, v, opSecret)
	admin := authenticateLive(t, v, adminSecret)

	if viewer.Role != model.RoleViewer || !slices.Equal(viewer.Scopes, want[model.RoleViewer]) || viewer.HasScope(model.ScopeNTPAdmin) {
		t.Fatalf("viewer %+v", viewer)
	}
	if operator.Role != model.RoleOperator || !slices.Equal(operator.Scopes, want[model.RoleOperator]) || operator.HasScope(model.ScopeNTPAdmin) {
		t.Fatalf("operator %+v", operator)
	}
	if admin.Role != model.RoleAdministrator || !slices.Equal(admin.Scopes, want[model.RoleAdministrator]) || !admin.HasScope(model.ScopeNTPAdmin) {
		t.Fatalf("administrator %+v", admin)
	}

	// REST authorize and MCP authorizeTool both call AuthorizeScopes with
	// the capability's RequiredScopes. Export is ntp.admin.
	for _, p := range []Principal{viewer, operator} {
		err := AuthorizeScopes(p.Scopes, restExport.RequiredScopes)
		de, ok := domainerr.As(err)
		if !ok || de.Code != domainerr.CodeForbidden || de.Message != "missing scope "+model.ScopeNTPAdmin {
			t.Fatalf("%s authorize export: %v", p.Role, err)
		}
		if err := Authorize(p, mcpExport[0].RequiredScopes); err == nil {
			t.Fatalf("%s Authorize accepted ntp.admin", p.Role)
		}
	}
	if err := AuthorizeScopes(admin.Scopes, restExport.RequiredScopes); err != nil {
		t.Fatal(err)
	}
}

func authenticateLive(t *testing.T, v *Verifier, secret string) Principal {
	t.Helper()
	p, err := v.Authenticate(Request{Authorization: "Bearer " + secret})
	if err != nil {
		t.Fatal(err)
	}
	if p.Class != ClassToken {
		t.Fatalf("class %q", p.Class)
	}
	return p
}

func writeLiveSecret(t *testing.T, dir, name, secret string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
