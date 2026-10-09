package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/model"
)

// Harden false reads a secret larger than the kit's 1 MiB cap.
func TestOptionPinHardenAllowsOversizeSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big")
	body := bytes.Repeat([]byte{'a'}, 1<<20+1)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{{
		ID: "admin", Role: model.RoleAdministrator, SecretFile: path,
	}}}); err != nil {
		t.Fatalf("oversize secret: %v", err)
	}
}

// TrimRef false opens the secretFile text as written, including padding.
func TestOptionPinTrimRefLeavesPadding(t *testing.T) {
	dir := t.TempDir()
	path := writeSecret(t, dir, "good", "0123456789abcdef0123456789abcdef\n")
	padded := "  " + path + "  "
	if _, err := FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer, Tokens: []model.TokenSpec{{
		ID: "admin", Role: model.RoleAdministrator, SecretFile: padded,
	}}}); err == nil {
		t.Fatal("padded secretFile resolved")
	}
}

// SeparateCookieSecret keeps a 32-hex public id distinct from the cookie.
func TestOptionPinSeparateCookieSecret(t *testing.T) {
	st := NewStore(DefaultSessionConfig())
	cookie, _, sess, err := st.Create(adminPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	if cookie == sess.ID {
		t.Fatal("cookie and public id are the same")
	}
	if len(sess.ID) != 32 {
		t.Fatalf("public id length %d", len(sess.ID))
	}
}

// DigestConstantTime rejects a hex CSRF whose only change is case.
func TestOptionPinCSRFCompareCase(t *testing.T) {
	st := NewStore(DefaultSessionConfig())
	var cookie, csrf string
	for i := 0; i < 8; i++ {
		var err error
		cookie, csrf, _, err = st.Create(adminPrincipal())
		if err != nil {
			t.Fatal(err)
		}
		if csrf != strings.ToUpper(csrf) {
			break
		}
	}
	if csrf == strings.ToUpper(csrf) {
		t.Fatal("csrf had no a-f digit")
	}
	if !st.ValidCSRF(cookie, csrf) {
		t.Fatal("exact csrf")
	}
	if st.ValidCSRF(cookie, strings.ToUpper(csrf)) {
		t.Fatal("uppercased csrf was accepted")
	}
}

// URLParse strips userinfo, so a loopback host after it is still loopback.
func TestOptionPinOriginURLParseUserinfo(t *testing.T) {
	err := CheckOrigin("http://user@127.0.0.1", []string{"https://lab.example"})
	if err != nil {
		t.Fatalf("userinfo loopback: %v", err)
	}
}

// ZonedLoopback false denies a zoned IPv6 loopback origin.
func TestOptionPinOriginZonedLoopbackDenied(t *testing.T) {
	err := CheckOrigin("http://[::1%25eth0]", []string{"https://lab.example"})
	if err == nil {
		t.Fatal("zoned loopback was allowed")
	}
}

// A nil Sentinels list does not treat "*" as allow-any.
func TestOptionPinOriginStarNotSentinel(t *testing.T) {
	err := CheckOrigin("https://evil.example", []string{"*"})
	if err == nil {
		t.Fatal("star allowlist entry was a sentinel")
	}
}

// Challenge is bearer-only. basic false adds no Basic challenge.
func TestOptionPinChallengeNoBasic(t *testing.T) {
	got := WWWAuthenticate()
	if len(got) != 1 {
		t.Fatalf("challenge %q", got)
	}
	for _, v := range got {
		if strings.Contains(strings.ToLower(v), "basic") {
			t.Fatalf("basic challenge %q", got)
		}
	}
}
