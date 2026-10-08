package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCredentialMetaRoundTrip(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	want := CredentialMeta{
		ServerName: "github", MaskedPreview: "ghp_••••1234",
		Fingerprint: "sha256:example", UpdatedAt: "2026-09-19T20:00:00Z",
	}
	if err := store.UpsertCredentialMeta(want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.CredentialMeta("github")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != want {
		t.Fatalf("meta=%#v ok=%v", got, ok)
	}
}

func TestCredentialMetaMissing(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, ok, err := store.CredentialMeta("missing"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestSettingRoundTrip(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SetSetting("browser_instance", "playwright-primary"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Setting("browser_instance")
	if err != nil || !ok || got != "playwright-primary" {
		t.Fatalf("got=%q ok=%v err=%v", got, ok, err)
	}
}

func TestAuthSessionRoundTripAndPurge(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	expiry := time.Now().UTC().Add(time.Hour).Round(0)
	if err := store.SaveAuthSession("sid", "csrf", expiry); err != nil {
		t.Fatal(err)
	}
	csrf, gotExpiry, ok, err := store.AuthSession("sid")
	if err != nil || !ok || csrf != "csrf" || !gotExpiry.Equal(expiry) {
		t.Fatalf("csrf=%q expiry=%v ok=%v err=%v", csrf, gotExpiry, ok, err)
	}
	if err := store.PurgeExpiredAuthSessions(expiry.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := store.AuthSession("sid"); err != nil || ok {
		t.Fatalf("purge ok=%v err=%v", ok, err)
	}
}
