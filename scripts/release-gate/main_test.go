package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateNotes(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.md")
	body := "# LabNTP 1.0.0-rc.2\n\n## Highlights\n\nx\n\n## Added\n\nx\n\n## Residual\n\nx\n\n## Deployment and operations\n\nx\n\n## CI and release evidence\n\nx\n"
	if err := os.WriteFile(ok, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateNotes(ok); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(bad, []byte("# x\nTODO\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateNotes(bad); err == nil {
		t.Fatal("expected missing headings")
	}
}
