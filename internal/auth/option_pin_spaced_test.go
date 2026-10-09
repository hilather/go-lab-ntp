package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// A configured secret of at least 32 bytes that contains a space or a tab
// loads through FromSpec. Main rejects that presented secret before the
// digest compare. REST and MCP pass the Authorization value through
// Header.Get and TrimSpace into Authenticate. mcp-stdio passes the first
// usable line to AuthenticateBearer. Both lookups fail.
func TestOptionPinSpacedSecretRejected(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		secret := "0123456789abcdef0123456789abcdef"
		v := loadSecret(t, secret)
		if p, err := v.Authenticate(Request{Authorization: bearerHeader(t, secret)}); err != nil || p.ID != "admin" {
			t.Fatalf("plain Authenticate: %+v %v", p, err)
		}
		if p, err := v.AuthenticateBearer(secret); err != nil || p.ID != "admin" {
			t.Fatalf("plain AuthenticateBearer: %+v %v", p, err)
		}
	})

	for _, tc := range []struct {
		name string
		sep  string
	}{
		{name: "space", sep: " "},
		{name: "tab", sep: "\t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secret := "0123456789abcdef" + tc.sep + "0123456789abcdef"
			if len(secret) < MinTokenBytes {
				t.Fatalf("secret length %d", len(secret))
			}
			v := loadSecret(t, secret)
			hdr := bearerHeader(t, secret)
			if hdr != "Bearer "+secret {
				t.Fatalf("header %q", hdr)
			}
			_, err := v.Authenticate(Request{Authorization: hdr})
			assertUnauthenticated(t, err)
			_, err = v.AuthenticateBearer(secret)
			assertUnauthenticated(t, err)
		})
	}
}

func loadSecret(t *testing.T, secret string) *Verifier {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := FromSpec(model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID: "admin", Role: model.RoleAdministrator, SecretFile: path,
		}},
	})
	if err != nil {
		t.Fatalf("FromSpec: %v", err)
	}
	if v.TokenCount() != 1 {
		t.Fatalf("tokens %d", v.TokenCount())
	}
	return v
}

// bearerHeader is the Authorization value REST and MCP hand to Authenticate.
func bearerHeader(t *testing.T, secret string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://lab.example/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	return strings.TrimSpace(req.Header.Get("Authorization"))
}

func assertUnauthenticated(t *testing.T, err error) {
	t.Helper()
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" {
		t.Fatalf("auth error: %v", err)
	}
}
