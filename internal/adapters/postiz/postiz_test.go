package postiz

import (
	"context"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type fakeEditor struct {
	name    string
	patch   mcpproxy.ServerPatch
	enabled string
}

func (f *fakeEditor) PatchServer(_ context.Context, name string, patch mcpproxy.ServerPatch) error {
	f.name, f.patch = name, patch
	return nil
}
func (f *fakeEditor) EnableServer(_ context.Context, name string) error {
	f.enabled = name
	return nil
}

func TestApplyBuildsURLKeyEndpoint(t *testing.T) {
	f := &fakeEditor{}
	a := Adapter{
		Editor:     f,
		BaseURL:    "https://social.example.com/api/mcp",
		ServerName: "postiz",
	}
	if err := a.Apply(context.Background(), "abc/def"); err != nil {
		t.Fatal(err)
	}
	if got := f.patch.URL; got != "https://social.example.com/api/mcp/abc%2Fdef" {
		t.Fatalf("URL = %q", got)
	}
	if f.enabled != "postiz" {
		t.Fatalf("enabled = %q", f.enabled)
	}
}
