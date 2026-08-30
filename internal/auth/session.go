package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

const (
	// CookieName is the REST-only UI session cookie.
	CookieName = "labntp_session"
	// CSRFHeader is required on cookie-authenticated mutations.
	CSRFHeader = "X-LabNTP-CSRF"
	cookiePath = "/"
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
type Session struct {
	ID        string
	TokenID   string
	Role      string
	Scopes    []string
	CreatedAt time.Time
	LastSeen  time.Time
}

// Store is a process-local session table. Cookie values and CSRF secrets
// stay in memory and are never persisted.
type Store struct {
	mu       sync.Mutex
	sessions map[string]*record
	cfg      SessionConfig
	now      func() time.Time
}

type record struct {
	public    Session
	csrf      string
	createdAt time.Time
	lastSeen  time.Time
}

// NewStore builds a session table. Non-positive durations use the defaults.
func NewStore(cfg SessionConfig) *Store {
	def := DefaultSessionConfig()
	if cfg.Idle <= 0 {
		cfg.Idle = def.Idle
	}
	if cfg.Absolute <= 0 {
		cfg.Absolute = def.Absolute
	}
	if cfg.Max <= 0 {
		cfg.Max = def.Max
	}
	return &Store{
		sessions: make(map[string]*record),
		cfg:      cfg,
		now:      time.Now,
	}
}

// SetClock overrides the clock (tests).
func (s *Store) SetClock(now func() time.Time) {
	if s == nil || now == nil {
		return
	}
	s.now = now
}

// Create issues a new session and CSRF secret.
func (s *Store) Create(p Principal) (cookieValue, csrf string, sess Session, err error) {
	if s == nil {
		return "", "", Session{}, domainerr.Internal("session store unavailable")
	}
	cookieValue, err = randomHex(MinTokenBytes)
	if err != nil {
		return "", "", Session{}, err
	}
	csrf, err = randomHex(MinTokenBytes)
	if err != nil {
		return "", "", Session{}, err
	}
	publicID, err := randomHex(16)
	if err != nil {
		return "", "", Session{}, err
	}
	now := s.now()
	rec := &record{
		public: Session{
			ID:        publicID,
			TokenID:   p.ID,
			Role:      p.Role,
			Scopes:    append([]string(nil), p.Scopes...),
			CreatedAt: now,
			LastSeen:  now,
		},
		csrf:      csrf,
		createdAt: now,
		lastSeen:  now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(now)
	if len(s.sessions) >= s.cfg.Max {
		s.evictOldestLocked()
	}
	s.sessions[cookieValue] = rec
	return cookieValue, csrf, rec.public, nil
}

// Lookup returns the session for cookieValue and touches LastSeen.
func (s *Store) Lookup(cookieValue string) (Session, string, bool) {
	if s == nil || cookieValue == "" {
		return Session{}, "", false
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.sessions[cookieValue]
	if !ok || s.expiredLocked(rec, now) {
		if ok {
			delete(s.sessions, cookieValue)
		}
		return Session{}, "", false
	}
	rec.lastSeen = now
	rec.public.LastSeen = now
	return rec.public, rec.csrf, true
}

// Delete removes one cookie session.
func (s *Store) Delete(cookieValue string) {
	if s == nil || cookieValue == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, cookieValue)
	s.mu.Unlock()
}

// Clear drops every session (reset / token reload when identity changes).
func (s *Store) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.sessions = make(map[string]*record)
	s.mu.Unlock()
}

// ValidCSRF compares the presented header to the session CSRF secret.
func (s *Store) ValidCSRF(cookieValue, presented string) bool {
	if s == nil || cookieValue == "" || presented == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.sessions[cookieValue]
	if !ok || s.expiredLocked(rec, s.now()) {
		if ok {
			delete(s.sessions, cookieValue)
		}
		return false
	}
	return EqualDigest(DigestSecret([]byte(rec.csrf)), DigestSecret([]byte(presented)))
}

// MaxAge is the cookie Max-Age (absolute TTL).
func (s *Store) MaxAge() int {
	if s == nil {
		return int(DefaultSessionConfig().Absolute.Seconds())
	}
	return int(s.cfg.Absolute.Seconds())
}

// ExpiresAt is the earlier of idle and absolute expiry.
func (s *Store) ExpiresAt(sess Session) time.Time {
	if s == nil {
		return time.Time{}
	}
	idle := sess.LastSeen.Add(s.cfg.Idle)
	abs := sess.CreatedAt.Add(s.cfg.Absolute)
	if idle.Before(abs) {
		return idle
	}
	return abs
}

func (s *Store) expiredLocked(rec *record, now time.Time) bool {
	if now.Sub(rec.lastSeen) > s.cfg.Idle {
		return true
	}
	return now.Sub(rec.createdAt) > s.cfg.Absolute
}

func (s *Store) expireLocked(now time.Time) {
	for k, rec := range s.sessions {
		if s.expiredLocked(rec, now) {
			delete(s.sessions, k)
		}
	}
}

func (s *Store) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	first := true
	for k, rec := range s.sessions {
		if first || rec.lastSeen.Before(oldest) {
			oldestKey = k
			oldest = rec.lastSeen
			first = false
		}
	}
	if oldestKey != "" {
		delete(s.sessions, oldestKey)
	}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", domainerr.Internal("session material unavailable")
	}
	return hex.EncodeToString(b), nil
}

// NewSessionCookie builds the browser cookie. Secure iff management TLS.
func NewSessionCookie(value string, secure bool, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     cookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearSessionCookie expires the UI cookie.
func ClearSessionCookie(secure bool) *http.Cookie {
	c := NewSessionCookie("", secure, -1)
	c.Expires = time.Unix(0, 0).UTC()
	return c
}

// CookieSecure is true when the request is TLS or the server requires Secure.
func CookieSecure(r *http.Request, force bool) bool {
	if force {
		return true
	}
	return r != nil && r.TLS != nil
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
