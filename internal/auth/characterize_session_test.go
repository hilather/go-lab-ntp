package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestCharacterizeSessionCookieFlags(t *testing.T) {
	if CookieName != "labntp_session" || CSRFHeader != "X-LabNTP-CSRF" {
		t.Fatalf("cookie %q csrf %q", CookieName, CSRFHeader)
	}
	def := DefaultSessionConfig()
	if def.Idle != 4*time.Hour || def.Absolute != 12*time.Hour || def.Max != 64 {
		t.Fatalf("defaults %+v", def)
	}
	store := NewStore(SessionConfig{})
	if store.MaxAge() != 12*60*60 {
		t.Fatalf("MaxAge %d", store.MaxAge())
	}
	substituted := NewStore(SessionConfig{Idle: -1, Absolute: 0, Max: 0})
	if substituted.MaxAge() != store.MaxAge() || substituted.MaxAge() != int(def.Absolute.Seconds()) {
		t.Fatalf("non-positive absolute was not substituted: MaxAge %d", substituted.MaxAge())
	}

	c := NewSessionCookie("abc", false, store.MaxAge())
	if c.Name != CookieName || c.Path != "/" || !c.HttpOnly || c.Secure || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 43200 {
		t.Fatalf("cookie %+v", c)
	}
	secure := NewSessionCookie("abc", true, 10)
	if !secure.Secure || secure.SameSite != http.SameSiteLaxMode {
		t.Fatalf("secure cookie %+v", secure)
	}
	cleared := ClearSessionCookie(false)
	if cleared.MaxAge != -1 || !cleared.Expires.Equal(time.Unix(0, 0).UTC()) || cleared.SameSite != http.SameSiteLaxMode || cleared.Value != "" {
		t.Fatalf("clear %+v", cleared)
	}

	if CookieSecure(nil, false) || CookieSecure(httptest.NewRequest(http.MethodGet, "/", nil), false) {
		t.Fatal("plain request must not force Secure")
	}
	tlsReq := httptest.NewRequest(http.MethodGet, "/", nil)
	tlsReq.TLS = &tls.ConnectionState{}
	if !CookieSecure(tlsReq, false) || !CookieSecure(httptest.NewRequest(http.MethodGet, "/", nil), true) {
		t.Fatal("Secure via TLS or force")
	}

	fixed := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	store.SetClock(func() time.Time { return fixed })
	store.SetClock(nil)
	_, _, sess, err := store.Create(adminPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	if !sess.CreatedAt.Equal(fixed) {
		t.Fatalf("SetClock(nil) replaced the clock: %s", sess.CreatedAt)
	}
}

func TestCharacterizeSessionTTL(t *testing.T) {
	t.Run("lookup slides idle", func(t *testing.T) {
		st, cur := clockedStore(t, DefaultSessionConfig())
		base := *cur
		cookie, _, _, err := st.Create(adminPrincipal())
		if err != nil {
			t.Fatal(err)
		}
		*cur = base.Add(3 * time.Hour)
		if _, _, ok := st.Lookup(cookie); !ok {
			t.Fatal("lookup inside idle")
		}
		*cur = base.Add(4*time.Hour + time.Second)
		if _, _, ok := st.Lookup(cookie); !ok {
			t.Fatal("lookup at +3h must slide idle past the original deadline")
		}
	})

	t.Run("no lookup expires at idle", func(t *testing.T) {
		st, cur := clockedStore(t, DefaultSessionConfig())
		base := *cur
		cookie, _, _, err := st.Create(adminPrincipal())
		if err != nil {
			t.Fatal(err)
		}
		*cur = base.Add(4*time.Hour + time.Second)
		if _, _, ok := st.Lookup(cookie); ok {
			t.Fatal("idle expiry")
		}
		*cur = base
		if _, _, ok := st.Lookup(cookie); ok {
			t.Fatal("expired row must be deleted")
		}
	})

	t.Run("valid csrf does not slide", func(t *testing.T) {
		st, cur := clockedStore(t, DefaultSessionConfig())
		base := *cur
		cookie, csrf, _, err := st.Create(adminPrincipal())
		if err != nil {
			t.Fatal(err)
		}
		*cur = base.Add(30 * time.Minute)
		if !st.ValidCSRF(cookie, csrf) {
			t.Fatal("csrf inside idle")
		}
		*cur = base.Add(4*time.Hour + time.Second)
		if _, _, ok := st.Lookup(cookie); ok {
			t.Fatal("ValidCSRF must not extend idle")
		}
	})

	t.Run("valid csrf deletes expired", func(t *testing.T) {
		st, cur := clockedStore(t, DefaultSessionConfig())
		base := *cur
		cookie, csrf, _, err := st.Create(adminPrincipal())
		if err != nil {
			t.Fatal(err)
		}
		*cur = base.Add(4*time.Hour + time.Second)
		if st.ValidCSRF(cookie, csrf) {
			t.Fatal("expired csrf")
		}
		*cur = base
		if _, _, ok := st.Lookup(cookie); ok {
			t.Fatal("ValidCSRF must delete the expired row")
		}
	})

	t.Run("absolute wins over slides", func(t *testing.T) {
		st, cur := clockedStore(t, DefaultSessionConfig())
		base := *cur
		cookie, _, _, err := st.Create(adminPrincipal())
		if err != nil {
			t.Fatal(err)
		}
		*cur = base.Add(4*time.Hour - time.Second)
		if _, _, ok := st.Lookup(cookie); !ok {
			t.Fatal("first slide")
		}
		*cur = base.Add(8*time.Hour - time.Second)
		if _, _, ok := st.Lookup(cookie); !ok {
			t.Fatal("second slide")
		}
		*cur = base.Add(12*time.Hour + time.Second)
		if _, _, ok := st.Lookup(cookie); ok {
			t.Fatal("absolute TTL")
		}
	})

	t.Run("non-positive idle expires at 4h", func(t *testing.T) {
		st, cur := clockedStore(t, SessionConfig{Idle: -1, Absolute: 0, Max: 0})
		base := *cur
		first, _, _, err := st.Create(adminPrincipal())
		if err != nil {
			t.Fatal(err)
		}
		second, _, _, err := st.Create(adminPrincipal())
		if err != nil {
			t.Fatal(err)
		}
		*cur = base.Add(4 * time.Hour)
		if _, _, ok := st.Lookup(first); !ok {
			t.Fatal("substituted idle still holds at 4h")
		}
		*cur = base.Add(4*time.Hour + time.Second)
		if _, _, ok := st.Lookup(second); ok {
			t.Fatal("substituted idle expires after 4h")
		}
	})
}

func TestCharacterizeSessionAtCap(t *testing.T) {
	const maxSessions = 64
	cfg := SessionConfig{Idle: time.Hour, Absolute: 48 * time.Hour, Max: maxSessions}
	principal := adminPrincipal()

	t.Run("expire before evict", func(t *testing.T) {
		st, cur := clockedStore(t, cfg)
		base := *cur
		cookies := make([]string, maxSessions)
		for i := 0; i < maxSessions; i++ {
			*cur = base.Add(time.Duration(i) * time.Second)
			cookie, _, _, err := st.Create(principal)
			if err != nil {
				t.Fatal(err)
			}
			cookies[i] = cookie
		}
		*cur = base.Add(time.Hour + time.Second)
		if _, _, _, err := st.Create(principal); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := st.Lookup(cookies[0]); ok {
			t.Fatal("expired oldest must be gone")
		}
		if _, _, ok := st.Lookup(cookies[1]); !ok {
			t.Fatal("live session must survive when an expired row frees a slot")
		}
	})

	t.Run("evict oldest live", func(t *testing.T) {
		st, cur := clockedStore(t, cfg)
		base := *cur
		cookies := make([]string, maxSessions)
		for i := 0; i < maxSessions; i++ {
			*cur = base.Add(time.Duration(i) * time.Second)
			cookie, _, _, err := st.Create(principal)
			if err != nil {
				t.Fatal(err)
			}
			cookies[i] = cookie
		}
		*cur = base.Add(time.Duration(maxSessions-1) * time.Second)
		if _, _, _, err := st.Create(principal); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := st.Lookup(cookies[0]); ok {
			t.Fatal("oldest live session must be evicted at cap")
		}
		if _, _, ok := st.Lookup(cookies[1]); !ok {
			t.Fatal("second-oldest must remain")
		}
	})

	t.Run("non-positive max evicts on the 65th", func(t *testing.T) {
		st, cur := clockedStore(t, SessionConfig{Idle: -1, Absolute: 0, Max: 0})
		base := *cur
		cookies := make([]string, 64)
		for i := 0; i < 64; i++ {
			*cur = base.Add(time.Duration(i) * time.Second)
			cookie, _, _, err := st.Create(principal)
			if err != nil {
				t.Fatal(err)
			}
			cookies[i] = cookie
		}
		*cur = base.Add(63 * time.Second)
		if _, _, _, err := st.Create(principal); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := st.Lookup(cookies[0]); ok {
			t.Fatal("65th session must evict the oldest")
		}
		if _, _, ok := st.Lookup(cookies[1]); !ok {
			t.Fatal("second-oldest must remain")
		}
	})
}

func adminPrincipal() Principal {
	return Principal{
		ID:     "admin",
		Class:  ClassToken,
		Role:   model.RoleAdministrator,
		Scopes: DefaultScopes(model.RoleAdministrator),
	}
}

// sessionAPI is the method set these tests call. The concrete type is inferred
// from NewStore.
type sessionAPI interface {
	Create(Principal) (string, string, Session, error)
	Lookup(string) (Session, string, bool)
	ValidCSRF(string, string) bool
}

func clockedStore(t *testing.T, cfg SessionConfig) (sessionAPI, *time.Time) {
	t.Helper()
	cur := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	st := NewStore(cfg)
	st.SetClock(func() time.Time { return cur })
	return st, &cur
}
