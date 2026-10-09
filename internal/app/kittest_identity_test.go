package app_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-controlkit/kittest"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/control/mcp"
	"github.com/hilather/go-lab-ntp/internal/control/rest"
)

func TestIdentityChangeClearsSessions(t *testing.T) {
	kittest.IdentityChangeClearsSessions(t, &identityDriver{t: t})
}

type identityDriver struct {
	t        *testing.T
	svc      *app.App
	verifier *auth.Verifier
	sessions *auth.Sessions
	path     string
	secret   string
}

func (d *identityDriver) Boot(_ context.Context, order kittest.IdentityOrder) {
	d.t.Helper()
	dir := d.t.TempDir()
	secretPath := filepath.Join(dir, "admin.token")
	if err := os.WriteFile(secretPath, []byte(rebindToken+"\n"), 0o600); err != nil {
		d.t.Fatal(err)
	}
	path := filepath.Join(dir, "labntp.yaml")
	writeIdentityYAML(d.t, path, secretPath, "administrator")
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: path})
	if err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(svc.Close)
	verifier, err := auth.FromSpec(svc.Active().Canonical.Spec.Auth)
	if err != nil {
		d.t.Fatal(err)
	}
	sessions := auth.NewStore(auth.DefaultSessionConfig())
	mcpCfg := mcp.Config{Service: svc, Auth: verifier, RatePerSec: -1}
	restCfg := rest.Config{Service: svc, Auth: verifier, Sessions: sessions, RatePerSec: -1}
	switch order {
	case kittest.OrderReverse:
		if _, err := rest.New(restCfg); err != nil {
			d.t.Fatal(err)
		}
		if _, err := mcp.New(mcpCfg); err != nil {
			d.t.Fatal(err)
		}
	default:
		if _, err := mcp.New(mcpCfg); err != nil {
			d.t.Fatal(err)
		}
		if _, err := rest.New(restCfg); err != nil {
			d.t.Fatal(err)
		}
	}
	d.svc = svc
	d.verifier = verifier
	d.sessions = sessions
	d.path = path
	d.secret = secretPath
}

func (d *identityDriver) Login(context.Context) string {
	d.t.Helper()
	p, err := d.verifier.Authenticate(auth.Request{Authorization: "Bearer " + rebindToken})
	if err != nil {
		d.t.Fatal(err)
	}
	cookie, _, _, err := d.sessions.Create(p)
	if err != nil {
		d.t.Fatal(err)
	}
	return cookie
}

func (d *identityDriver) Demote(ctx context.Context) {
	d.t.Helper()
	writeIdentityYAML(d.t, d.path, d.secret, "viewer")
	if _, err := d.svc.Reset(ctx, app.Actor{ID: "admin", Transport: "rest"}, app.ResetIn{Reason: "demote"}); err != nil {
		d.t.Fatal(err)
	}
}

func (d *identityDriver) CookieWorks(_ context.Context, cookie string) bool {
	_, _, ok := d.sessions.Lookup(cookie)
	return ok
}

func writeIdentityYAML(t *testing.T, path, secret, role string) {
	t.Helper()
	body := fmt.Sprintf(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: identity
spec:
  listeners:
    ntp:
      address: "127.0.0.1:10123"
    management:
      address: "127.0.0.1:9"
  filters:
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
  auth:
    mode: bearer
    tokens:
      - id: admin
        role: %s
        secretFile: %q
`, role, secret)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
