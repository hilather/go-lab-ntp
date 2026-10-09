package auth

import (
	"errors"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func TestCharacterizeOriginText(t *testing.T) {
	allow := []string{"https://lab.example"}
	cases := []struct {
		name   string
		origin string
		ok     bool
	}{
		{name: "missing", origin: "", ok: true},
		{name: "whitespace", origin: "  ", ok: true},
		{name: "allowed", origin: "https://lab.example", ok: true},
		{name: "allowed trailing slash", origin: "https://lab.example/", ok: true},
		{name: "allowed case", origin: "HTTPS://LAB.EXAMPLE", ok: true},
		{name: "foreign", origin: "https://evil.example", ok: false},
		{name: "file", origin: "file:///tmp/x", ok: false},
		{name: "loopback host", origin: "http://127.0.0.1:8080", ok: true},
		{name: "loopback name", origin: "http://localhost", ok: true},
		{name: "loopback v6", origin: "http://[::1]", ok: true},
		{name: "localhost case sensitive", origin: "http://LocalHost", ok: false},
		{name: "private not listed", origin: "http://10.1.2.3", ok: false},
		{name: "empty host", origin: "http://", ok: false},
		{name: "unparsed", origin: "://bad", ok: false},
		{name: "not a url", origin: "not a url", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckOrigin(tc.origin, allow)
			if tc.ok {
				if err != nil {
					t.Fatalf("origin %q: %v", tc.origin, err)
				}
				return
			}
			assertForbidden(t, err, "origin is not allowed")
		})
	}
}

func assertForbidden(t *testing.T, err error, message string) {
	t.Helper()
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeForbidden || de.Message != message {
		t.Fatalf("err=%v want forbidden %q", err, message)
	}
	if !errors.Is(err, domainerr.Forbidden(message)) {
		t.Fatalf("errors.Is: %v", err)
	}
}
