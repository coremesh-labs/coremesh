package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

// Anmeldung im WebServer.
//
// Benutzer, Passwörter und Rollen verwaltet das Core-Plugin iam; der WebServer
// prüft Anmeldedaten über Account.Authenticate und lädt das Profil des
// angemeldeten Benutzers (inkl. Berechtigungen) über Account.Me. Er selbst
// hält nur:
//
//   - Sessions: Das Cookie enthält ein zufälliges Token (32 Byte); in der
//     Datenbank steht nur dessen SHA-256.
//   - Brute-Force-Schutz: Nach 5 Fehlversuchen je Benutzer+IP wird gesperrt,
//     mit wachsender Wartezeit (1, 2, 4, 8 … max. 15 Minuten).

var (
	errInvalidCredentials = errors.New("Benutzername oder Passwort falsch")
	errTooManyAttempts    = errors.New("Zu viele Fehlversuche – bitte später erneut versuchen")
)

// user ist der angemeldete Benutzer (aus Account.Me).
type user struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	TenantID    string   `json:"tenant_id"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"` // "Object.Action[@Buchungskreis,…]" mit Platzhaltern
	Locale      string   `json:"locale"`      // Sprache aus dem Profil ("" = automatisch)
}

// Name ist der Anzeigename.
func (u *user) Name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// Can meldet, ob der Benutzer object.action aufrufen darf (in irgendeinem
// Buchungskreis) – nur für die
// Darstellung (Buttons, Navigation). Verbindlich prüft der Dispatcher.
func (u *user) Can(object, action string) bool {
	for _, p := range u.Permissions {
		spec, _, _ := strings.Cut(p, "@") // Buchungskreise zählen hier nicht
		o, a, _ := strings.Cut(spec, ".")
		okO, _ := path.Match(o, object)
		okA, _ := path.Match(a, action)
		if okO && okA {
			return true
		}
	}
	return false
}

// CanAny meldet, ob der Benutzer irgendeine Action auf object darf.
func (u *user) CanAny(object string) bool {
	for _, p := range u.Permissions {
		o, _, _ := strings.Cut(p, ".")
		if ok, _ := path.Match(o, object); ok {
			return true
		}
	}
	return false
}

// identity ist der Zugang zu Benutzerdaten (iam über den Host; in Tests im Speicher).
type identity interface {
	// Authenticate liefert den Benutzer oder errInvalidCredentials.
	Authenticate(ctx context.Context, username, password string) (*user, error)
	// Me liefert den aktiven Benutzer zur ID oder sdk.ErrNotFound.
	Me(ctx context.Context, userID string) (*user, error)
	ChangePassword(ctx context.Context, userID, current, next string) error
}

