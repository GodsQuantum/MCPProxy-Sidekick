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

type Manager struct {
	key      []byte
	lifetime time.Duration
	mu       sync.RWMutex
	sessions map[string]Session
	now      func() time.Time
}

func NewManager(keyFile string, lifetime time.Duration) (*Manager, error) {
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	key := []byte(strings.TrimSpace(string(raw)))
	if len(key) == 0 {
		return nil, errors.New("empty MCPProxy admin key")
	}
	if lifetime <= 0 {
		return nil, errors.New("session lifetime must be positive")
	}
	return &Manager{
		key:      key,
		lifetime: lifetime,
		sessions: make(map[string]Session),
		now:      time.Now,
	}, nil
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
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) Validate(id string) (Session, bool) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if !m.now().Before(s.ExpiresAt) {
		m.mu.Lock()
		delete(m.sessions, id)
		m.mu.Unlock()
		return Session{}, false
	}
	return s, true
}

func (m *Manager) Delete(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func randomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
