package ntpserver

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/compiler"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/ntpwire"
	"github.com/hilather/go-lab-ntp/internal/querylog"
	"github.com/hilather/go-lab-ntp/internal/snapshot"
)

func TestServeAndOversize(t *testing.T) {
	srv, addr := startServer(t, testdata(t, "config/valid/defaults.yaml"), "")
	defer shutdown(t, srv)

	c, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))

	req := ntpwire.Encode(ntpwire.Packet{VN: 4, Mode: ntpwire.ModeClient, XmtTime: ntpwire.FromTime(time.Now())})
	if _, err := c.Write(req); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 128)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := ntpwire.Parse(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if rep.Mode != ntpwire.ModeServer || rep.VN != 4 {
		t.Fatalf("%+v", rep)
	}

	big := bytes.Repeat([]byte{1}, ntpwire.MaxUDPSize+1)
	if _, err := c.Write(big); err != nil {
		t.Fatal(err)
	}
	_ = c.SetDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := c.Read(buf); err == nil {
		t.Fatal("oversize must drop")
	}
	if srv.Oversize.Load() < 1 {
		t.Fatal("oversize metric")
	}
}

func TestZeroXmitDrop(t *testing.T) {
	srv, addr := startServer(t, testdata(t, "config/valid/defaults.yaml"), "")
	defer shutdown(t, srv)
	c, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	req := ntpwire.Encode(ntpwire.Packet{VN: 4, Mode: ntpwire.ModeClient})
	_, _ = c.Write(req)
	_ = c.SetDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 64)
	if _, err := c.Read(buf); err == nil {
		t.Fatal("zero xmit must drop")
	}
}

func TestMACRoundTrip(t *testing.T) {
	root := moduleRoot(t)
	keys := filepath.Join(root, "testdata/keys/ntp.keys")
	raw := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  ntp:
    symmetricKeys:
      file: ` + keys + `
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
	snap, err := compiler.Compile(st, compiler.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	log := querylog.New(8)
	srv, err := New(Config{Addr: "127.0.0.1:0", Store: store, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, srv)

	vec, err := os.ReadFile(filepath.Join(root, "testdata/packets/mac-md5.raw"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.Dial("udp", srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write(vec); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 128)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n <= ntpwire.PacketSize {
		t.Fatal("expected MAC trailer on reply")
	}
	if !ntpwire.VerifyMAC("MD5", []byte("labdev-md5-key"), buf[:n]) {
		t.Fatal("reply MAC")
	}

	// unsigned dropped when keys on
	plain := ntpwire.Encode(ntpwire.Packet{VN: 4, Mode: ntpwire.ModeClient, XmtTime: ntpwire.FromTime(time.Now())})
	_, _ = c.Write(plain)
	_ = c.SetDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := c.Read(buf); err == nil {
		t.Fatal("unsigned must drop when keys on")
	}
}

func TestRebindUnchangedIdentity(t *testing.T) {
	srv, _ := startServer(t, testdata(t, "config/valid/defaults.yaml"), "")
	defer shutdown(t, srv)
	old := srv.PacketConn()
	if err := srv.Rebind(srv.bindAddr); err != nil {
		t.Fatal(err)
	}
	if srv.PacketConn() != old {
		t.Fatal("unchanged listen must keep PacketConn")
	}
}

func TestKoDRate(t *testing.T) {
	raw := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  ntp:
    restrict:
      default: limited
      kod: true
    admission:
      maxPacketsPerSec: 1000
      maxPacketsPerIP: 1000
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
	snap, err := compiler.Compile(st, compiler.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	srv, err := New(Config{Addr: "127.0.0.1:0", Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer shutdown(t, srv)
	c, err := net.Dial("udp", srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	req := ntpwire.Encode(ntpwire.Packet{VN: 4, Mode: ntpwire.ModeClient, XmtTime: ntpwire.FromTime(time.Now())})
	_, _ = c.Write(req)
	buf := make([]byte, 64)
	if _, err := c.Read(buf); err != nil {
		t.Fatal(err)
	}
	_, _ = c.Write(req)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := ntpwire.Parse(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if rep.Stratum != 0 || rep.RefID != ntpwire.KissRATE {
		t.Fatalf("want KoD RATE, got %+v", rep)
	}
}

func TestRebindFailKeepsOld(t *testing.T) {
	srv, _ := startServer(t, testdata(t, "config/valid/defaults.yaml"), "")
	defer shutdown(t, srv)
	old := srv.PacketConn()
	hold, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hold.Close() }()
	if err := srv.Rebind(hold.LocalAddr().String()); err == nil {
		t.Fatal("expected bind failure")
	}
	if srv.PacketConn() != old {
		t.Fatal("failed rebind must keep old conn")
	}
}

func TestRebindNewFirst(t *testing.T) {
	srv, _ := startServer(t, testdata(t, "config/valid/defaults.yaml"), "")
	defer shutdown(t, srv)
	old := srv.PacketConn()
	if err := srv.Rebind("[::1]:0"); err != nil {
		t.Fatal(err)
	}
	if srv.PacketConn() == old {
		t.Fatal("changed listen must rebind")
	}
	if srv.Addr() == nil {
		t.Fatal("new addr")
	}
}

func TestDualStackClients(t *testing.T) {
	srv, _ := startServer(t, testdata(t, "config/valid/defaults.yaml"), ":0")
	defer shutdown(t, srv)
	host, port, err := net.SplitHostPort(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = host
	try := func(network, dest string) {
		t.Helper()
		c, err := net.Dial(network, dest)
		if err != nil {
			t.Skipf("%s: %v", network, err)
		}
		defer func() { _ = c.Close() }()
		_ = c.SetDeadline(time.Now().Add(2 * time.Second))
		req := ntpwire.Encode(ntpwire.Packet{VN: 4, Mode: ntpwire.ModeClient, XmtTime: ntpwire.FromTime(time.Now())})
		if _, err := c.Write(req); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 64)
		if _, err := c.Read(buf); err != nil {
			t.Fatalf("%s read: %v", network, err)
		}
	}
	try("udp4", net.JoinHostPort("127.0.0.1", port))
	try("udp6", net.JoinHostPort("::1", port))
}

func startServer(t *testing.T, cfgPath, listen string) (*Server, string) {
	t.Helper()
	st, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := compiler.Compile(st, compiler.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	store := snapshot.NewStore()
	store.InstallBootstrap(snap)
	if listen == "" {
		listen = "127.0.0.1:0"
	}
	srv, err := New(Config{Addr: listen, Store: store, Log: querylog.New(16)})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	return srv, srv.Addr().String()
}

func shutdown(t *testing.T, s *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.Shutdown(ctx)
}

func testdata(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "testdata", filepath.FromSlash(rel))
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
