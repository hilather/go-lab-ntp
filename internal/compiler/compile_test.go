package compiler

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/testutil"
)

func TestFirstMatchNotLongestPrefix(t *testing.T) {
	raw := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  filters:
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
    - name: tester
      match:
        cidrs: ["10.99.42.20/32"]
      view:
        mode: offset
        offset: -6m
`)
	st, err := config.Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Compile(st, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	ip := netip.MustParseAddr("10.99.42.20")
	f := snap.Match(ip)
	if f == nil || f.Name != "default" {
		t.Fatalf("/32 below /0 must not win, got %#v", f)
	}

	raw2 := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  filters:
    - name: tester
      match:
        cidrs: ["10.99.42.20/32"]
      view:
        mode: offset
        offset: -6m
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`)
	st2, err := config.Load(raw2)
	if err != nil {
		t.Fatal(err)
	}
	snap2, err := Compile(st2, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	f = snap2.Match(ip)
	if f == nil || f.Name != "tester" {
		t.Fatalf("/32 above /0 must win, got %#v", f)
	}
}

func TestIPv4MappedUnmap(t *testing.T) {
	st, err := config.LoadFile(filepath.Join(moduleRoot(t), "testdata/config/valid/full.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Compile(st, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	mapped := netip.MustParseAddr("::ffff:10.99.42.20")
	f := snap.Match(mapped)
	if f == nil || f.Name != "tester-a-kerberos" {
		t.Fatalf("mapped got %#v", f)
	}
}

func TestOverlapWarning(t *testing.T) {
	st, err := config.LoadFile(filepath.Join(moduleRoot(t), "testdata/config/valid/full.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Compile(st, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range snap.Warnings {
		if w.Code == "overlap" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want overlap warning, got %+v", snap.Warnings)
	}
}

func TestMissingKeysFileFailsCompile(t *testing.T) {
	raw := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  ntp:
    symmetricKeys:
      file: /no/such/labntp.keys
  filters:
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`)
	st, err := config.Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(st, CompileOpts{}); err == nil {
		t.Fatal("missing keys file must fail compile")
	}
}

func TestCatchAllRequired(t *testing.T) {
	raw := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  filters:
    - name: only
      match:
        cidrs: ["10.0.0.0/8"]
      view:
        mode: follow-real
`)
	_, err := config.Load(raw)
	if err == nil {
		t.Fatal("missing catch-all must fail validate")
	}
}

func TestRateKeepEpoch(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC))
	raw := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  filters:
    - name: r
      match:
        cidrs: ["10.0.0.1/32"]
      view:
        mode: rate
        rate: 2
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`)
	st, err := config.Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Compile(st, CompileOpts{Clock: clk, Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(10 * time.Second)
	snap2, err := Compile(st, CompileOpts{Clock: clk, Generation: 2, Previous: snap})
	if err != nil {
		t.Fatal(err)
	}
	v1 := snap.Filters[0].View
	v2 := snap2.Filters[0].View
	if !v1.EpochMono.Equal(v2.EpochMono) || !v1.EpochVirtual.Equal(v2.EpochVirtual) {
		t.Fatal("unchanged rate/epoch must keep epoch pair")
	}
}

func TestKeysFileCompile(t *testing.T) {
	root := moduleRoot(t)
	raw := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  ntp:
    symmetricKeys:
      file: ` + filepath.Join(root, "testdata/keys/ntp.keys") + `
  filters:
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`)
	st, err := config.Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Compile(st, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Keys == nil || len(snap.Keys.ByID) != 3 {
		t.Fatalf("keys %+v", snap.Keys)
	}
}

func moduleRoot(t *testing.T) string {
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
			t.Fatal("go.mod")
		}
		dir = parent
	}
}
