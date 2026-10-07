package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// memStore hält Sessions im Speicher (gleiches Verhalten wie sqlStore).
type memStore struct {
	mu       sync.Mutex
	sessions map[string]memSession
}

type memSession struct {
	userID  string
	expires time.Time
}

func newMemStore() *memStore { return &memStore{sessions: map[string]memSession{}} }

func (m *memStore) CreateSession(_ context.Context, idHash, userID string, _, expires time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[idHash] = memSession{userID: userID, expires: expires}
	return nil
}

func (m *memStore) SessionUser(_ context.Context, idHash string, now time.Time) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[idHash]
	if !ok || !now.Before(s.expires) {
		return "", sdk.ErrNotFound
	}
	return s.userID, nil
}

func (m *memStore) DeleteSession(_ context.Context, idHash string) error {
	m.mu.Lock()
	delete(m.sessions, idHash)
	m.mu.Unlock()
	return nil
}

func (m *memStore) DeleteUserSessions(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		if s.userID == userID {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *memStore) DeleteExpiredSessions(_ context.Context, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		if !now.Before(s.expires) {
			delete(m.sessions, k)
		}
	}
	return nil
}

// memIdentity spielt das Core-Plugin iam (Account.*).
type memIdentity struct {
	mu    sync.Mutex
	users map[string]*memUser // username → Benutzer
}

type memUser struct {
	user
	password string
	active   bool
}

func newMemIdentity() *memIdentity { return &memIdentity{users: map[string]*memUser{}} }

func (m *memIdentity) add(username, password, tenant string, perms ...string) *user {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := &memUser{user: user{ID: "id-" + username, Username: username, TenantID: tenant, Permissions: perms}, password: password, active: true}
	m.users[username] = u
	cp := u.user
	return &cp
}

func (m *memIdentity) Authenticate(_ context.Context, username, password string) (*user, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[strings.ToLower(strings.TrimSpace(username))]
	if !ok || u.password != password || !u.active {
		return nil, errInvalidCredentials
	}
	cp := u.user
	return &cp, nil
}

func (m *memIdentity) byID(id string) *memUser {
	for _, u := range m.users {
		if u.ID == id {
			return u
		}
	}
	return nil
}

func (m *memIdentity) Me(_ context.Context, userID string) (*user, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID(userID)
	if u == nil || !u.active {
		return nil, sdk.ErrNotFound
	}
	cp := u.user
	return &cp, nil
}

func (m *memIdentity) ChangePassword(_ context.Context, userID, current, next string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID(userID)
	if u == nil || u.password != current {
		return fmt.Errorf("%w: Das aktuelle Passwort ist falsch", sdk.ErrInvalidArgument)
	}
	if len(next) < 10 {
		return fmt.Errorf("%w: Das Passwort muss mindestens 10 Zeichen haben", sdk.ErrInvalidArgument)
	}
	u.password = next
	return nil
}

func (m *memIdentity) deactivate(username string) {
	m.mu.Lock()
	m.users[username].active = false
	m.mu.Unlock()
}
