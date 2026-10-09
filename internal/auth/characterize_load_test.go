package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestCharacterizeLoadText(t *testing.T) {
	dir := t.TempDir()
	good := writeSecret(t, dir, "good", "0123456789abcdef0123456789abcdef\n")
	other := writeSecret(t, dir, "other", "abcdef0123456789abcdef0123456789\n")
	same := writeSecret(t, dir, "same", "0123456789abcdef0123456789abcdef\n")
	short := writeSecret(t, dir, "short", "too-short\n")
	comment := writeSecret(t, dir, "comment", "# only\n\n")

	_, err := FromSpec(model.AuthSpec{Mode: "pat", Tokens: []model.TokenSpec{{ID: "a", Role: model.RoleAdministrator, SecretFile: good}}})
	assertLoad(t, err, "unknown auth mode", "spec.auth.mode", "invalid_value", "unknown auth mode")

	_, err = FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{{ID: "  ", Role: model.RoleAdministrator, SecretFile: good}}})
	assertLoad(t, err, "token id is required", "spec.auth.tokens[0].id", "empty_id", "token id is required")

	_, err = FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{
		{ID: "admin", Role: model.RoleAdministrator, SecretFile: good},
		{ID: "admin", Role: model.RoleViewer, SecretFile: other},
	}})
	assertLoad(t, err, "duplicate token id", "spec.auth.tokens[1].id", "duplicate_id", "duplicate token id")

	missing := filepath.Join(dir, "missing")
	_, err = FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{{ID: "admin", Role: model.RoleAdministrator, SecretFile: missing}}})
	assertLoad(t, err, "token secret is unavailable", "spec.auth.tokens[0].secretFile", "unresolved_reference", "token secret file does not resolve")
	if err != nil && strings.Contains(err.Error(), "no such file") {
		t.Fatalf("os error leaked into the message: %v", err)
	}

	_, err = FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{{ID: "admin", Role: model.RoleAdministrator, SecretFile: comment}}})
	assertLoad(t, err, "token secret is unavailable", "spec.auth.tokens[0].secretFile", "unresolved_reference", "token secret file does not resolve")

	unreadable := writeSecret(t, dir, "unreadable", "0123456789abcdef0123456789abcdef\n")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })
	_, err = FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{{ID: "admin", Role: model.RoleAdministrator, SecretFile: unreadable}}})
	if os.Getuid() == 0 {
		t.Log("uid 0 can read mode 000; skipping the unreadable-file sentence")
	} else {
		assertLoad(t, err, "token secret is unavailable", "spec.auth.tokens[0].secretFile", "unresolved_reference", "token secret file does not resolve")
	}

	_, err = FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{{ID: "admin", Role: model.RoleAdministrator, SecretFile: short}}})
	assertLoad(t, err, "token entropy is below 256 bits", "spec.auth.tokens[0].secretFile", "invalid_value", "token secret must be at least 32 bytes")

	_, err = FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{
		{ID: "admin", Role: model.RoleAdministrator, SecretFile: good},
		{ID: "clone", Role: model.RoleViewer, SecretFile: same},
	}})
	assertLoad(t, err, "duplicate token value", "spec.auth.tokens[1].secretFile", "duplicate_id", "token value matches admin")

	_, err = FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{{ID: "admin", Role: "reader", SecretFile: good}}})
	assertLoad(t, err, "unknown role", "spec.auth.tokens[0].role", "invalid_value", "role must be viewer, operator, or administrator")

	emptyMode, err := FromSpec(model.AuthSpec{Tokens: []model.TokenSpec{{ID: "admin", Role: model.RoleAdministrator, SecretFile: good}}})
	if err != nil {
		t.Fatal(err)
	}
	if emptyMode.Mode() != model.MgmtAuthBearer {
		t.Fatalf("empty mode %q", emptyMode.Mode())
	}

	zero, err := FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer})
	if err != nil {
		t.Fatal(err)
	}
	if err := zero.RequireListen(); err == nil || err.Error() != "spec.auth.mode bearer requires at least one usable token" {
		t.Fatalf("zero bearer RequireListen: %v", err)
	}
	loop, err := FromSpec(model.AuthSpec{Mode: model.MgmtAuthDevLoopbackUnauth})
	if err != nil {
		t.Fatal(err)
	}
	if err := loop.RequireListen(); err != nil {
		t.Fatal(err)
	}
	var nilVerifier *Verifier
	if err := nilVerifier.RequireListen(); err == nil || err.Error() != "management bind requires a verifier" {
		t.Fatalf("nil RequireListen: %v", err)
	}
	if _, err := nilVerifier.Authenticate(Request{}); err == nil {
		t.Fatal("nil verifier must fail closed")
	} else {
		de, ok := domainerr.As(err)
		if !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" {
			t.Fatalf("nil authenticate: %v", err)
		}
	}
}

func TestCharacterizeSecretLinePadding(t *testing.T) {
	dir := t.TempDir()
	const secret = "0123456789abcdef0123456789abcdef"
	body := "\n# comment\n\u00a0\u2003" + secret + "\u00a0\r\n"
	path := writeSecret(t, dir, "padded", body)
	v, err := FromSpec(model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID:         "admin",
			Role:       model.RoleAdministrator,
			SecretFile: path,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.AuthenticateBearer(secret)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "admin" || p.Role != model.RoleAdministrator {
		t.Fatalf("%+v", p)
	}
	// AuthenticateBearer trims the presented secret with strings.TrimSpace,
	// so the same unicode padding still matches the stored digest.
	padded, err := v.AuthenticateBearer("\u00a0\u2003" + secret + "\u00a0\r")
	if err != nil {
		t.Fatal(err)
	}
	if padded.ID != p.ID {
		t.Fatalf("%+v", padded)
	}
}

func TestCharacterizePaddedRole(t *testing.T) {
	dir := t.TempDir()
	path := writeSecret(t, dir, "role", "0123456789abcdef0123456789abcdef\n")
	for _, role := range []string{" administrator", "administrator ", "administrator\t", "Administrator"} {
		_, err := FromSpec(model.AuthSpec{
			Mode: model.MgmtAuthBearer,
			Tokens: []model.TokenSpec{{
				ID:         "admin",
				Role:       role,
				SecretFile: path,
			}},
		})
		assertLoad(t, err, "unknown role", "spec.auth.tokens[0].role", "invalid_value", "role must be viewer, operator, or administrator")
	}
}
