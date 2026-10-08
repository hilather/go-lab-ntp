package mcp

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/model"
)

const rotatedBearer = "abcdef0123456789abcdef0123456789"

func TestStdioFixedActorFollowsTokenDowngrade(t *testing.T) {
	s := newStdioServer(t, auth.Static(testBearerToken, "admin", model.RoleAdministrator))
	if err := s.authorizeTool(s.actorFrom(context.Background()), "ntp_state_reset"); err != nil {
		t.Fatal(err)
	}

	s.cfg.Auth.Replace(auth.Static(testBearerToken, "admin", model.RoleViewer))
	actor := s.actorFrom(context.Background())
	if err := s.authorizeTool(actor, "ntp_state_reset"); err == nil {
		t.Fatalf("stdio actor still authorized for ntp_state_reset after downgrade, scopes=%v", actor.Scopes)
	}
	if err := s.authorizeTool(actor, "ntp_state_get"); err != nil {
		t.Fatalf("viewer read: %v", err)
	}
}

func TestStdioSecretRotationDropsActor(t *testing.T) {
	s := newStdioServer(t, auth.Static(testBearerToken, "admin", model.RoleAdministrator))
	s.cfg.Auth.Replace(auth.Static(rotatedBearer, "admin", model.RoleAdministrator))
	actor := s.actorFrom(context.Background())
	if actor.ID != "" || len(actor.Scopes) != 0 {
		t.Fatalf("rotated secret kept actor %+v", actor)
	}
	if err := s.authorizeTool(actor, "ntp_state_reset"); err == nil {
		t.Fatal("ntp_state_reset allowed after secret rotation")
	}
}

func TestStdioSecretRemovalDropsActor(t *testing.T) {
	s := newStdioServer(t, auth.Static(testBearerToken, "admin", model.RoleAdministrator))
	empty, err := auth.FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer})
	if err != nil {
		t.Fatal(err)
	}
	s.cfg.Auth.Replace(empty)
	actor := s.actorFrom(context.Background())
	if actor.ID != "" || len(actor.Scopes) != 0 {
		t.Fatalf("removed secret kept actor %+v", actor)
	}
	if err := s.authorizeTool(actor, "ntp_state_reset"); err == nil {
		t.Fatal("ntp_state_reset allowed after token removal")
	}
}

func newStdioServer(t *testing.T, verifier *auth.Verifier) *Server {
	t.Helper()
	svc := bootTestApp(t)
	principal, err := verifier.AuthenticateBearer(testBearerToken)
	if err != nil {
		t.Fatal(err)
	}
	fixed := app.Actor{
		ID:        principal.ID,
		Class:     principal.Class,
		Role:      principal.Role,
		Scopes:    append([]string(nil), principal.Scopes...),
		Transport: "mcp",
	}
	s, err := New(Config{
		Service:            svc,
		Auth:               verifier,
		FixedActor:         &fixed,
		StdioSecret:        testBearerToken,
		AllowLegacyClients: true,
		RatePerSec:         -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
