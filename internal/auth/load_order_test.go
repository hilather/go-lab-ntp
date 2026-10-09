package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-controlkit/authn"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// These faults are paired with a missing secret file. A loader that opens
// every file before the pre-facade checks reports the missing file instead
// of the earlier fault. The sentences are the ones FromSpec returned at
// d65c558 (internal/auth/verifier.go).

func TestLoadOrderUnknownModeMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.token")
	opens, err := loadOrder(t, model.AuthSpec{
		Mode: "pat",
		Tokens: []model.TokenSpec{{
			ID:         "admin",
			Role:       model.RoleAdministrator,
			SecretFile: missing,
		}},
	})
	assertMainLoad(t, err, "unknown auth mode", "spec.auth.mode", "invalid_value", "unknown auth mode")
	if opens[missing] != 0 || len(opens) != 0 {
		t.Fatalf("opened %v", opens)
	}
}

func TestLoadOrderBlankIDMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.token")
	opens, err := loadOrder(t, model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID:         "  ",
			Role:       model.RoleAdministrator,
			SecretFile: missing,
		}},
	})
	assertMainLoad(t, err, "token id is required", "spec.auth.tokens[0].id", "empty_id", "token id is required")
	if opens[missing] != 0 || len(opens) != 0 {
		t.Fatalf("opened %v", opens)
	}
}

func TestLoadOrderDuplicateIDMissingFile(t *testing.T) {
	dir := t.TempDir()
	good := writeOrderSecret(t, dir, "good.token", "0123456789abcdef0123456789abcdef\n")
	missing := filepath.Join(dir, "missing.token")
	opens, err := loadOrder(t, model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{
			{ID: "admin", Role: model.RoleAdministrator, SecretFile: good},
			{ID: "admin", Role: model.RoleViewer, SecretFile: missing},
		},
	})
	assertMainLoad(t, err, "duplicate token id", "spec.auth.tokens[1].id", "duplicate_id", "duplicate token id")
	if opens[good] != 1 || opens[missing] != 0 {
		t.Fatalf("opens %v", opens)
	}
}

func TestLoadOrderShortSecretThenMissingFile(t *testing.T) {
	dir := t.TempDir()
	short := writeOrderSecret(t, dir, "short.token", "too-short\n")
	missing := filepath.Join(dir, "missing.token")
	opens, err := loadOrder(t, model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{
			{ID: "admin", Role: model.RoleAdministrator, SecretFile: short},
			{ID: "other", Role: model.RoleViewer, SecretFile: missing},
		},
	})
	assertMainLoad(t, err, "token entropy is below 256 bits", "spec.auth.tokens[0].secretFile", "invalid_value", "token secret must be at least 32 bytes")
	if opens[short] != 1 || opens[missing] != 0 {
		t.Fatalf("opens %v", opens)
	}
}

func TestMapLoadErrUnrecognizedDoesNotClaimMissingFile(t *testing.T) {
	err := mapLoadErr(&authn.LoadError{
		Code:  "required",
		Field: "spec.auth.tokens[0].secretFile",
		Msg:   `secretFile "x": no such file or directory`,
	})
	de, ok := domainerr.As(err)
	if !ok {
		t.Fatalf("err=%v", err)
	}
	if de.Message != "validation failed" {
		t.Fatalf("top %q", de.Message)
	}
	if strings.Contains(err.Error(), "does not resolve") || strings.Contains(err.Error(), "unavailable") || strings.Contains(err.Error(), "no such file") {
		t.Fatalf("missing-file claim: %v", err)
	}
}

func TestMapLoadErrNonLoadErrorIsNeutral(t *testing.T) {
	err := mapLoadErr(errors.New("authn: mode is required"))
	de, ok := domainerr.As(err)
	if !ok {
		t.Fatalf("err=%v", err)
	}
	if de.Message != "token configuration is invalid" {
		t.Fatalf("top %q", de.Message)
	}
	if strings.Contains(err.Error(), "authn:") {
		t.Fatalf("kit sentence leaked: %v", err)
	}
}

// loadOrder compiles spec through FromSpecWith and records secret paths Read
// returned. FromSpec is the same call with a nil wrap.
func loadOrder(t *testing.T, spec model.AuthSpec) (map[string]int, error) {
	t.Helper()
	opens := map[string]int{}
	_, err := FromSpecWith(spec, func(src TokenSource) TokenSource {
		return &orderCount{inner: src, opens: opens}
	})
	return opens, err
}

type orderCount struct {
	inner TokenSource
	opens map[string]int
}

func (c *orderCount) Read() ([]RawToken, []FileResult, error) {
	toks, files, err := c.inner.Read()
	for _, f := range files {
		c.opens[f.Path]++
	}
	return toks, files, err
}

func (c *orderCount) Spec() any { return c.inner.Spec() }

func assertMainLoad(t *testing.T, err error, top, path, code, message string) {
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
		t.Fatalf("violation %+v want path %s code %s message %s", v, path, code, message)
	}
}

func writeOrderSecret(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
