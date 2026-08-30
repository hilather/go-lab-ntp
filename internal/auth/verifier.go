package auth

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

const realmBearer = `Bearer realm="labntp"`

// Verifier is the process-local token index. There is no HTTP Basic.
type Verifier struct {
	mu       sync.RWMutex
	mode     string
	tokens   []storedToken
	onChange []func()
}

type storedToken struct {
	id     string
	role   string
	scopes []string
	digest [sha256.Size]byte
}

// Request is one authentication attempt. Adapters fill it from the HTTP
// request; X-Forwarded-For is never consulted.
type Request struct {
	Authorization string
	RemoteAddr    string
}

// FromSpec compiles spec.auth. Missing secret files fail closed.
func FromSpec(spec model.AuthSpec) (*Verifier, error) {
	mode := strings.TrimSpace(spec.Mode)
	if mode == "" {
		mode = model.MgmtAuthBearer
	}
	switch mode {
	case model.MgmtAuthBearer, model.MgmtAuthDevLoopbackUnauth:
	default:
		return nil, domainerr.ValidationFailed("unknown auth mode",
			domainerr.FieldViolation{Path: "spec.auth.mode", Code: "invalid_value", Message: "unknown auth mode"})
	}

	seenID := map[string]int{}
	seenDigest := map[[sha256.Size]byte]string{}
	tokens := make([]storedToken, 0, len(spec.Tokens))
	for i, tok := range spec.Tokens {
		id := strings.TrimSpace(tok.ID)
		if id == "" {
			return nil, domainerr.ValidationFailed("token id is required",
				domainerr.FieldViolation{Path: indexPath("spec.auth.tokens", i) + ".id", Code: "empty_id", Message: "token id is required"})
		}
		if _, ok := seenID[id]; ok {
			return nil, domainerr.ValidationFailed("duplicate token id",
				domainerr.FieldViolation{Path: indexPath("spec.auth.tokens", i) + ".id", Code: "duplicate_id", Message: "duplicate token id"})
		}
		raw, err := readSecretFile(tok.SecretFile)
		if err != nil {
			return nil, domainerr.ValidationFailed("token secret is unavailable",
				domainerr.FieldViolation{Path: indexPath("spec.auth.tokens", i) + ".secretFile", Code: "unresolved_reference", Message: "token secret file does not resolve"})
		}
		if len(raw) < MinTokenBytes {
			zero(raw)
			return nil, domainerr.ValidationFailed("token entropy is below 256 bits",
				domainerr.FieldViolation{Path: indexPath("spec.auth.tokens", i) + ".secretFile", Code: "invalid_value", Message: "token secret must be at least 32 bytes"})
		}
		d := DigestSecret(raw)
		zero(raw)
		if other, ok := seenDigest[d]; ok {
			return nil, domainerr.ValidationFailed("duplicate token value",
				domainerr.FieldViolation{Path: indexPath("spec.auth.tokens", i) + ".secretFile", Code: "duplicate_id", Message: "token value matches " + other})
		}
		if tok.Role != "" && !model.KnownRole(tok.Role) {
			return nil, domainerr.ValidationFailed("unknown role",
				domainerr.FieldViolation{Path: indexPath("spec.auth.tokens", i) + ".role", Code: "invalid_value", Message: "role must be viewer, operator, or administrator"})
		}
		seenID[id] = len(tokens)
		seenDigest[d] = id
		role, scopes := expandScopes(tok.Role, tok.Scopes)
		tokens = append(tokens, storedToken{id: id, role: role, scopes: scopes, digest: d})
	}

	return &Verifier{mode: mode, tokens: tokens}, nil
}

// Static builds a bearer verifier from an in-memory secret (contract tests).
func Static(secret, id, role string) *Verifier {
	if id == "" {
		id = "admin"
	}
	r, scopes := expandScopes(role, nil)
	return &Verifier{
		mode: model.MgmtAuthBearer,
		tokens: []storedToken{{
			id:     id,
			role:   r,
			scopes: scopes,
			digest: DigestSecret([]byte(secret)),
		}},
	}
}

// OnIdentityChange registers a hook fired after Replace when identity changed.
func (v *Verifier) OnIdentityChange(fn func()) {
	if v == nil || fn == nil {
		return
	}
	v.mu.Lock()
	v.onChange = append(v.onChange, fn)
	v.mu.Unlock()
}

