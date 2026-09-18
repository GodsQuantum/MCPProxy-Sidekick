package immich

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type fakeEditor struct{ enabled string }

func (f *fakeEditor) PatchServer(context.Context, string, mcpproxy.ServerPatch) error { return nil }
func (f *fakeEditor) EnableServer(_ context.Context, name string) error {
	f.enabled = name
	return nil
}

func TestApplyReplacesExistingKeyWithoutWritingParentDirectory(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "api-key")
	if err := os.WriteFile(keyPath, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	f := &fakeEditor{}
	a := Adapter{Editor: f, KeyFile: keyPath, ServerName: "immich-profile-a"}
	if err := a.Apply(context.Background(), "new-secret"); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	got, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-secret\n" {
		t.Fatalf("key content = %q", string(got))
	}
	if f.enabled != "immich-profile-a" {
		t.Fatalf("enabled = %q", f.enabled)
	}
}
