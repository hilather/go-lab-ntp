package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRepoDocuments(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(root); err != nil {
		t.Fatal(err)
	}
}

func TestCheckReportsMissingAndBroken(t *testing.T) {
	dir := t.TempDir()
	if err := Check(dir); err == nil {
		t.Fatal("expected missing documents")
	} else if !strings.Contains(err.Error(), "required documents missing") {
		t.Fatalf("error = %v", err)
	}

	for _, rel := range RequiredRootDocs {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# x\n\nSee [missing](no-such-file.md).\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Check(dir); err == nil {
		t.Fatal("expected broken link")
	} else if !strings.Contains(err.Error(), "broken markdown links") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckReportsMissingMetadata(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range RequiredRootDocs {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "# x\n"
		if rel == "docs/01-architecture.md" {
			body = "# System Architecture\n\nNo metadata.\n"
		} else if strings.HasPrefix(rel, "docs/") && strings.HasSuffix(rel, ".md") && !strings.Contains(rel, "adr/") && rel != "docs/implementation-design.md" && rel != "docs/README.md" {
			body = "# x\n\nStatus: Proposed\nOwners: Test\nLast reviewed: 2026-08-30\n"
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Check(dir); err == nil {
		t.Fatal("expected missing metadata")
	} else if !strings.Contains(err.Error(), "documentation metadata missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestFuzzCorporaPresent(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkFuzzCorpora(root); err != nil {
		t.Fatal(err)
	}
}

func TestFuzzCorporaReportsMissing(t *testing.T) {
	dir := t.TempDir()
	if err := checkFuzzCorpora(dir); err == nil {
		t.Fatal("expected missing corpora")
	}
}

func TestSkipFileURL(t *testing.T) {
	if !skipLink("file:///home/brewerm/git/go-lab-dns") {
		t.Fatal("file: links must be skipped")
	}
	if skipLink("docs/01-architecture.md") {
		t.Fatal("relative docs links must be checked")
	}
}

func TestRequiredPhrasesOnFixture(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "x.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkRequiredPhrases(dir); err == nil {
		t.Fatal("expected missing phrases")
	}
}
