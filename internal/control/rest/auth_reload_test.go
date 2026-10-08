package rest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

const reloadSecret = "0123456789abcdef0123456789abcdef"

func TestResetUnreadableSecretKeepsSnapshotAndVerifier(t *testing.T) {
	t.Run("missing secret file", func(t *testing.T) {
		svc, s, verifier, cookie, boot := bootReload(t)
		missing := filepath.Join(t.TempDir(), "missing.token")
		writeReloadBoot(t, filepath.Dir(boot), fmt.Sprintf(`    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: %q
`, missing))
		assertResetKeepsAuth(t, svc, s, verifier, cookie, reloadSecret)
	})
	t.Run("TestResetEmptyBearerKeepsSnapshotAndVerifier", func(t *testing.T) {
		svc, s, verifier, cookie, boot := bootReload(t)
		writeReloadBoot(t, filepath.Dir(boot), `    mode: bearer
    tokens: []
`)
		assertResetKeepsAuth(t, svc, s, verifier, cookie, reloadSecret)
	})
}

func TestResetReadableDemotionRevokesOldBearer(t *testing.T) {
	dir := t.TempDir()
	oldSecret := filepath.Join(dir, "old.token")
	newSecretPath := filepath.Join(dir, "new.token")
	const newSecret = "fedcba9876543210fedcba9876543210"
	if err := os.WriteFile(oldSecret, []byte(reloadSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newSecretPath, []byte(newSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeReloadBoot(t, dir, fmt.Sprintf(`    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: %q
`, oldSecret))
	svc, s, verifier, cookie := bootReloadAt(t, filepath.Join(dir, "labntp.yaml"))
	writeReloadBoot(t, dir, fmt.Sprintf(`    mode: bearer
    tokens:
      - id: admin
        role: viewer
        secretFile: %q
`, newSecretPath))
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "admin", Transport: "rest"}, app.ResetIn{Reason: "demote"}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := verifier.Authenticate(auth.Request{Authorization: "Bearer " + reloadSecret}); err == nil {
		t.Fatal("old bearer still authenticates after a readable demotion")
	}
	if _, err := verifier.Authenticate(auth.Request{Authorization: "Bearer " + newSecret}); err != nil {
		t.Fatalf("new bearer: %v", err)
	}
	if _, _, ok := s.cfg.Sessions.Lookup(cookie); ok {
		t.Fatal("pre-reset session still looks up")
	}
}

func bootReload(t *testing.T) (*app.App, *Server, *auth.Verifier, string, string) {
	t.Helper()
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "old.token")
	if err := os.WriteFile(secretPath, []byte(reloadSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	boot := writeReloadBoot(t, dir, fmt.Sprintf(`    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: %q
`, secretPath))
	svc, s, verifier, cookie := bootReloadAt(t, boot)
	return svc, s, verifier, cookie, boot
}

func bootReloadAt(t *testing.T, path string) (*app.App, *Server, *auth.Verifier, string) {
	t.Helper()
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	verifier, err := auth.FromSpec(svc.Active().Canonical.Spec.Auth)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: verifier, RatePerSec: -1})
	if err != nil || s == nil {
		t.Fatal(err)
	}
	p, err := verifier.Authenticate(auth.Request{Authorization: "Bearer " + reloadSecret})
	if err != nil {
		t.Fatal(err)
	}
	cookie, _, _, err := s.cfg.Sessions.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	return svc, s, verifier, cookie
}

func writeReloadBoot(t *testing.T, dir, authBody string) string {
	t.Helper()
	body := fmt.Sprintf(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: reload
spec:
  filters:
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
  auth:
%s`, authBody)
	path := filepath.Join(dir, "labntp.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertResetKeepsAuth(t *testing.T, svc *app.App, s *Server, verifier *auth.Verifier, cookie, secret string) {
	t.Helper()
	rev := svc.Active().Revision
	_, err := svc.Reset(context.Background(), app.Actor{ID: "admin", Transport: "rest"}, app.ResetIn{Reason: "rotate"})
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("reset err=%v want validation_failed", err)
	}
	if got := svc.Active().Revision; got != rev {
		t.Fatalf("revision changed %s -> %s", rev, got)
	}
	if _, err := verifier.Authenticate(auth.Request{Authorization: "Bearer " + secret}); err != nil {
		t.Fatalf("old bearer rejected: %v", err)
	}
	if _, _, ok := s.cfg.Sessions.Lookup(cookie); !ok {
		t.Fatal("pre-reset session no longer looks up")
	}
}
