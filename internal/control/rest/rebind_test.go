package rest

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"
)

func TestRebindLeavesOldListenerWhenPortIsTaken(t *testing.T) {
	s, _ := newTestServer(t)
	ln := listenLocal(t)
	old := ln.Addr().String()
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	waitBound(t, s)

	hold := listenLocal(t)
	t.Cleanup(func() { _ = hold.Close() })
	if err := s.Rebind(hold.Addr().String()); err == nil {
		t.Fatal("expected bind error")
	}
	if !s.Bound() || s.Addr() != old {
		t.Fatalf("after failed rebind bound=%v addr=%s want %s", s.Bound(), s.Addr(), old)
	}
	if got := getLive(t, old); got != http.StatusOK {
		t.Fatalf("old listener status %d", got)
	}

	if err := s.Rebind(old); err != nil {
		t.Fatal(err)
	}
	if !s.Bound() || s.Addr() != old {
		t.Fatalf("same address bound=%v addr=%s", s.Bound(), s.Addr())
	}

	nextLn := listenLocal(t)
	next := nextLn.Addr().String()
	if err := nextLn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Rebind(next); err != nil {
		t.Fatal(err)
	}
	if !s.Bound() || s.Addr() != next {
		t.Fatalf("after move bound=%v addr=%s want %s", s.Bound(), s.Addr(), next)
	}
	if got := getLive(t, next); got != http.StatusOK {
		t.Fatalf("new listener status %d", got)
	}
	if got := dialState(old); got != "refused" {
		t.Fatalf("old dial %s", got)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("original Serve still running")
	}

	if err := s.Rebind(""); err != nil {
		t.Fatal(err)
	}
	if s.Bound() {
		t.Fatal("bound after off")
	}
	if got := dialState(next); got != "refused" {
		t.Fatalf("off dial %s", got)
	}

	againLn := listenLocal(t)
	again := againLn.Addr().String()
	if err := againLn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Rebind(again); err != nil {
		t.Fatal(err)
	}
	if !s.Bound() || s.Addr() != again {
		t.Fatalf("rebind after off bound=%v addr=%s want %s", s.Bound(), s.Addr(), again)
	}
	if got := getLive(t, again); got != http.StatusOK {
		t.Fatalf("listener after off status %d", got)
	}
}

func listenLocal(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func waitBound(t *testing.T, s *Server) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.Bound() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("server not bound")
}

func getLive(t *testing.T, addr string) int {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/v1/health/live")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func dialState(addr string) string {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err == nil {
		_ = conn.Close()
		return "connected"
	}
	if isConnRefused(err) {
		return "refused"
	}
	return err.Error()
}

func isConnRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}
