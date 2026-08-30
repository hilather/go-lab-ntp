package rest

import (
	"net/http"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	if s.cfg.Sessions == nil {
		s.writeProblem(w, r, instance, domainerr.Internal("session store unavailable"))
		return
	}
	p := auth.Principal{ID: actor.ID, Class: actor.Class, Role: actor.Role, Scopes: actor.Scopes}
	cookie, csrf, sess, err := s.cfg.Sessions.Create(p)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	http.SetCookie(w, auth.NewSessionCookie(cookie, s.cookieSecure(r), s.cfg.Sessions.MaxAge()))
	s.writeJSON(w, http.StatusOK, sessionCreateJSON{
		CSRF:      csrf,
		ExpiresAt: rfc3339(s.cfg.Sessions.ExpiresAt(sess)),
	})
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	out := sessionViewJSON{
		ID:     actor.ID,
		Role:   actor.Role,
		Scopes: actor.Scopes,
	}
	if out.Scopes == nil {
		out.Scopes = []string{}
	}
	if c, err := r.Cookie(auth.CookieName); err == nil && s.cfg.Sessions != nil {
		if sess, csrf, ok := s.cfg.Sessions.Lookup(c.Value); ok {
			out.CSRF = csrf
			out.ExpiresAt = rfc3339(s.cfg.Sessions.ExpiresAt(sess))
		}
	}
	s.writeJSON(w, http.StatusOK, out)
	_ = instance
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	if c, err := r.Cookie(auth.CookieName); err == nil && s.cfg.Sessions != nil {
		s.cfg.Sessions.Delete(c.Value)
	}
	http.SetCookie(w, auth.ClearSessionCookie(s.cookieSecure(r)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
	_ = instance
	_ = actor
}
