package auth

import (
	"net"
	"net/url"
	"strings"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

// CheckOrigin implements the LabDNS wording: a present non-loopback Origin
// is rejected unless it is on allowedOrigins. Missing Origin is allowed.
// Only http/https Origins are accepted (file:// is denied even on loopback).
func CheckOrigin(origin string, allowlist []string) error {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return domainerr.Forbidden("origin is not allowed")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return domainerr.Forbidden("origin is not allowed")
	}
	host := u.Hostname()
	if isLoopbackHost(host) {
		return nil
	}
	for _, allowed := range allowlist {
		if originMatches(origin, allowed) {
			return nil
		}
	}
	return domainerr.Forbidden("origin is not allowed")
}

func originMatches(got, want string) bool {
	got = strings.TrimRight(strings.TrimSpace(got), "/")
	want = strings.TrimRight(strings.TrimSpace(want), "/")
	return strings.EqualFold(got, want)
}

func isLoopbackHost(host string) bool {
	h := strings.TrimSpace(host)
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
