package profiles

import (
	"path/filepath"
	"testing"
)

func TestProfileAssignmentRoundTrip(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.UpsertProfile(Profile{ID: "profile-a", Label: "Profile A", SortOrder: 10}); err != nil {
		t.Fatal(err)
	}
	if err := store.AssignServer("profile-a", "paperless-profile-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.AssignServer("profile-a", "immich-profile-a"); err != nil {
		t.Fatal(err)
	}

	got, err := store.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "profile-a" {
		t.Fatalf("profiles = %#v", got)
	}
	if len(got[0].Servers) != 2 {
		t.Fatalf("servers = %#v", got[0].Servers)
	}
}

func TestAssignServerRequiresExistingProfile(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.AssignServer("missing", "github"); err == nil {
		t.Fatal("expected foreign-key failure")
	}
}
