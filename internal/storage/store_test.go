package storage

import (
	"path/filepath"
	"testing"
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
