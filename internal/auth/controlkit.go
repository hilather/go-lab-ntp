package auth

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hilather/go-lab-controlkit/authn"
	kitorigin "github.com/hilather/go-lab-controlkit/origin"
	"github.com/hilather/go-lab-controlkit/scope"
	kitsession "github.com/hilather/go-lab-controlkit/session"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// MinTokenBytes is 256 bits of secret material.
const MinTokenBytes = 32

// TokenSource loads tokens. Tests wrap it to count secret-file opens.
type TokenSource = authn.TokenSource

// RawToken is one token before it is digested.
type RawToken = authn.RawToken

// FileResult is one candidate path a token source opened.
type FileResult = authn.FileResult

type kitStore = kitsession.Store
type kitSession = kitsession.Session

// Request is one authentication attempt. Adapters fill it from the HTTP
// request; X-Forwarded-For is never consulted.
type Request struct {
	Authorization string
	RemoteAddr    string
}

// Verifier is the process-local token index. There is no HTTP Basic.
type Verifier struct {
	mu sync.Mutex
	k  *authn.Verifier
	m  *authn.Material
}

// FromSpec compiles spec.auth. Missing secret files fail closed.
func FromSpec(spec model.AuthSpec) (*Verifier, error) {
	return FromSpecWith(spec, nil)
}

// FromSpecWith compiles spec.auth. wrap, when non-nil, sees the per-token
// file source before it is loaded. Production passes nil.
//
// Checks follow the pre-facade FromSpec order and stop at the first failure:
// unknown mode, then each token's empty id, duplicate id, secret read,
// length, duplicate digest, and unknown role. A later secret file is not
// opened after that return. A role that differs from its trimmed form is
// rejected before that token's file is opened and before Load, which trims
// the role and would accept it (plan NB6).
//
// Each secret is read once, through wrap. Load then compiles those bytes
// from memory so a second pass does not open the files again.
func FromSpecWith(spec model.AuthSpec, wrap func(TokenSource) TokenSource) (*Verifier, error) {
	mode, modeText := parseSpecMode(spec.Mode)
	if mode == authn.ModeUnknown {
		return nil, invalidSpec("unknown auth mode", "spec.auth.mode", "invalid_value", "unknown auth mode")
	}

	src := tokenFiles(spec)
	if wrap != nil {
		src = wrap(src)
	}
	hold := &rawHold{}
	defer hold.wipe()

	seen := make(map[string]struct{}, len(spec.Tokens))
	var compiled *authn.Material
	for i, tok := range spec.Tokens {
		id := strings.TrimSpace(tok.ID)
		if id == "" {
			return nil, invalidSpec("token id is required", tokenPath(i, "id"), "empty_id", "token id is required")
		}
		if _, dup := seen[id]; dup {
			return nil, invalidSpec("duplicate token id", tokenPath(i, "id"), "duplicate_id", "duplicate token id")
		}
		// NB6. Load trims the role before Expand, so a padded role would
		// compile. Reject it before this token's file is opened.
		if tok.Role != strings.TrimSpace(tok.Role) {
			return nil, invalidSpec("unknown role", tokenPath(i, "role"), "invalid_value", "role must be viewer, operator, or administrator")
		}
		got, _, err := src.Read()
		if err != nil || len(got) != 1 {
			wipeRaw(got)
			return nil, invalidSpec("token secret is unavailable", tokenPath(i, "secretFile"), "unresolved_reference", "token secret file does not resolve")
		}
		hold.tokens = append(hold.tokens, got[0])
		seen[id] = struct{}{}
		// Length, duplicate digest, and unknown role run here, on the tokens
		// read so far, before the next file is opened.
		m, lerr := authn.Load(ntpAuthConfig(mode, modeText, authn.Memory(hold.tokens)))
		if lerr != nil {
			return nil, mapLoadErr(lerr)
		}
		compiled = m
	}
	if compiled == nil {
		if _, _, err := src.Read(); err != nil {
			return nil, mapLoadErr(err)
		}
		m, err := authn.Load(ntpAuthConfig(mode, modeText, authn.Memory(nil)))
		if err != nil {
			return nil, mapLoadErr(err)
		}
		compiled = m
	}
	return newVerifier(compiled)
}

