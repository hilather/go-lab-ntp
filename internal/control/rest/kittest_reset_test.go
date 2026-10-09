package rest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-controlkit/kittest"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/querylog"
)

func TestKittestResetZeroTokens(t *testing.T) {
	kittest.ResetZeroTokens(t, &zeroTokenDriver{t: t})
}

func TestKittestResetUnreadableSecret(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("uid 0 can read a mode 000 token file")
	}
	kittest.ResetUnreadableSecret(t, newUnreadableDriver(t))
}

type zeroTokenDriver struct {
	t *testing.T
}

func (d *zeroTokenDriver) Shapes() []kittest.ZeroTokenShape {
	return []kittest.ZeroTokenShape{kittest.ZeroNTPBearer, kittest.ZeroNTPLoopback}
}

func (d *zeroTokenDriver) Apply(ctx context.Context, shape kittest.ZeroTokenShape) kittest.ZeroTokenResult {
	d.t.Helper()
	svc, s, verifier, cookie, boot := bootReload(d.t)
	before := string(svc.Active().Revision)
	dir := filepath.Dir(boot)
	switch shape {
	case kittest.ZeroNTPBearer:
		writeReloadBoot(d.t, dir, `    mode: bearer
    tokens: []
`)
	case kittest.ZeroNTPLoopback:
		writeReloadBoot(d.t, dir, `    mode: dev-loopback-unauth
    tokens: []
`)
	default:
		d.t.Fatalf("shape %s", shape)
	}
	_, err := svc.Reset(ctx, app.Actor{ID: "admin", Transport: "rest"}, app.ResetIn{Reason: "rotate"})
	code := ""
	if err != nil {
		de, ok := domainerr.As(err)
		if !ok {
			d.t.Fatal(err)
		}
		code = string(de.Code)
	}
	_, berr := verifier.Authenticate(auth.Request{Authorization: "Bearer " + reloadSecret})
	_, _, sok := s.cfg.Sessions.Lookup(cookie)
	after := string(svc.Active().Revision)
	return kittest.ZeroTokenResult{
		Code:              code,
		OldBearerWorks:    berr == nil,
		OldCookieWorks:    sok,
		RevisionUnchanged: after == before,
	}
}

type unreadableDriver struct {
	t        *testing.T
	svc      *app.App
	s        *Server
	verifier *auth.Verifier
	cookie   string
	dir      string
}

func newUnreadableDriver(t *testing.T) *unreadableDriver {
	t.Helper()
	svc, s, verifier, cookie, boot := bootReload(t)
	// A refused reset must not clear the query log. Seeding makes that
	// observable: SideEffects is nonzero before the reset and stays equal.
	q := svc.QueryLog()
	if q == nil || !q.TryInsert(querylog.Entry{Filter: "seed"}) || len(q.List()) == 0 {
		t.Fatal("query log seed failed")
	}
	return &unreadableDriver{
		t: t, svc: svc, s: s, verifier: verifier, cookie: cookie, dir: filepath.Dir(boot),
	}
}

func (d *unreadableDriver) Code() string { return "validation_failed" }

func (d *unreadableDriver) Observe(context.Context) kittest.ResetView {
	d.t.Helper()
	snap := d.svc.Active()
	if snap == nil {
		d.t.Fatal("no snapshot")
	}
	_, berr := d.verifier.Authenticate(auth.Request{Authorization: "Bearer " + reloadSecret})
	_, _, sok := d.s.cfg.Sessions.Lookup(d.cookie)
	side := 0
	if q := d.svc.QueryLog(); q != nil {
		side = len(q.List())
	}
	return kittest.ResetView{
		Revision:     string(snap.Revision),
		SideEffects:  side,
		Listeners:    []string{snap.NTPAddress, snap.ManagementAddress},
		BearerWorks:  berr == nil,
		SessionWorks: sok,
	}
}

func (d *unreadableDriver) Reset(ctx context.Context, kind string) (string, error) {
	d.t.Helper()
	var secretPath string
	switch kind {
	case "missing":
		secretPath = filepath.Join(d.t.TempDir(), "missing.token")
	case "unreadable":
		secretPath = filepath.Join(d.dir, "unreadable.token")
		if err := os.WriteFile(secretPath, []byte(reloadSecret+"\n"), 0o600); err != nil {
			return "", err
		}
		if err := os.Chmod(secretPath, 0); err != nil {
			return "", err
		}
		d.t.Cleanup(func() { _ = os.Chmod(secretPath, 0o600) })
	case "short":
		secretPath = filepath.Join(d.dir, "short.token")
		if err := os.WriteFile(secretPath, []byte("too-short\n"), 0o600); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("kind %s", kind)
	}
	writeReloadBoot(d.t, d.dir, fmt.Sprintf(`    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: %q
`, secretPath))
	_, err := d.svc.Reset(ctx, app.Actor{ID: "admin", Transport: "rest"}, app.ResetIn{Reason: "rotate"})
	if err == nil {
		return "", fmt.Errorf("reset succeeded")
	}
	de, ok := domainerr.As(err)
	if !ok {
		return "", err
	}
	return string(de.Code), nil
}
