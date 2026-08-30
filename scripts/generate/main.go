// Command generate writes api/capabilities/v1.json, api/openapi/v1.json,
// api/mcp/v1.json, and api/metrics/v1alpha1.json.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/control/mcp"
	"github.com/hilather/go-lab-ntp/internal/control/rest"
	"github.com/hilather/go-lab-ntp/internal/observability"
)

func main() {
	check := flag.Bool("check", false, "fail if generated files are stale")
	flag.Parse()
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	files, err := plannedFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	if *check {
		if err := checkFiles(root, files); err != nil {
			fmt.Fprintf(os.Stderr, "generate: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := writeFiles(root, files); err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
}

type artifact struct {
	rel  string
	body []byte
}

func plannedFiles() ([]artifact, error) {
	manifest, err := capabilities.RenderManifest()
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	openapi, err := rest.RenderOpenAPI()
	if err != nil {
		return nil, fmt.Errorf("openapi: %w", err)
	}
	mcpManifest, err := mcp.RenderManifest()
	if err != nil {
		return nil, fmt.Errorf("mcp: %w", err)
	}
	metrics, err := observability.RenderCatalog()
	if err != nil {
		return nil, fmt.Errorf("metrics: %w", err)
	}
	return []artifact{
		{capabilities.ManifestRelPath, manifest},
		{rest.OpenAPIRelPath, openapi},
		{mcp.ManifestRelPath, mcpManifest},
		{observability.CatalogRelPath, metrics},
	}, nil
}

func checkFiles(root string, files []artifact) error {
	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.rel))
		cur, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", f.rel, err)
		}
		if string(cur) != string(f.body) {
			return fmt.Errorf("%s is stale; run make generate", f.rel)
		}
	}
	return nil
}

func writeFiles(root string, files []artifact) error {
	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, f.body, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.rel, err)
		}
	}
	return nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}
