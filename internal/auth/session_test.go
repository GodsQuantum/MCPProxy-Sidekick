package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeKeyFile(t *testing.T, value string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(p, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoginValidatesAdminKeyAndCreatesSession(t *testing.T) {
	m, err := NewManager(writeKeyFile(t, "secret-key"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s, err := m.Login("secret-key")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if len(s.ID) < 32 || len(s.CSRFToken) < 32 {
		t.Fatalf("session entropy too small: %+v", s)
	}
	if _, ok := m.Validate(s.ID); !ok {
		t.Fatal("new session not valid")
	}
}

func TestLoginRejectsWrongAdminKey(t *testing.T) {
	m, err := NewManager(writeKeyFile(t, "secret-key"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Login("wrong"); err == nil {
		t.Fatal("expected invalid key to fail")
	}
}

func TestSessionExpires(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, err := NewManager(writeKeyFile(t, "secret-key"), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return now }
	s, err := m.Login("secret-key")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := m.Validate(s.ID); ok {
		t.Fatal("expired session accepted")
	}
}
