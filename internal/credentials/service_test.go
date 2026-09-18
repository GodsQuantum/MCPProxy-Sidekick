package credentials

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

func TestSetGenericBearer(t *testing.T) {
	f := &fakeEditor{}
	s := Service{Editor: f}
	if err := s.SetGeneric(context.Background(), "github", "bearer", "", "token-value"); err != nil {
		t.Fatal(err)
	}
	if f.patch.Headers["Authorization"] != "Bearer token-value" {
		t.Fatalf("patch = %#v", f.patch)
	}
	if f.enabled != "github" {
		t.Fatalf("enabled = %q", f.enabled)
	}
}

func TestSetGenericCustomHeaderRequiresName(t *testing.T) {
	s := Service{Editor: &fakeEditor{}}
	if err := s.SetGeneric(context.Background(), "x", "custom-header", "", "value"); err == nil {
		t.Fatal("expected missing header name to fail")
	}
}
