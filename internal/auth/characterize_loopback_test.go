package auth

import (
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestCharacterizeDevLoopbackRemoteAddr(t *testing.T) {
	v, err := FromSpec(model.AuthSpec{Mode: model.MgmtAuthDevLoopbackUnauth})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		addr string
		ok   bool
	}{
		{addr: "127.0.0.1:1", ok: true},
		{addr: "[::1]:1", ok: true},
		{addr: "[::ffff:127.0.0.1]:1", ok: true},
		{addr: "localhost:1", ok: true},
		{addr: "10.0.0.1:1", ok: false},
		{addr: "[2001:db8::1]:1", ok: false},
		{addr: "", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			p, err := v.Authenticate(Request{RemoteAddr: tc.addr})
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				if p.ID != "loopback" || p.Class != ClassLoopback || p.Role != model.RoleAdministrator {
					t.Fatalf("%+v", p)
				}
				want := DefaultScopes(model.RoleAdministrator)
				if !sameScopes(p.Scopes, want) {
					t.Fatalf("scopes %v", p.Scopes)
				}
				return
			}
			de, ok := domainerr.As(err)
			if !ok || de.Code != domainerr.CodeUnauthenticated || de.Message != "authentication required" {
				t.Fatalf("addr %q err=%v", tc.addr, err)
			}
		})
	}
	_, err = v.Authenticate(Request{Authorization: "Bearer nope", RemoteAddr: "127.0.0.1:1"})
	de, ok := domainerr.As(err)
	if !ok || de.Message != "authentication required" {
		t.Fatalf("presented header must not fall back to loopback: %v", err)
	}
}
