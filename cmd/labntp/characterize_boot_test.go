package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/ntpwire"
)

func TestCharacterizeBootBoundSecret(t *testing.T) {
	const unavailable = "labntp serve: auth: validation_failed: token secret is unavailable\n"
	const entropy = "labntp serve: auth: validation_failed: token entropy is below 256 bits\n"

	t.Run("absent", func(t *testing.T) {
		dir := t.TempDir()
		code, stderr, _ := serveAuthFailure(t, dir, filepath.Join(dir, "missing.token"), "")
		if code != 1 || stderr != unavailable {
			t.Fatalf("code %d stderr %q", code, stderr)
		}
	})
	t.Run("mode 000", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("uid 0 can read a mode 000 token file")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "secret.token")
		if err := os.WriteFile(path, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
		code, stderr, _ := serveAuthFailure(t, dir, path, "")
		if code != 1 || stderr != unavailable {
			t.Fatalf("code %d stderr %q", code, stderr)
		}
	})
	t.Run("short", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "short.token")
		if err := os.WriteFile(path, []byte("too-short\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		code, stderr, _ := serveAuthFailure(t, dir, path, "")
		if code != 1 || stderr != entropy {
			t.Fatalf("code %d stderr %q", code, stderr)
		}
	})
	t.Run("management off answers without the secret", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "missing.token")
		ntpAddr := reserveUDP(t)
		cfg := writeBootYAML(t, dir, missing, ntpAddr)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var stdout, stderr lockedBuf
		errCh := make(chan int, 1)
		go func() {
			errCh <- serveCmd(ctx, []string{
				"--config", cfg,
				"--ntp-listen", ntpAddr,
				"--management-listen", "off",
				"--shutdown-timeout", "200ms",
			}, &stdout, &stderr)
		}()
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(stdout.String(), "labntp management: not bound") || !strings.Contains(stdout.String(), "labntp ntp listen=") {
			select {
			case code := <-errCh:
				t.Fatalf("serve exited %d stdout %q stderr %q", code, stdout.String(), stderr.String())
			default:
			}
			if time.Now().After(deadline) {
				t.Fatalf("timeout stdout %q stderr %q", stdout.String(), stderr.String())
			}
			time.Sleep(20 * time.Millisecond)
		}
		if strings.Contains(stderr.String(), "validation_failed") || strings.Contains(stderr.String(), "token secret") {
			t.Fatalf("management-off read a secret: %q", stderr.String())
		}
		assertNTPAnswer(t, ntpAddr)
		cancel()
		select {
		case code := <-errCh:
			if code != 0 {
				t.Fatalf("exit %d stderr %q", code, stderr.String())
			}
		case <-time.After(5 * time.Second):
			t.Fatal("serve did not exit")
		}
	})
}

// lockedBuf is a bytes.Buffer safe for the serve goroutine and the test goroutine.
type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func serveAuthFailure(t *testing.T, dir, secretPath, secretBody string) (int, string, string) {
	t.Helper()
	_ = secretBody
	ntpAddr := reserveUDP(t)
	cfg := writeBootYAML(t, dir, secretPath, ntpAddr)
	var stdout, stderr bytes.Buffer
	code := serveCmd(context.Background(), []string{
		"--config", cfg,
		"--ntp-listen", ntpAddr,
		"--management-listen", reserveTCP(t),
		"--shutdown-timeout", "200ms",
	}, &stdout, &stderr)
	return code, stderr.String(), stdout.String()
}

func writeBootYAML(t *testing.T, dir, secretPath, ntpAddr string) string {
	t.Helper()
	body := fmt.Sprintf(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: boot-secret
spec:
  listeners:
    ntp:
      address: %q
    management:
      address: "127.0.0.1:9"
  auth:
    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: %q
  ui:
    enabled: false
  ntp:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
  filters:
    - name: default
      enabled: true
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`, ntpAddr, secretPath)
	path := filepath.Join(dir, "labntp.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func reserveUDP(t *testing.T) string {
	t.Helper()
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.LocalAddr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func reserveTCP(t *testing.T) string {
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

func assertNTPAnswer(t *testing.T, addr string) {
	t.Helper()
	c, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if err := c.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	req := ntpwire.Encode(ntpwire.Packet{VN: 4, Mode: ntpwire.ModeClient, XmtTime: ntpwire.FromTime(time.Now())})
	if _, err := c.Write(req); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 128)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := ntpwire.Parse(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if rep.Mode != ntpwire.ModeServer || rep.VN != 4 {
		t.Fatalf("reply %+v", rep)
	}
}
