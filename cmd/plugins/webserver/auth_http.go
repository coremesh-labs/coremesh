package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/camel/coremesh/pkg/sdk"
)

const sessionCookie = "coremesh_session"

type userKey struct{}

// userFrom liefert den angemeldeten Benutzer der Anfrage (nach requireAuth).
func userFrom(r *http.Request) *user {
	u, _ := r.Context().Value(userKey{}).(*user)
	return u
}

// publicPath: ohne Anmeldung erreichbar.
func publicPath(p string) bool {
	return p == "/login" || strings.HasPrefix(p, "/static/")
}

// securityHeaders setzt Schutz-Header für alle Antworten.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

// checkOrigin ist der CSRF-Schutz für verändernde Anfragen (alles außer
// GET/HEAD/OPTIONS): Browser senden Sec-Fetch-Site bzw. Origin mit; kommt die
// Anfrage von einer fremden Seite, wird sie abgelehnt. Zusammen mit
// SameSite=Lax-Cookies und GET ohne Seiteneffekte genügt das ohne Token.
func checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "Anfrage von fremder Seite abgelehnt", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				http.Error(w, "Anfrage von fremder Seite abgelehnt", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth lässt nur angemeldete Benutzer durch. Ohne Session: bei HTMX
// HX-Redirect auf /login (Status 401), sonst Redirect mit Rücksprungziel.
func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		u, err := s.currentUser(r)
		if err != nil {
			s.logError(r, "Session", err)
			http.Error(w, "Anmeldung derzeit nicht möglich", http.StatusServiceUnavailable)
			return
		}
		if u == nil {
			target := "/login"
			if r.Method == http.MethodGet && r.URL.Path != "/" {
				target += "?next=" + url.QueryEscape(r.URL.RequestURI())
			}
			if isHTMX(r) {
				w.Header().Set("HX-Redirect", target)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

func (s *server) currentUser(r *http.Request) (*user, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, nil
	}
	return s.auth.Session(r.Context(), c.Value)
}

// GET /login
func (s *server) loginForm(w http.ResponseWriter, r *http.Request) {
	if u, _ := s.currentUser(r); u != nil {
		http.Redirect(w, r, safeNext(r.URL.Query().Get("next")), http.StatusSeeOther)
		return
	}
	s.renderLogin(w, r, http.StatusOK, "", "", r.URL.Query().Get("next"))
}

// POST /login
func (s *server) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ungültige Anfrage", http.StatusBadRequest)
		return
	}
	username, next := r.PostForm.Get("username"), r.PostForm.Get("next")
	_, token, expires, err := s.auth.Login(r.Context(), username, r.PostForm.Get("password"), clientIP(r))
	switch {
	case errors.Is(err, errInvalidCredentials):
		s.renderLogin(w, r, http.StatusUnauthorized, err.Error(), username, next)
		return
	case errors.Is(err, errTooManyAttempts):
		s.renderLogin(w, r, http.StatusTooManyRequests, err.Error(), username, next)
		return
	case err != nil:
		s.logError(r, "Login", err)
		s.renderLogin(w, r, http.StatusServiceUnavailable, "Anmeldung derzeit nicht möglich", username, next)
		return
	}
	s.setSessionCookie(w, r, token, expires)
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

// POST /logout
func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.auth.Logout(r.Context(), c.Value); err != nil {
			s.logError(r, "Logout", err)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.secureCookie(r), SameSite: http.SameSiteLaxMode})
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// GET /account/password
func (s *server) passwordForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "password", map[string]any{"User": userFrom(r)}, "Passwort ändern", "")
}

// POST /account/password
func (s *server) changePassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ungültige Anfrage", http.StatusBadRequest)
		return
	}
	u := userFrom(r)
	next := r.PostForm.Get("new_password")
	again := func(msg string) {
		s.render(w, r, http.StatusUnprocessableEntity, "password", map[string]any{"User": u, "Error": msg}, "Passwort ändern", "")
	}
	if next != r.PostForm.Get("confirm_password") {
		again("Die beiden neuen Passwörter stimmen nicht überein")
		return
	}
	token, expires, err := s.auth.ChangePassword(r.Context(), u, r.PostForm.Get("current_password"), next)
	if errors.Is(err, sdk.ErrInvalidArgument) {
		again(strings.TrimPrefix(err.Error(), sdk.ErrInvalidArgument.Error()+": "))
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSessionCookie(w, r, token, expires) // alle anderen Sessions sind beendet
	s.render(w, r, http.StatusOK, "password", map[string]any{"User": u, "Done": true}, "Passwort ändern", "")
}

func (s *server) renderLogin(w http.ResponseWriter, r *http.Request, status int, msg, username, next string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := s.views.fragment(w, "login", map[string]any{
		"AppTitle": s.cfg.Title, "Error": msg, "Username": username, "Next": safeNext(next),
	}); err != nil {
		s.logError(r, "Template login", err)
	}
}

func (s *server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, Secure: s.secureCookie(r), SameSite: http.SameSiteLaxMode,
	})
}

// secureCookie: Secure-Flag bei TLS (auto) oder per Einstellung erzwungen,
// z. B. hinter einem TLS-terminierenden Reverse Proxy.
func (s *server) secureCookie(r *http.Request) bool {
	switch s.cfg.CookieSecure {
	case "true":
		return true
	case "false":
		return false
	}
	return r.TLS != nil
}

// safeNext lässt nur lokale Pfade als Rücksprungziel zu (kein Open Redirect).
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.Contains(next, `\`) {
		return "/"
	}
	return next
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
