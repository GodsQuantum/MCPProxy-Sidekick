package storage

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize sidekick metadata db: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure sidekick metadata db: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

type CredentialMeta struct {
	ServerName    string `json:"server_name"`
	MaskedPreview string `json:"masked_preview"`
	Fingerprint   string `json:"fingerprint"`
	UpdatedAt     string `json:"updated_at"`
}

func (s *Store) UpsertCredentialMeta(m CredentialMeta) error {
	_, err := s.db.Exec(
		"INSERT INTO credential_meta(server_name,masked_preview,fingerprint,updated_at) VALUES(?,?,?,?) ON CONFLICT(server_name) DO UPDATE SET masked_preview=excluded.masked_preview, fingerprint=excluded.fingerprint, updated_at=excluded.updated_at",
		m.ServerName, m.MaskedPreview, m.Fingerprint, m.UpdatedAt,
	)
	return err
}

func (s *Store) CredentialMeta(server string) (CredentialMeta, bool, error) {
	var m CredentialMeta
	err := s.db.QueryRow("SELECT server_name,masked_preview,fingerprint,updated_at FROM credential_meta WHERE server_name=?", server).
		Scan(&m.ServerName, &m.MaskedPreview, &m.Fingerprint, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return CredentialMeta{}, false, nil
	}
	if err != nil {
		return CredentialMeta{}, false, err
	}
	return m, true, nil
}

func (s *Store) Setting(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at",
		key, value, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) SaveAuthSession(id, csrf string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		"INSERT INTO auth_sessions(session_id,csrf_token,expires_at) VALUES(?,?,?) ON CONFLICT(session_id) DO UPDATE SET csrf_token=excluded.csrf_token, expires_at=excluded.expires_at",
		id, csrf, expiresAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) AuthSession(id string) (string, time.Time, bool, error) {
	var csrf, rawExpiry string
	err := s.db.QueryRow("SELECT csrf_token,expires_at FROM auth_sessions WHERE session_id=?", id).Scan(&csrf, &rawExpiry)
	if err == sql.ErrNoRows {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, rawExpiry)
	if err != nil {
		return "", time.Time{}, false, err
	}
	return csrf, expiresAt, true, nil
}

func (s *Store) DeleteAuthSession(id string) error {
	_, err := s.db.Exec("DELETE FROM auth_sessions WHERE session_id=?", id)
	return err
}

func (s *Store) PurgeExpiredAuthSessions(now time.Time) error {
	_, err := s.db.Exec("DELETE FROM auth_sessions WHERE expires_at<=?", now.UTC().Format(time.RFC3339Nano))
	return err
}
