package auth

import (
	"net/http"
	"time"
)

const (
	// CookieName is the REST-only UI session cookie.
	CookieName = "labntp_session"
	// CSRFHeader is required on cookie-authenticated mutations.
	CSRFHeader = "X-LabNTP-CSRF"
)

// SessionConfig sizes the process-local session table.
type SessionConfig struct {
	Idle     time.Duration
	Absolute time.Duration
	Max      int
}

// DefaultSessionConfig is TTL 12h, idle 4h, max 64.
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		Idle:     4 * time.Hour,
		Absolute: 12 * time.Hour,
		Max:      64,
	}
}

// Session is the public, non-secret view of an in-memory session.
// kit is the controlkit session from the same Create or Lookup, so
// ExpiresAt can ask the kit without recomputing the deadline.
type Session struct {
	ID        string
	TokenID   string
	Role      string
	Scopes    []string
	CreatedAt time.Time
	LastSeen  time.Time
	kit       kitSession
}

// Sessions is a process-local session table. Cookie values and CSRF secrets
// stay in memory and are never persisted.
type Sessions struct {
	k *kitStore
}

// UnsafeMethod is a cookie-CSRF-protected mutation.
func UnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// PrincipalFromSession copies the token principal off a cookie session.
func PrincipalFromSession(sess Session) Principal {
	return Principal{
		ID:     sess.TokenID,
		Class:  ClassToken,
		Role:   sess.Role,
		Scopes: append([]string(nil), sess.Scopes...),
	}
}
