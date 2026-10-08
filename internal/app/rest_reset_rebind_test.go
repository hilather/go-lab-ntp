package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/control/rest"
)

const rebindToken = "0123456789abcdef0123456789abcdef"

// TestRESTResetRebindsManagementWithoutSelfDrain posts a real reset over the
// management listener. Rebind must not wait on that request.
func TestRESTResetRebindsManagementWithoutSelfDrain(t *testing.T) {
	t.Run("move", func(t *testing.T) {
		env := bootManagement(t)
		next := freeAddr(t)
		env.rewrite(t, next, "rebind")
		beforeGen := env.svc.Active().Generation

		status, body, elapsed := env.postReset(t)
		t.Logf("move elapsed=%s status=%d body=%s", elapsed, status, body)
		if status != http.StatusOK || elapsed >= time.Second {
			t.Fatalf("move status=%d elapsed=%s body=%s", status, elapsed, body)
		}
		assertApplied(t, body, uint64(beforeGen))

		if env.srv.Addr() != next || !env.srv.Bound() {
			t.Fatalf("after move addr=%s bound=%v want %s bound", env.srv.Addr(), env.srv.Bound(), next)
		}
		if got := env.svc.Active().ManagementAddress; got != next {
			t.Fatalf("active management %q want %q", got, next)
		}
		resp := env.get(t, "http://"+next+"/v1/state")
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("new listener GET status %d", resp.StatusCode)
		}
		mustRefused(t, env.addr)
		env.wantOriginalServeDone(t)
	})

	t.Run("off", func(t *testing.T) {
		env := bootManagement(t)
		beforeGen := env.svc.Active().Generation
		env.svc.SetMgmtOverrideForTest("off")

		status, body, elapsed := env.postReset(t)
		t.Logf("off elapsed=%s status=%d body=%s bound=%v dial=%s active=%s gen=%d",
			elapsed, status, body, env.srv.Bound(), dialResult(env.addr), env.svc.Active().ManagementAddress, env.svc.Active().Generation)
		if status != http.StatusOK || elapsed >= time.Second {
			t.Fatalf("off status=%d elapsed=%s body=%s", status, elapsed, body)
		}
		assertApplied(t, body, uint64(beforeGen))
		if env.srv.Bound() {
			t.Fatal("management still bound after off")
		}
		mustMgmtStatus(t, env.svc, "off")
		mustRefused(t, env.addr)
		env.wantOriginalServeDone(t)
	})

	t.Run("taken_port", func(t *testing.T) {
		env := bootManagement(t)
		hold, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = hold.Close() })
		taken := hold.Addr().String()
		env.rewrite(t, taken, "rebind")
		before := env.svc.Active()
		beforeRev, beforeGen, beforeMgmt := before.Revision, before.Generation, before.ManagementAddress

		status, body, elapsed := env.postReset(t)
		t.Logf("taken_port elapsed=%s status=%d body=%s", elapsed, status, body)
		if elapsed >= time.Second {
			t.Fatalf("taken_port elapsed=%s status=%d body=%s", elapsed, status, body)
		}
		if status/100 == 2 {
			t.Fatalf("taken_port status=%d body=%s", status, body)
		}
		var prob struct {
			Status int    `json:"status"`
			Code   string `json:"code"`
		}
		if err := json.Unmarshal(body, &prob); err != nil {
			t.Fatalf("problem: %v body %s", err, body)
		}
		t.Logf("taken_port problem status=%d code=%s", prob.Status, prob.Code)
		if status != http.StatusInternalServerError || prob.Code != "internal_error" {
			t.Fatalf("taken_port status=%d code=%s body=%s", status, prob.Code, body)
		}
		snap := env.svc.Active()
		if snap.Revision != beforeRev || snap.Generation != beforeGen || snap.ManagementAddress != beforeMgmt {
			t.Fatalf("snapshot changed rev=%s gen=%d mgmt=%s", snap.Revision, snap.Generation, snap.ManagementAddress)
		}
		resp := env.get(t, "http://"+env.addr+"/v1/state")
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("old listener GET status %d", resp.StatusCode)
		}
		if !env.srv.Bound() || env.srv.Addr() != env.addr {
			t.Fatalf("bound=%v addr=%s want %s", env.srv.Bound(), env.srv.Addr(), env.addr)
		}
	})

	t.Run("same_address", func(t *testing.T) {
		env := bootManagement(t)
		env.rewrite(t, env.addr, "rebind-same")
		beforeGen := env.svc.Active().Generation

		status, body, elapsed := env.postReset(t)
		t.Logf("same_address elapsed=%s status=%d body=%s", elapsed, status, body)
		if status != http.StatusOK || elapsed >= time.Second {
			t.Fatalf("same_address status=%d elapsed=%s body=%s", status, elapsed, body)
		}
		assertApplied(t, body, uint64(beforeGen))
		if !env.srv.Bound() || env.srv.Addr() != env.addr {
			t.Fatalf("bound=%v addr=%s", env.srv.Bound(), env.srv.Addr())
		}
		resp := env.get(t, "http://"+env.addr+"/v1/state")
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("same listener GET status %d", resp.StatusCode)
		}
	})
}

