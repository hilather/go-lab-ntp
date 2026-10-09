package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/testutil/bootsuite"
)

func TestBootManagementOffNoSecretRead(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("uid 0 can read a mode 000 token file")
	}
	t.Cleanup(func() { wrapTokenSource = nil })
	bootsuite.Run(t, newBootDriver(t))
}

type bootDriver struct {
	t     *testing.T
	dirs  map[string]string
	paths map[string]string
}

func newBootDriver(t *testing.T) *bootDriver {
	t.Helper()
	d := &bootDriver{t: t, dirs: map[string]string{}, paths: map[string]string{}}
	for _, shape := range []string{"absent", "short", "mode000"} {
		dir := t.TempDir()
		d.dirs[shape] = dir
		switch shape {
		case "absent":
			d.paths[shape] = filepath.Join(dir, "missing.token")
		case "short":
			path := filepath.Join(dir, "short.token")
			if err := os.WriteFile(path, []byte("too-short\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			d.paths[shape] = path
		case "mode000":
			path := filepath.Join(dir, "secret.token")
			if err := os.WriteFile(path, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			d.paths[shape] = path
		}
	}
	return d
}

func (d *bootDriver) Files(arm, files string) []string {
	if arm == "off" {
		return nil
	}
	return []string{d.paths[files]}
}

func (d *bootDriver) BoundMessage(files string) string {
	if files == "short" {
		return "labntp serve: auth: validation_failed: token entropy is below 256 bits\n"
	}
	return "labntp serve: auth: validation_failed: token secret is unavailable\n"
}

func (d *bootDriver) Boot(_ context.Context, arm, files string) bootsuite.BootResult {
	d.t.Helper()
	opens := map[string]int{}
	var mu sync.Mutex
	calls := 0
	wrapTokenSource = func(src auth.TokenSource) auth.TokenSource {
		return &countSrc{inner: src, mu: &mu, opens: opens, calls: &calls}
	}
	defer func() { wrapTokenSource = nil }()

	dir := d.dirs[files]
	ntpAddr := reserveUDP(d.t)
	cfg := writeBootYAML(d.t, dir, d.paths[files], ntpAddr)
	mgmt := "off"
	if arm == "bound" {
		mgmt = reserveTCP(d.t)
	}
	args := []string{
		"--config", cfg,
		"--ntp-listen", ntpAddr,
		"--management-listen", mgmt,
		"--shutdown-timeout", "200ms",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr lockedBuf

	if arm == "bound" {
		code := serveCmd(ctx, args, &stdout, &stderr)
		return bootsuite.BootResult{
			Booted:      code == 0,
			DataPlaneOK: false,
			Opens:       snapshotOpens(&mu, opens),
			Message:     authStderrLine(stderr.String()),
		}
	}

	errCh := make(chan int, 1)
	go func() {
		errCh <- serveCmd(ctx, args, &stdout, &stderr)
	}()
	deadline := time.Now().Add(5 * time.Second)
	want := "labntp ntp listen=" + ntpAddr
	for !strings.Contains(stdout.String(), want) {
		select {
		case code := <-errCh:
			d.t.Fatalf("off %s exited %d stdout %q stderr %q", files, code, stdout.String(), stderr.String())
		default:
		}
		if time.Now().After(deadline) {
			d.t.Fatalf("off %s timeout stdout %q stderr %q", files, stdout.String(), stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	called := calls
	mu.Unlock()
	if called != 0 {
		d.t.Fatalf("off %s opened a secret file (%d reads)", files, called)
	}
	assertNTPAnswer(d.t, ntpAddr)
	cancel()
	select {
	case code := <-errCh:
		if code != 0 {
			d.t.Fatalf("off %s exit %d stderr %q", files, code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		d.t.Fatal("serve did not exit")
	}
	mu.Lock()
	called = calls
	mu.Unlock()
	if called != 0 {
		d.t.Fatalf("off %s opened a secret file after the probe (%d reads)", files, called)
	}
	return bootsuite.BootResult{
		Booted:      true,
		DataPlaneOK: true,
		Opens:       snapshotOpens(&mu, opens),
	}
}

type countSrc struct {
	inner auth.TokenSource
	mu    *sync.Mutex
	opens map[string]int
	calls *int
}

func (c *countSrc) Read() ([]auth.RawToken, []auth.FileResult, error) {
	c.mu.Lock()
	*c.calls++
	c.mu.Unlock()
	toks, files, err := c.inner.Read()
	c.mu.Lock()
	for _, f := range files {
		c.opens[f.Path]++
	}
	c.mu.Unlock()
	return toks, files, err
}

func (c *countSrc) Spec() any { return c.inner.Spec() }

func snapshotOpens(mu *sync.Mutex, opens map[string]int) map[string]int {
	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]int, len(opens))
	for k, v := range opens {
		out[k] = v
	}
	return out
}

func authStderrLine(stderr string) string {
	const prefix = "labntp serve: auth:"
	for _, line := range strings.SplitAfter(stderr, "\n") {
		if strings.HasPrefix(line, prefix) {
			if !strings.HasSuffix(line, "\n") {
				return line + "\n"
			}
			return line
		}
	}
	return stderr
}
