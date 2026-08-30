package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestCommittedDistIsProduction(t *testing.T) {
	raw, err := fs.ReadFile(Files(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, "UI assets were not copied") {
		t.Fatal("Files() served the stub page; commit a Vite tree in internal/web/dist")
	}
	if !strings.Contains(body, "LabNTP") && !strings.Contains(body, `id="root"`) {
		t.Fatalf("committed dist missing LabNTP title or #root: %s", body)
	}
}