type mgmtEnv struct {
	svc      *app.App
	srv      *rest.Server
	client   *http.Client
	serveErr chan error
	path     string
	secret   string
	addr     string
	ntp      string
}

func bootManagement(t *testing.T) mgmtEnv {
	t.Helper()
	dir := t.TempDir()
	secret := filepath.Join(dir, "admin.token")
	if err := os.WriteFile(secret, []byte(rebindToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	addr := freeAddr(t)
	path := filepath.Join(dir, "labntp.yaml")
	const ntp = "127.0.0.1:10123"
	writeBoot(t, path, ntp, addr, secret, "rebind")

	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	verifier, err := auth.FromSpec(svc.Active().Canonical.Spec.Auth)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := rest.New(rest.Config{
		Service:    svc,
		Auth:       verifier,
		RatePerSec: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetHTTPRebind(srv.Rebind)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return mgmtEnv{
		svc: svc,
		srv: srv,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
		serveErr: serveErr,
		path:     path,
		secret:   secret,
		addr:     addr,
		ntp:      ntp,
	}
}

func (e mgmtEnv) rewrite(t *testing.T, mgmt, name string) {
	t.Helper()
	writeBoot(t, e.path, e.ntp, mgmt, e.secret, name)
}

func (e mgmtEnv) postReset(t *testing.T) (int, []byte, time.Duration) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://"+e.addr+"/v1/state:reset", strings.NewReader(`{"reason":"rebind"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+rebindToken)
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := e.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("reset after %s: %v", elapsed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body, elapsed
}

func (e mgmtEnv) wantOriginalServeDone(t *testing.T) {
	t.Helper()
	select {
	case err := <-e.serveErr:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("original Serve still running")
	}
}

func (e mgmtEnv) get(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+rebindToken)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertApplied(t *testing.T, body []byte, before uint64) {
	t.Helper()
	var got struct {
		Applied    bool   `json:"applied"`
		Generation uint64 `json:"generation"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("apply body: %v %s", err, body)
	}
	if !got.Applied || got.Generation != before+1 {
		t.Fatalf("applied=%v generation=%d want %d body=%s", got.Applied, got.Generation, before+1, body)
	}
}

func mustMgmtStatus(t *testing.T, svc *app.App, want string) {
	t.Helper()
	st, err := svc.Status(context.Background(), app.Actor{ID: "admin", Transport: "rest"})
	if err != nil {
		t.Fatal(err)
	}
	for _, ln := range st.Listeners {
		if ln.Name == "management" {
			if ln.Address != want {
				t.Fatalf("status management %q want %q", ln.Address, want)
			}
			return
		}
	}
	t.Fatalf("status has no management listener: %+v", st.Listeners)
}

func mustRefused(t *testing.T, addr string) {
	t.Helper()
	if got := dialResult(addr); got != "refused" {
		t.Fatalf("dial %s: %s", addr, got)
	}
}

func dialResult(addr string) string {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err == nil {
		_ = conn.Close()
		return "connected"
	}
	if isRefused(err) {
		return "refused"
	}
	return err.Error()
}

func isRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func writeBoot(t *testing.T, path, ntp, mgmt, secret, name string) {
	t.Helper()
	body := fmt.Sprintf(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: %s
spec:
  listeners:
    ntp:
      address: %q
    management:
      address: %q
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
        role: administrator
        secretFile: %q
`, name, ntp, mgmt, secret)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