// Static builds a bearer verifier from an in-memory secret.
// It is a test-only helper. secret must be at least MinTokenBytes (32).
// role must be empty or a known role (viewer, operator, or administrator);
// a shorter secret or any other role panics.
func Static(secret, id, role string) *Verifier {
	if id == "" {
		id = "admin"
	}
	m, err := authn.Load(ntpAuthConfig(authn.ModeBearer, model.MgmtAuthBearer, authn.Memory([]authn.RawToken{{
		ID:     id,
		Role:   role,
		Secret: authn.NewSecret([]byte(secret)),
	}})))
	if err != nil {
		panic("auth.Static: " + err.Error())
	}
	v, err := newVerifier(m)
	if err != nil {
		panic("auth.Static: " + err.Error())
	}
	return v
}

func newVerifier(m *authn.Material) (*Verifier, error) {
	k, err := authn.NewVerifier(m)
	if err != nil {
		return nil, mapLoadErr(err)
	}
	return &Verifier{k: k, m: m}, nil
}

// OnIdentityChange registers a hook fired after Replace when identity changed.
func (v *Verifier) OnIdentityChange(fn func()) {
	if v == nil || v.k == nil || fn == nil {
		return
	}
	v.k.OnIdentityChange(fn)
}

// Replace swaps the compiled index in place so REST and MCP share one pointer.
func (v *Verifier) Replace(next *Verifier) {
	if v == nil || next == nil || v.k == nil {
		return
	}
	next.mu.Lock()
	m := next.m
	next.mu.Unlock()
	if m == nil {
		return
	}
	v.mu.Lock()
	v.m = m
	v.mu.Unlock()
	v.k.Swap(m)
}

// Equivalent reports whether the compiled identity matches.
func (v *Verifier) Equivalent(other *Verifier) bool {
	if v == nil || other == nil {
		return v == other
	}
	v.mu.Lock()
	a := v.m
	v.mu.Unlock()
	other.mu.Lock()
	b := other.m
	other.mu.Unlock()
	if a == nil || b == nil {
		return a == b
	}
	return a.Equivalent(b)
}

// Mode is the compiled auth mode.
func (v *Verifier) Mode() string {
	if v == nil || v.k == nil {
		return ""
	}
	return v.k.Mode().String()
}

// TokenCount is the number of compiled bearer principals.
func (v *Verifier) TokenCount() int {
	if v == nil || v.k == nil {
		return 0
	}
	return v.k.TokenCount()
}

// RequireListen refuses a management bind that would be allow-all.
func (v *Verifier) RequireListen() error {
	if v == nil || v.k == nil {
		return fmt.Errorf("management bind requires a verifier")
	}
	v.mu.Lock()
	m := v.m
	v.mu.Unlock()
	if err := authn.BearerNeedsToken(true)(m); err != nil {
		return fmt.Errorf("%s", err.Error())
	}
	return nil
}

// WWWAuthenticate is the 401 challenge list. There is no Basic.
func WWWAuthenticate() []string {
	return authn.Challenge("labntp", false)
}

