package auth

import (
	"net/http"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestSessionCookieAndCSRF(t *testing.T) {
	s := NewStore(DefaultSessionConfig())
	p := Principal{ID: "admin", Class: ClassToken, Role: model.RoleAdministrator, Scopes: DefaultScopes(model.RoleAdministrator)}
	cookie, csrf, sess, err := s.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if cookie == "" || csrf == "" || sess.TokenID != "admin" {
		t.Fatal(sess)
	}
	got, gotCSRF, ok := s.Lookup(cookie)
	if !ok || got.TokenID != "admin" || gotCSRF != csrf {
		t.Fatal(got)
	}
	if !s.ValidCSRF(cookie, csrf) {
		t.Fatal("csrf")
	}
	if s.ValidCSRF(cookie, "nope") {
		t.Fatal("bad csrf")
	}
	c := NewSessionCookie(cookie, false, s.MaxAge())
	if c.Name != CookieName || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("%+v", c)
	}
	if CookieName != "labntp_session" || CSRFHeader != "X-LabNTP-CSRF" {
		t.Fatal(CookieName, CSRFHeader)
	}
}

func TestOriginAllowlist(t *testing.T) {
	if err := CheckOrigin("", nil); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("http://127.0.0.1:8088", nil); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("https://evil.example", nil); err == nil {
		t.Fatal("non-loopback origin must be denied")
	}
	if err := CheckOrigin("https://lab.example", []string{"https://lab.example"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("file://tmp", nil); err == nil {
		t.Fatal("file://")
	}
}
