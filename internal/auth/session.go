package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"sync"
	"time"
)

type Session struct {
	ID        string
	CSRFToken string
	ExpiresAt time.Time
}

type SessionStore interface {
	SaveAuthSession(id, csrf string, expiresAt time.Time) error
	AuthSession(id string) (csrf string, expiresAt time.Time, ok bool, err error)
	DeleteAuthSession(id string) error
	PurgeExpiredAuthSessions(now time.Time) error
}

type Manager struct {
	key      []byte
	lifetime time.Duration
	mu       sync.RWMutex
	sessions map[string]Session
	store    SessionStore
	now      func() time.Time
}

func NewManager(keyFile string, lifetime time.Duration) (*Manager, error) {
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	return NewManagerFromKey(string(raw), lifetime)
}

func NewPersistentManager(keyFile string, lifetime time.Duration, store SessionStore) (*Manager, error) {
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	return NewPersistentManagerFromKey(string(raw), lifetime, store)
}

func NewManagerFromKey(rawKey string, lifetime time.Duration) (*Manager, error) {
	return newManager(rawKey, lifetime, nil)
}

func NewPersistentManagerFromKey(rawKey string, lifetime time.Duration, store SessionStore) (*Manager, error) {
	if store == nil {
		return nil, errors.New("session store is required")
	}
	return newManager(rawKey, lifetime, store)
}

func newManager(rawKey string, lifetime time.Duration, store SessionStore) (*Manager, error) {
	key := []byte(strings.TrimSpace(rawKey))
	if len(key) == 0 {
		return nil, errors.New("empty MCPProxy admin key")
	}
	if lifetime <= 0 {
		return nil, errors.New("session lifetime must be positive")
	}
	m := &Manager{
		key:      key,
		lifetime: lifetime,
		sessions: make(map[string]Session),
		store:    store,
		now:      time.Now,
	}
	if store != nil {
		if err := store.PurgeExpiredAuthSessions(m.now()); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Manager) Login(provided string) (Session, error) {
	got := []byte(strings.TrimSpace(provided))
	if len(got) != len(m.key) || subtle.ConstantTimeCompare(got, m.key) != 1 {
		return Session{}, errors.New("invalid MCPProxy admin key")
	}
	s := Session{
		ID:        randomToken(32),
		CSRFToken: randomToken(32),
		ExpiresAt: m.now().Add(m.lifetime),
	}
	if m.store != nil {
		if err := m.store.SaveAuthSession(s.ID, s.CSRFToken, s.ExpiresAt); err != nil {
			return Session{}, err
		}
	}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) Validate(id string) (Session, bool) {
	var s Session
	var ok bool
	if m.store != nil {
		csrf, expiresAt, found, err := m.store.AuthSession(id)
		if err != nil || !found {
			return Session{}, false
		}
		s = Session{ID: id, CSRFToken: csrf, ExpiresAt: expiresAt}
		ok = true
	} else {
		m.mu.RLock()
		s, ok = m.sessions[id]
		m.mu.RUnlock()
	}
	if !ok {
		return Session{}, false
	}
	if !m.now().Before(s.ExpiresAt) {
		m.mu.Lock()
		delete(m.sessions, id)
		m.mu.Unlock()
		if m.store != nil {
			_ = m.store.DeleteAuthSession(id)
		}
		return Session{}, false
	}
	return s, true
}

func (m *Manager) Delete(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	if m.store != nil {
		_ = m.store.DeleteAuthSession(id)
	}
}

func randomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
