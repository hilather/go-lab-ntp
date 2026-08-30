package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeUIEnabledIsHTML(t *testing.T) {
	addr := serveWithUI(t, true)
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / code=%d body=%s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type=%q", ct)
	}
	if !strings.Contains(string(body), "LabNTP") {
		t.Fatalf("body=%s", body)
	}
}

func TestServeUIDisabledIs404(t *testing.T) {
	addr := serveWithUI(t, false)
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET / code=%d body=%s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("content-type=%q body=%s", ct, body)
	}
	if strings.Contains(string(body), "<!doctype") {
		t.Fatalf("disabled UI served HTML: %s", body)
	}
}

func serveWithUI(t *testing.T, uiEnabled bool) string {
	t.Helper()
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ntpLn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ntpAddr := ntpLn.LocalAddr().String()
	_ = ntpLn.Close()
	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mgmtAddr := httpLn.Addr().String()
	_ = httpLn.Close()

	cfg := filepath.Join(dir, "labntp.yaml")
	body := fmt.Sprintf(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: serve-ui
spec:
  listeners:
    ntp:
      address: %q
    management:
      address: %q
  auth:
    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: %q
  ui:
    enabled: %t
  ntp:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
  filters:
    - name: default
      enabled: true
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`, ntpAddr, mgmtAddr, tokenPath, uiEnabled)
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var stdout, stderr strings.Builder
	errCh := make(chan int, 1)
	go func() {
		errCh <- serveCmd(ctx, []string{
			"--config", cfg,
			"--ntp-listen", ntpAddr,
			"--management-listen", mgmtAddr,
		}, &stdout, &stderr)
	}()

	deadline := time.Now().Add(5 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		select {
		case code := <-errCh:
			t.Fatalf("serve exited %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		default:
		}
		resp, err := http.Get("http://" + mgmtAddr + "/v1/health/live")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return mgmtAddr
			}
			last = fmt.Errorf("live status %d", resp.StatusCode)
		} else {
			last = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("management never became live: %v stdout=%q stderr=%q", last, stdout.String(), stderr.String())
	return ""
}
