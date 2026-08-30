package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
)

func TestParityEveryRequiredRowHasMCPTool(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	live := map[string]bool{}
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		live[tool.Name] = true
	}
	for _, c := range capabilities.All() {
		if c.RESTOnly || c.DifferentBinding {
			continue
		}
		if c.MCP == nil || len(c.MCP.Tools) == 0 {
			t.Errorf("%s missing MCP tool", c.ID)
			continue
		}
		for _, name := range c.MCP.Tools {
			if !strings.HasPrefix(name, "ntp_") {
				t.Errorf("%s tool %s must use ntp_ prefix", c.ID, name)
			}
			if strings.HasPrefix(name, "labntp_") {
				t.Errorf("%s tool %s uses rejected labntp_ prefix", c.ID, name)
			}
			if !live[name] {
				t.Errorf("%s tool %s not registered on the live server", c.ID, name)
			}
		}
	}
}

func TestParityGoldens(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "testdata", "mcp", "goldens")
	compareLines(t, filepath.Join(dir, "tools.txt"), capabilities.Tools())
	compareLines(t, filepath.Join(dir, "resources.txt"), capabilities.Resources())
	compareLines(t, filepath.Join(dir, "features.txt"), capabilities.FeatureIDs())
}

func TestParityMCPMutationsHaveREST(t *testing.T) {
	for _, c := range capabilities.All() {
		if !c.Mutating {
			continue
		}
		if c.RESTOnly {
			continue
		}
		if len(c.REST) == 0 {
			t.Errorf("%s mutating without REST", c.ID)
		}
		if c.MCP == nil || len(c.MCP.Tools) == 0 {
			t.Errorf("%s mutating without MCP tool", c.ID)
		}
	}
}

func TestStatelessTrue(t *testing.T) {
	s, _ := newTestServer(t)
	if s.http == nil {
		t.Fatal("http handler")
	}
}

func compareLines(t *testing.T, path string, got []string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Split(strings.TrimSpace(string(body)), "\n")
	if strings.TrimSpace(string(body)) == "" {
		want = nil
	}
	if len(want) != len(got) {
		t.Fatalf("%s: got %d lines want %d\ngot=%v\nwant=%v", path, len(got), len(want), got, want)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("%s:%d got %q want %q", path, i+1, got[i], want[i])
		}
	}
}
