package profiles

import (
	"database/sql"
	_ "embed"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Profile struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	SortOrder int      `json:"sort_order"`
	Servers   []string `json:"servers"`
}

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
		return nil, fmt.Errorf("initialize profiles db: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) UpsertProfile(p Profile) error {
	_, err := s.db.Exec("INSERT INTO profiles(id,label,sort_order) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET label=excluded.label, sort_order=excluded.sort_order", p.ID, p.Label, p.SortOrder)
	return err
}

func (s *Store) AssignServer(profileID, server string) error {
	_, err := s.db.Exec("INSERT INTO profile_servers(profile_id,server_name) VALUES(?,?) ON CONFLICT(profile_id,server_name) DO NOTHING", profileID, server)
	return err
}

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

func (s *Store) ListProfiles() ([]Profile, error) {
	rows, err := s.db.Query("SELECT id,label,sort_order FROM profiles ORDER BY sort_order,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Profile
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.ID, &p.Label, &p.SortOrder); err != nil {
			return nil, err
		}
		sRows, err := s.db.Query("SELECT server_name FROM profile_servers WHERE profile_id=? ORDER BY server_name", p.ID)
		if err != nil {
			return nil, err
		}
		for sRows.Next() {
			var name string
			if err := sRows.Scan(&name); err != nil {
				_ = sRows.Close()
				return nil, err
			}
			p.Servers = append(p.Servers, name)
		}
		_ = sRows.Close()
		sort.Strings(p.Servers)
		out = append(out, p)
	}
	return out, rows.Err()
}
