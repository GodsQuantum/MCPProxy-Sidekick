package paperless

import (
	"context"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type fakeEditor struct {
	patches map[string]mcpproxy.ServerPatch
	enabled []string
}

func (f *fakeEditor) PatchServer(_ context.Context, name string, patch mcpproxy.ServerPatch) error {
	if f.patches == nil {
		f.patches = map[string]mcpproxy.ServerPatch{}
	}
	f.patches[name] = patch
	return nil
}
func (f *fakeEditor) EnableServer(_ context.Context, name string) error {
	f.enabled = append(f.enabled, name)
	return nil
}

func TestApplyUsesDistinctBearerTokenPerAlias(t *testing.T) {
	f := &fakeEditor{}
	a := Adapter{Editor: f, Endpoint: "http://paperless-mcp:3000/mcp"}
	if err := a.Apply(context.Background(), "paperless-profile-a", "token-a"); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(context.Background(), "paperless-profile-b", "token-b"); err != nil {
		t.Fatal(err)
	}
	if got := f.patches["paperless-profile-a"].Headers["Authorization"]; got != "Bearer token-a" {
		t.Fatalf("profile-a header = %q", got)
	}
	if got := f.patches["paperless-profile-b"].Headers["Authorization"]; got != "Bearer token-b" {
		t.Fatalf("profile-b header = %q", got)
	}
	if f.patches["paperless-profile-a"].URL != "http://paperless-mcp:3000/mcp" {
		t.Fatalf("profile-a URL = %q", f.patches["paperless-profile-a"].URL)
	}
}