// Replace swaps the compiled index in place so REST/MCP share one pointer.
func (v *Verifier) Replace(next *Verifier) {
	if v == nil || next == nil {
		return
	}
	changed := !v.Equivalent(next)
	mode, toks := next.snapshot()
	v.mu.Lock()
	v.mode = mode
	v.tokens = toks
	hooks := append([]func(){}, v.onChange...)
	v.mu.Unlock()
	if !changed {
		return
	}
	for _, fn := range hooks {
		fn()
	}
}

// Equivalent reports whether the compiled identity matches.
func (v *Verifier) Equivalent(other *Verifier) bool {
	if v == nil || other == nil {
		return v == other
	}
	modeA, toksA := v.snapshot()
	modeB, toksB := other.snapshot()
	if modeA != modeB || len(toksA) != len(toksB) {
		return false
	}
	byID := make(map[string]storedToken, len(toksA))
	for _, t := range toksA {
		byID[t.id] = t
	}
	for _, t := range toksB {
		got, ok := byID[t.id]
		if !ok || !EqualDigest(got.digest, t.digest) || got.role != t.role || !sameScopes(got.scopes, t.scopes) {
			return false
		}
	}
	return true
}

func sameScopes(a, b []string) bool {
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}

func (v *Verifier) snapshot() (string, []storedToken) {
	if v == nil {
		return "", nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.mode, append([]storedToken(nil), v.tokens...)
}

// Mode is the compiled auth mode.
func (v *Verifier) Mode() string {
	if v == nil {
		return ""
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.mode
}

// TokenCount is the number of compiled bearer principals.
func (v *Verifier) TokenCount() int {
	if v == nil {
		return 0
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.tokens)
}

// RequireListen refuses a management bind that would be allow-all.
func (v *Verifier) RequireListen() error {
	if v == nil {
		return fmt.Errorf("management bind requires a verifier")
	}
	mode, tokens := v.snapshot()
	switch mode {
	case model.MgmtAuthDevLoopbackUnauth:
		return nil
	case model.MgmtAuthBearer:
		if len(tokens) == 0 {
			return fmt.Errorf("spec.auth.mode bearer requires at least one usable token")
		}
		return nil
	default:
		return fmt.Errorf("management bind refused: unknown auth mode %q", mode)
	}
}

// WWWAuthenticate is the 401 challenge list. There is no Basic.
func WWWAuthenticate() []string {
	return []string{realmBearer}
}

// Authenticate verifies Authorization. A missing header is unauthenticated
// unless mode is dev-loopback-unauth and RemoteAddr is loopback.
func (v *Verifier) Authenticate(in Request) (Principal, error) {
	if v == nil {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	v.mu.RLock()
	defer v.mu.RUnlock()

	h := strings.TrimSpace(in.Authorization)
	if h == "" {
		if v.mode == model.MgmtAuthDevLoopbackUnauth && IsLoopback(in.RemoteAddr) {
			return loopbackPrincipal(), nil
		}
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}

	scheme, rest, ok := strings.Cut(h, " ")
	if !ok {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	rest = strings.TrimSpace(rest)
	if !strings.EqualFold(scheme, "Bearer") {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	return v.lookupBearerLocked(rest)
}

// AuthenticateBearer looks up a raw token secret (mcp-stdio --token-file).
func (v *Verifier) AuthenticateBearer(secret string) (Principal, error) {
	if v == nil {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.lookupBearerLocked(strings.TrimSpace(secret))
}

func (v *Verifier) lookupBearerLocked(secret string) (Principal, error) {
	if secret == "" || strings.ContainsAny(secret, " \t") {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	digest := DigestSecret([]byte(secret))
	found := 0
	idx := 0
	for i, t := range v.tokens {
		eq := 0
		if EqualDigest(t.digest, digest) {
			eq = 1
		}
		mask := eq
		idx = idx*(1-mask) + i*mask
		found += eq
	}
	if found != 1 {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	return principalOf(v.tokens[idx]), nil
}

func principalOf(t storedToken) Principal {
	return Principal{
		ID:     t.id,
		Class:  ClassToken,
		Role:   t.role,
		Scopes: append([]string(nil), t.scopes...),
	}
}

func loopbackPrincipal() Principal {
	return Principal{
		ID:     "loopback",
		Class:  ClassLoopback,
		Role:   model.RoleAdministrator,
		Scopes: allScopes(),
	}
}

func readSecretFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return []byte(line), nil
	}
	return nil, os.ErrInvalid
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func indexPath(base string, i int) string {
	return strings.TrimSuffix(base, ".") + "[" + strconv.Itoa(i) + "]"
}

// IsLoopback reports whether remoteAddr is a loopback host (with or without port).
func IsLoopback(remoteAddr string) bool {
	host := strings.TrimSpace(remoteAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback()
}
