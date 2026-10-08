package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/querylog"
)

// A failed management rebind must not leave the NTP listener on the candidate
// address while the active snapshot is still the previous generation.
func TestFailedResetDoesNotKeepNTPRebind(t *testing.T) {
	dir := t.TempDir()
	write := func(ntp, mgmt string) string {
		body := fmt.Sprintf(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: rebind
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
`, ntp, mgmt)
		path := filepath.Join(dir, "labntp.yaml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := write("127.0.0.1:1123", "127.0.0.1:18088")
	svc, err := Boot(context.Background(), Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	before := svc.Active().Revision
	if !svc.QueryLog().TryInsert(querylog.Entry{Filter: "keep"}) {
		t.Fatal("query log insert")
	}
	liveNTP := "127.0.0.1:1123"
	svc.SetNTPRebind(func(addr string) error {
		liveNTP = addr
		return nil
	})
	svc.SetHTTPRebind(func(addr string) error {
		return fmt.Errorf("management bind %s failed", addr)
	})
	write("127.0.0.1:1124", "127.0.0.1:18089")
	_, err = svc.Reset(context.Background(), actor(), ResetIn{Reason: "move"})
	if err == nil {
		t.Fatal("expected reset to fail when management rebind fails")
	}
	if svc.Active().Revision != before {
		t.Fatalf("failed reset swapped snapshot to %s", svc.Active().Revision)
	}
	if liveNTP != "127.0.0.1:1123" {
		t.Fatalf("failed reset left NTP listener on %s", liveNTP)
	}
	found := false
	for _, e := range svc.QueryLog().List() {
		if e.Filter == "keep" {
			found = true
		}
	}
	if !found {
		t.Fatal("failed reset wiped the query log")
	}
}
