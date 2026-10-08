package rest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestShutdownWaitsForDetachedDrain is the process-exit grace: after
// Rebind detaches a server, Shutdown waits for that background drain
// and stops when ctx ends. It already passed on 690d312, where
// Shutdown waited on the WaitGroup. It is a regression guard for the
// channel-based drain tracking, not a red-before-fix test.
func TestShutdownWaitsForDetachedDrain(t *testing.T) {
	t.Run("waits for the in-flight request", func(t *testing.T) {
		srv, release := startHoldServer(t)
		if err := srv.Rebind(""); err != nil {
			t.Fatal(err)
		}
		if srv.Bound() {
			t.Fatal("Rebind(\"\") left the listener bound")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- srv.Shutdown(ctx) }()

		select {
		case err := <-done:
			t.Fatalf("Shutdown returned before the request was released: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		release()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Shutdown: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Shutdown did not return after the request finished")
		}
	})

	t.Run("expired context returns promptly", func(t *testing.T) {
		srv, release := startHoldServer(t)
		defer release()
		if err := srv.Rebind(""); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		err := srv.Shutdown(ctx)
		elapsed := time.Since(start)
		if elapsed > time.Second {
			t.Fatalf("Shutdown with an expired context took %s", elapsed)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Shutdown err %v, want context.Canceled", err)
		}
	})
}

// TestAddrAfterShutdownKeepsBoundAddress pins Addr after a normal
// process Shutdown. Serve binds 127.0.0.1:0 and nothing calls Rebind.
// Shutdown leaves the listener in place, so Bound is false and Addr
// is still the address that was bound. That address is not empty and
// is not Config.Addr or DefaultAddr. Rebind("") is the path that
// clears Addr. This passes on the current code; it pins existing
// behavior.
func TestAddrAfterShutdownKeepsBoundAddress(t *testing.T) {
	svc := bootTestApp(t)
	s, err := New(Config{
		Service:    svc,
		RatePerSec: -1,
		Addr:       DefaultAddr,
	})
	if err != nil {
		t.Fatal(err)
	}
	ln := listenLocal(t)
	bound := ln.Addr().String()
	if bound == "" || bound == s.cfg.Addr || bound == DefaultAddr {
		t.Fatalf("bound address %q is not distinct from Config.Addr or DefaultAddr", bound)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	waitBound(t, s)
	if got := s.Addr(); got != bound {
		t.Fatalf("Addr while bound %q, want %q", got, bound)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after Shutdown")
	}
	if s.Bound() {
		t.Fatal("Bound after Shutdown")
	}
	got := s.Addr()
	if got != bound || got == "" || got == s.cfg.Addr || got == DefaultAddr {
		t.Fatalf("Addr after Shutdown %q, want %q", got, bound)
	}
}

// TestRebindShutdownOverlapNoPanic overlaps Rebind and Shutdown.
// A WaitGroup Add while Wait is still registered panics once the
// counter has hit zero. Rebind and Shutdown do not share a lock, so
// an embedder can hit that window. The panic is unrecovered.
func TestRebindShutdownOverlapNoPanic(t *testing.T) {
	srv, release := startHoldServer(t)
	if err := srv.Rebind(""); err != nil {
		t.Fatal(err)
	}

	// The detached drain keeps the counter above zero. Shutdown with
	// an already-canceled context returns, and on a WaitGroup its Wait
	// stays registered until that counter drops to zero.
	const parked = 64
	var started sync.WaitGroup
	started.Add(parked)
	for i := 0; i < parked; i++ {
		go func() {
			started.Done()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_ = srv.Shutdown(ctx)
		}()
	}
	started.Wait()
	for i := 0; i < 2000; i++ {
		runtime.Gosched()
	}

	stop := make(chan struct{})
	var bg sync.WaitGroup
	for i := 0; i < 4; i++ {
		bg.Add(1)
		go func() {
			defer bg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				_ = srv.Shutdown(ctx)
				cancel()
			}
		}()
	}

	errCh := make(chan error, 4)
	var reb sync.WaitGroup
	for i := 0; i < 4; i++ {
		reb.Add(1)
		go func() {
			defer reb.Done()
			for j := 0; j < 40; j++ {
				if err := srv.Rebind("127.0.0.1:0"); err != nil {
					errCh <- err
					return
				}
				if err := srv.Rebind(""); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	release()
	reb.Wait()
	close(stop)
	bg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// startHoldServer serves on 127.0.0.1:0 and blocks one GET /hold until
// release. release is idempotent.
func startHoldServer(t *testing.T) (*Server, func()) {
	t.Helper()
	entered := make(chan struct{})
	letGo := make(chan struct{})
	var enterOnce sync.Once
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(letGo) }) }

	srv, err := New(Config{
		Service:    bootTestApp(t),
		RatePerSec: -1,
		Mounts: map[string]http.Handler{
			"/hold": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				enterOnce.Do(func() { close(entered) })
				<-letGo
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	waitBound(t, srv)

	reqErr := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Get("http://" + ln.Addr().String() + "/hold")
		if err != nil {
			reqErr <- err
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			reqErr <- fmt.Errorf("status %d", resp.StatusCode)
			return
		}
		reqErr <- nil
	}()

	select {
	case <-entered:
	case err := <-reqErr:
		t.Fatalf("request ended before the handler blocked: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	t.Cleanup(func() {
		release()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		select {
		case <-serveErr:
		case <-time.After(2 * time.Second):
		}
		select {
		case <-reqErr:
		case <-time.After(2 * time.Second):
		}
	})
	return srv, release
}
