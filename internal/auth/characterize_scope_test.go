package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestCharacterizeScopeMatrix(t *testing.T) {
	if got := DefaultScopes(model.RoleViewer); !sameScopes(got, []string{model.ScopeNTPRead}) {
		t.Fatalf("viewer %v", got)
	}
	if got := DefaultScopes(model.RoleOperator); !sameScopes(got, []string{model.ScopeNTPRead, model.ScopeNTPWrite}) {
		t.Fatalf("operator %v", got)
	}
	admin := DefaultScopes(model.RoleAdministrator)
	wantAdmin := []string{model.ScopeNTPRead, model.ScopeNTPWrite, model.ScopeNTPAdmin, model.ScopeNTPAuditRead}
	if !sameScopes(admin, wantAdmin) {
		t.Fatalf("administrator %v", admin)
	}
	for i := range wantAdmin {
		if admin[i] != wantAdmin[i] {
			t.Fatalf("administrator order %v", admin)
		}
	}
	if DefaultScopes("reader") != nil || DefaultScopes(" administrator") != nil {
		t.Fatal("unknown role scopes must be nil")
	}

	staticAdmin := Static("0123456789abcdef0123456789abcdef", "admin", model.RoleAdministrator)
	p, err := staticAdmin.AuthenticateBearer("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if !sameScopes(p.Scopes, wantAdmin) || p.Role != model.RoleAdministrator || p.Class != ClassToken {
		t.Fatalf("static %+v", p)
	}

	dir := t.TempDir()
	secret := writeSecret(t, dir, "tok", "0123456789abcdef0123456789abcdef\n")
	explicit, err := FromSpec(model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID:         "viewer",
			Role:       model.RoleViewer,
			SecretFile: secret,
			Scopes:     []string{model.ScopeNTPAdmin},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err = explicit.AuthenticateBearer("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != model.RoleViewer || len(p.Scopes) != 1 || p.Scopes[0] != model.ScopeNTPAdmin {
		t.Fatalf("explicit scopes must replace the role set: %+v", p)
	}

	emptyRole, err := FromSpec(model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID:         "empty-role",
			SecretFile: secret,
			Scopes:     []string{model.ScopeNTPRead},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err = emptyRole.AuthenticateBearer("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != model.RoleAdministrator || len(p.Scopes) != 1 || p.Scopes[0] != model.ScopeNTPRead {
		t.Fatalf("empty role with explicit scopes: %+v", p)
	}

	_, err = FromSpec(model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID:         "bad-role",
			Role:       "nope",
			SecretFile: secret,
			Scopes:     []string{model.ScopeNTPRead},
		}},
	})
	assertLoad(t, err, "unknown role", "spec.auth.tokens[0].role", "invalid_value", "role must be viewer, operator, or administrator")

	if !HasScope([]string{model.ScopeNTPAdmin}, model.ScopeNTPWrite) {
		t.Fatal("ntp.admin satisfies every scope")
	}
	if !HasScope(nil, "") {
		t.Fatal("empty want is true")
	}
	if HasScope([]string{model.ScopeNTPRead}, model.ScopeNTPWrite) {
		t.Fatal("viewer scope must not satisfy ntp.write")
	}
	err = AuthorizeScopes([]string{model.ScopeNTPRead}, []string{model.ScopeNTPAdmin})
	assertForbidden(t, err, "missing scope "+model.ScopeNTPAdmin)
	if err := AuthorizeScopes([]string{model.ScopeNTPAdmin}, []string{model.ScopeNTPRead, model.ScopeNTPAuditRead}); err != nil {
		t.Fatal(err)
	}
}

func writeSecret(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertLoad(t *testing.T, err error, top, path, code, message string) {
	t.Helper()
	de, ok := domainerr.As(err)
	if !ok {
		t.Fatalf("err=%v", err)
	}
	if de.Code != domainerr.CodeValidationFailed || de.Message != top {
		t.Fatalf("top=%q code=%s want %q", de.Message, de.Code, top)
	}
	if len(de.FieldViolations) != 1 {
		t.Fatalf("violations %+v", de.FieldViolations)
	}
	v := de.FieldViolations[0]
	if v.Path != path || v.Code != code || v.Message != message {
		t.Fatalf("violation %+v", v)
	}
}
