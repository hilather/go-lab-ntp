package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/config"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labntp", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "labntp") {
		t.Fatalf("version output %q missing labntp", stdout.String())
	}
	if !strings.Contains(stdout.String(), "labntp.dev/v1alpha1") {
		t.Fatalf("version %q missing config API", stdout.String())
	}
	if !strings.Contains(stdout.String(), "2026-07-28") {
		t.Fatalf("version %q missing MCP protocol", stdout.String())
	}
}

func TestUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labntp"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr %q missing usage", stderr.String())
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labntp", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	out := stdout.String()
	for _, s := range []string{"version", "serve", "validate", "canonicalize", "query", "healthcheck"} {
		if !strings.Contains(out, s) {
			t.Fatalf("help missing %s: %q", s, out)
		}
	}
}

func TestValidateAndCanonicalize(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata/config/valid/full.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labntp", "validate", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok revision=") {
		t.Fatalf("validate %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"labntp", "canonicalize", "--config", path, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("canonicalize exit %d stderr=%q", code, stderr.String())
	}
	st, err := config.Load(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != "LabNTP" {
		t.Fatalf("kind %q", st.Kind)
	}
}

func TestValidateRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labntp", "validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestQueryImportFence(t *testing.T) {
	// query.go lives in cmd/labntp; ntpserver import test covers the other direction.
	if _, err := os.Stat(filepath.Join(repoRoot(t), "cmd/labntp/query.go")); err != nil {
		t.Fatal(err)
	}
}
