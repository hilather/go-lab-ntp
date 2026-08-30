package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/querylog"
)

func TestResetRereadsAndNeverWrites(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "labntp.yaml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := Boot(context.Background(), Options{BootstrapPath: path, QueryLog: querylog.New(8)})
	if err != nil {
		t.Fatal(err)
	}
	a := actor()
	_, err = svc.Apply(context.Background(), a, ChangeIn{
		ExpectedRevision: svc.Active().Revision,
		Operations: []model.Operation{{
			Op:       model.OpReplaceQueryLog,
			QueryLog: &model.QueryLogSpec{Size: 8},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.QueryLog().TryInsert(querylog.Entry{Filter: "x"})
	if len(svc.QueryLog().List()) == 0 {
		t.Fatal("query log empty before reset")
	}
	res, err := svc.Reset(context.Background(), a, ResetIn{Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied {
		t.Fatal("reset")
	}
	if len(svc.QueryLog().List()) != 0 {
		t.Fatal("reset must wipe query log")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("reset must never write bootstrap")
	}
}

func TestResetUnchangedListenDoesNotRebind(t *testing.T) {
	svc, snap := mustBoot(t)
	calls := 0
	svc.SetNTPRebind(func(addr string) error {
		calls++
		return nil
	})
	svc.ntpOverride = snap.NTPAddress
	_, err := svc.Reset(context.Background(), actor(), ResetIn{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("unchanged listen rebind calls=%d", calls)
	}
}

func TestResetChangedYAMLListenRebinds(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "labntp.yaml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := Boot(context.Background(), Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	got := ""
	svc.SetNTPRebind(func(addr string) error {
		got = addr
		return nil
	})
	rewritten := strings.Replace(string(src), `address: ":123"`, `address: ":1199"`, 1)
	if rewritten == string(src) {
		body := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: defaults
spec:
  listeners:
    ntp:
      address: ":1199"
  filters:
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.WriteFile(path, []byte(rewritten), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err = svc.Reset(context.Background(), actor(), ResetIn{})
	if err != nil {
		t.Fatal(err)
	}
	if got != ":1199" {
		t.Fatalf("rebind addr %q", got)
	}
}