// Authenticate verifies Authorization. A missing header is unauthenticated
// unless mode is dev-loopback-unauth and RemoteAddr is loopback.
func (v *Verifier) Authenticate(in Request) (Principal, error) {
	if v == nil || v.k == nil {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	p, err := v.k.Authenticate(authn.Request{
		Authorization: in.Authorization,
		RemoteAddr:    in.RemoteAddr,
	})
	if err != nil {
		return Principal{}, mapUnauth(err)
	}
	return principalFromKit(p), nil
}

// AuthenticateBearer looks up a raw token secret (mcp-stdio --token-file).
func (v *Verifier) AuthenticateBearer(secret string) (Principal, error) {
	if v == nil || v.k == nil {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	p, err := v.k.AuthenticateBearer([]byte(secret))
	if err != nil {
		return Principal{}, mapUnauth(err)
	}
	return principalFromKit(p), nil
}

// CheckOrigin implements the LabDNS wording: a present non-loopback Origin
// is rejected unless it is on allowedOrigins. Missing Origin is allowed.
// Only http/https Origins are accepted (file:// is denied even on loopback).
func CheckOrigin(origin string, allowlist []string) error {
	err := kitorigin.Check(origin, allowlist, kitorigin.Policy{
		Match:              kitorigin.FoldTrimSlash,
		HostParse:          kitorigin.URLParse,
		LocalhostFold:      false,
		ListUnionsLoopback: true,
		ZonedLoopback:      false,
		Sentinels:          nil,
	})
	if err != nil {
		return domainerr.Forbidden("origin is not allowed")
	}
	return nil
}

// ReadTokenFile reads the raw bytes of a token file. Line selection stays
// with the caller; a comment-only file is returned as raw bytes.
func ReadTokenFile(path string) ([]byte, error) {
	return authn.ReadFile(path, fileOpts())
}

// NewStore builds a session table. Non-positive durations use the defaults.
func NewStore(cfg SessionConfig) *Sessions {
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
	k, err := kitsession.New(kitsession.Config{
		CookieName:  CookieName,
		CSRFHeader:  CSRFHeader,
		Idle:        cfg.Idle,
		Absolute:    cfg.Absolute,
		Max:         cfg.Max,
		AtCap:       kitsession.EvictOldest,
		IDShape:     kitsession.SeparateCookieSecret,
		CSRFCompare: kitsession.DigestConstantTime,
	})
	if err != nil {
		panic("session.New: " + err.Error())
	}
	return &Sessions{k: k}
}

// SetClock overrides the clock (tests). Nil leaves the current clock in place.
func (s *Sessions) SetClock(now func() time.Time) {
	if s == nil || s.k == nil || now == nil {
		return
	}
	s.k.SetNow(now)
}

// Create issues a new session and CSRF secret.
func (s *Sessions) Create(p Principal) (cookieValue, csrf string, sess Session, err error) {
	if s == nil || s.k == nil {
		return "", "", Session{}, domainerr.Internal("session store unavailable")
	}
	issued, err := s.k.Create(scope.Principal{
		ID:     p.ID,
		Class:  ClassToken,
		Role:   p.Role,
		Scopes: append([]string(nil), p.Scopes...),
	})
	if err != nil {
		return "", "", Session{}, mapSessionMint(err)
	}
	return issued.Cookie, issued.CSRF, sessionFromKit(issued.Session), nil
}

// Lookup returns the session for cookieValue and touches LastSeen.
func (s *Sessions) Lookup(cookieValue string) (Session, string, bool) {
	if s == nil || s.k == nil || cookieValue == "" {
		return Session{}, "", false
	}
	ks, ok := s.k.Lookup(cookieValue)
	if !ok {
		return Session{}, "", false
	}
	csrf, ok := s.k.CSRF(cookieValue)
	if !ok {
		return Session{}, "", false
	}
	return sessionFromKit(ks), csrf, true
}

// Delete removes one cookie session.
func (s *Sessions) Delete(cookieValue string) {
	if s == nil || s.k == nil || cookieValue == "" {
		return
	}
	s.k.Delete(cookieValue)
}

// Clear drops every session (reset / token reload when identity changes).
func (s *Sessions) Clear() {
	if s == nil || s.k == nil {
		return
	}
	s.k.Clear()
}

// ValidCSRF compares the presented header to the session CSRF secret.
func (s *Sessions) ValidCSRF(cookieValue, presented string) bool {
	if s == nil || s.k == nil || cookieValue == "" || presented == "" {
		return false
	}
	return s.k.ValidCSRF(cookieValue, presented)
}

// MaxAge is the cookie Max-Age (absolute TTL).
func (s *Sessions) MaxAge() int {
	if s == nil || s.k == nil {
		return int(DefaultSessionConfig().Absolute.Seconds())
	}
	return s.k.MaxAge()
}

// ExpiresAt is the earlier of idle and absolute expiry.
func (s *Sessions) ExpiresAt(sess Session) time.Time {
	if s == nil || s.k == nil {
		return time.Time{}
	}
	return s.k.ExpiresAt(sess.kit)
}

// NewSessionCookie builds the browser cookie. Secure iff management TLS.
func NewSessionCookie(value string, secure bool, maxAge int) *http.Cookie {
	return kitsession.SessionCookie(CookieName, value, secure, maxAge)
}

// ClearSessionCookie expires the UI cookie.
func ClearSessionCookie(secure bool) *http.Cookie {
	return kitsession.ClearCookie(CookieName, secure)
}

// CookieSecure is true when the request is TLS or the server requires Secure.
func CookieSecure(r *http.Request, force bool) bool {
	return kitsession.CookieSecure(r, force)
}

func sessionFromKit(ks kitSession) Session {
	return Session{
		ID:        ks.ID,
		TokenID:   ks.Principal.ID,
		Role:      ks.Principal.Role,
		Scopes:    append([]string(nil), ks.Principal.Scopes...),
		CreatedAt: ks.CreatedAt,
		LastSeen:  ks.LastSeen,
		kit:       ks,
	}
}

func principalFromKit(p scope.Principal) Principal {
	return Principal{
		ID:     p.ID,
		Class:  p.Class,
		Role:   p.Role,
		Scopes: append([]string(nil), p.Scopes...),
	}
}

func parseSpecMode(specMode string) (authn.Mode, string) {
	text := strings.TrimSpace(specMode)
	switch text {
	case "", model.MgmtAuthBearer:
		if text == "" {
			text = model.MgmtAuthBearer
		}
		return authn.ModeBearer, text
	case model.MgmtAuthDevLoopbackUnauth:
		return authn.ModeDevLoopbackUnauth, text
	default:
		return authn.ModeUnknown, text
	}
}

func ntpAuthConfig(mode authn.Mode, modeText string, src TokenSource) authn.Config {
	return authn.Config{
		Mode:                mode,
		ModeText:            modeText,
		Source:              src,
		Duplicates:          authn.RejectDuplicateValue,
		MinSecretBytes:      MinTokenBytes,
		WarnBelowBytes:      0,
		Accept:              nil,
		PathPrefix:          "spec.auth",
		Roles:               ntpScopeTable(),
		RejectEmptyRole:     false,
		RejectBlankTokens:   false,
		LocalhostIsLoopback: true,
		ManagementBound:     false,
		Basic:               nil,
		DNSDefaults:         nil,
	}
}

func ntpScopeTable() scope.Table {
	return scope.Table{
		Roles: map[string][]string{
			model.RoleViewer:        {model.ScopeNTPRead},
			model.RoleOperator:      {model.ScopeNTPRead, model.ScopeNTPWrite},
			model.RoleAdministrator: allScopes(),
		},
		EmptyRole:                model.RoleAdministrator,
		ExplicitReplacesRole:     true,
		AllowUnknownRoleExplicit: false,
		WildcardScope:            model.ScopeNTPAdmin,
	}
}

func fileOpts() authn.FileOpts {
	return authn.FileOpts{
		Line:        authn.FirstNonCommentLine,
		Resolve:     authn.AsGiven,
		BaseDir:     "",
		SkipMissing: false,
		Harden:      false,
		TrimRef:     false,
	}
}

// tokenFileSource opens one secret file per Read, in spec order.
// Stopping after a failed Read leaves every later file untouched.
type tokenFileSource struct {
	entries []authn.FileToken
	opts    authn.FileOpts
	next    int
}

func tokenFiles(spec model.AuthSpec) TokenSource {
	entries := make([]authn.FileToken, len(spec.Tokens))
	for i, tok := range spec.Tokens {
		entries[i] = authn.FileToken{
			ID:         tok.ID,
			Role:       tok.Role,
			Scopes:     append([]string(nil), tok.Scopes...),
			SecretFile: tok.SecretFile,
		}
	}
	return &tokenFileSource{entries: entries, opts: fileOpts()}
}

func (s *tokenFileSource) Read() ([]authn.RawToken, []authn.FileResult, error) {
	if s == nil || s.next >= len(s.entries) {
		return nil, nil, nil
	}
	ent := s.entries[s.next]
	s.next++
	return authn.PerTokenFiles([]authn.FileToken{ent}, s.opts).Read()
}

func (s *tokenFileSource) Spec() any {
	if s == nil {
		return nil
	}
	return authn.PerTokenFiles(s.entries, s.opts).Spec()
}

type rawHold struct {
	tokens []authn.RawToken
}

func (h *rawHold) wipe() {
	if h == nil {
		return
	}
	wipeRaw(h.tokens)
}

func wipeRaw(tokens []authn.RawToken) {
	for i := range tokens {
		tokens[i].Secret.Zero()
	}
}

func invalidSpec(top, path, code, message string) error {
	return domainerr.ValidationFailed(top, domainerr.FieldViolation{
		Path:    path,
		Code:    code,
		Message: message,
	})
}

func tokenPath(i int, leaf string) string {
	return fmt.Sprintf("spec.auth.tokens[%d].%s", i, leaf)
}