// store hält die Sessions.
type store interface {
	CreateSession(ctx context.Context, idHash, userID string, now, expires time.Time) error
	// SessionUser liefert die Benutzer-ID einer gültigen Session oder sdk.ErrNotFound.
	SessionUser(ctx context.Context, idHash string, now time.Time) (string, error)
	DeleteSession(ctx context.Context, idHash string) error
	DeleteUserSessions(ctx context.Context, userID string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error
}

type authService struct {
	id      identity
	store   store
	ttl     time.Duration
	now     func() time.Time
	limiter *limiter
}

func newAuthService(id identity, st store, ttl time.Duration) *authService {
	return &authService{id: id, store: st, ttl: ttl, now: time.Now, limiter: newLimiter()}
}

// Login prüft die Anmeldedaten (iam) und eröffnet eine Session.
func (a *authService) Login(ctx context.Context, username, password, client string) (*user, string, time.Time, error) {
	key := strings.ToLower(strings.TrimSpace(username)) + "|" + client
	if a.limiter.blocked(key, a.now()) {
		return nil, "", time.Time{}, errTooManyAttempts
	}
	u, err := a.id.Authenticate(ctx, username, password)
	if errors.Is(err, errInvalidCredentials) {
		a.limiter.fail(key, a.now())
		return nil, "", time.Time{}, err
	}
	if err != nil {
		return nil, "", time.Time{}, err
	}
	a.limiter.reset(key)
	token, expires, err := a.newSession(ctx, u.ID)
	return u, token, expires, err
}

func (a *authService) newSession(ctx context.Context, userID string) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	now := a.now().UTC()
	expires := now.Add(a.ttl)
	if err := a.store.CreateSession(ctx, hashToken(token), userID, now, expires); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// Session liefert den Benutzer zum Cookie-Token oder nil. Gelöschte oder
// deaktivierte Benutzer verlieren ihre Session sofort.
func (a *authService) Session(ctx context.Context, token string) (*user, error) {
	if token == "" {
		return nil, nil
	}
	userID, err := a.store.SessionUser(ctx, hashToken(token), a.now().UTC())
	if errors.Is(err, sdk.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u, err := a.id.Me(ctx, userID)
	if errors.Is(err, sdk.ErrNotFound) {
		_ = a.store.DeleteUserSessions(ctx, userID)
		return nil, nil
	}
	return u, err
}

func (a *authService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return a.store.DeleteSession(ctx, hashToken(token))
}

// ChangePassword ändert das Passwort (iam), beendet alle Sessions des
// Benutzers und eröffnet eine neue für die aktuelle Anmeldung.
func (a *authService) ChangePassword(ctx context.Context, u *user, current, next string) (string, time.Time, error) {
	if err := a.id.ChangePassword(ctx, u.ID, current, next); err != nil {
		return "", time.Time{}, err
	}
	if err := a.store.DeleteUserSessions(ctx, u.ID); err != nil {
		return "", time.Time{}, err
	}
	return a.newSession(ctx, u.ID)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// --- Schutz vor Brute Force -------------------------------------------------

type limiter struct {
	mu      sync.Mutex
	entries map[string]*attempts
}

type attempts struct {
	fails int
	until time.Time
}

const (
	freeAttempts = 5
	maxLockout   = 15 * time.Minute
)

func newLimiter() *limiter { return &limiter{entries: map[string]*attempts{}} }

func (l *limiter) blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	return e != nil && now.Before(e.until)
}

func (l *limiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil {
		e = &attempts{}
		l.entries[key] = e
	}
	e.fails++
	if e.fails >= freeAttempts {
		lock := time.Minute << min(e.fails-freeAttempts, 4)
		e.until = now.Add(min(lock, maxLockout))
	}
	if len(l.entries) > 10000 { // Speicher begrenzen
		for k, v := range l.entries {
			if now.After(v.until) {
				delete(l.entries, k)
			}
		}
	}
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

// --- iam über den Host ----------------------------------------------------------

// hostIdentity ruft das Core-Plugin iam über den Host auf (Ingress).
type hostIdentity struct{ host sdk.Host }

func (h *hostIdentity) call(ctx context.Context, userID, action string, payload any) (sdk.Response, error) {
	ctx = sdk.WithCall(ctx, sdk.CallContext{RequestID: newID(), UserID: userID,
		Metadata: map[string]string{"ingress": name, "component": "auth"}})
	return h.host.Handle(ctx, sdk.Request{Object: "Account", Action: action, Payload: payload})
}

func (h *hostIdentity) Authenticate(ctx context.Context, username, password string) (*user, error) {
	resp, err := h.call(ctx, "", "Authenticate", map[string]any{"username": username, "password": password})
	if errors.Is(err, sdk.ErrPermissionDenied) {
		return nil, errInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	return decodeUser(resp.Payload)
}

func (h *hostIdentity) Me(ctx context.Context, userID string) (*user, error) {
	resp, err := h.call(ctx, userID, "Me", nil)
	if err != nil {
		return nil, err
	}
	return decodeUser(resp.Payload)
}

func (h *hostIdentity) ChangePassword(ctx context.Context, userID, current, next string) error {
	_, err := h.call(ctx, userID, "ChangePassword", map[string]any{"current": current, "new": next})
	return err
}

func decodeUser(payload any) (*user, error) {
	var u user
	if err := sdk.Decode(payload, &u); err != nil {
		return nil, err
	}
	if u.ID == "" {
		return nil, fmt.Errorf("Account-Antwort ohne id")
	}
	return &u, nil
}

// --- Sessions in der Datenbank ----------------------------------------------------

// sqlStore greift über den Host auf webserver__sessions zu. Der WebServer ist
// Ingress: Jeder Aufruf läuft als eigene kurze Wurzelanfrage.
type sqlStore struct {
	host sdk.Host
	db   string
}

func (s *sqlStore) ctx(ctx context.Context) context.Context {
	return sdk.WithCall(ctx, sdk.CallContext{RequestID: newID(), Metadata: map[string]string{"ingress": name, "component": "auth"}})
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func (s *sqlStore) CreateSession(ctx context.Context, idHash, userID string, now, expires time.Time) error {
	_, err := s.host.Exec(s.ctx(ctx), s.db,
		`INSERT INTO webserver__sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		idHash, userID, ts(now), ts(expires))
	return err
}

func (s *sqlStore) SessionUser(ctx context.Context, idHash string, now time.Time) (string, error) {
	res, err := s.host.Query(s.ctx(ctx), s.db,
		`SELECT user_id FROM webserver__sessions WHERE id = ? AND expires_at > ?`, idHash, ts(now))
	if err != nil {
		return "", err
	}
	if len(res.Rows) == 0 {
		return "", sdk.ErrNotFound
	}
	return scalar(res.Rows[0][0]), nil
}

func (s *sqlStore) DeleteSession(ctx context.Context, idHash string) error {
	_, err := s.host.Exec(s.ctx(ctx), s.db, `DELETE FROM webserver__sessions WHERE id = ?`, idHash)
	return err
}

func (s *sqlStore) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.host.Exec(s.ctx(ctx), s.db, `DELETE FROM webserver__sessions WHERE user_id = ?`, userID)
	return err
}

func (s *sqlStore) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.host.Exec(s.ctx(ctx), s.db, `DELETE FROM webserver__sessions WHERE expires_at <= ?`, ts(now))
	return err
}
