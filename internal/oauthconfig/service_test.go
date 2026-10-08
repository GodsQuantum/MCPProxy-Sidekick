package oauthconfig

import (
	"context"
	"reflect"
	"testing"
)

type fakeBackend struct {
	doc       map[string]any
	validated map[string]any
	applied   map[string]any
}

func (f *fakeBackend) GetConfig(context.Context) (map[string]any, error) {
	return cloneMap(f.doc), nil
}
func (f *fakeBackend) ValidateConfig(_ context.Context, doc map[string]any) error {
	f.validated = cloneMap(doc)
	return nil
}
func (f *fakeBackend) ApplyConfig(_ context.Context, doc map[string]any) error {
	f.applied = cloneMap(doc)
	f.doc = cloneMap(doc)
	return nil
}

func configFixture() map[string]any {
	return map[string]any{
		"api_key": "[REDACTED]",
		"mcpServers": map[string]any{
			"google": map[string]any{
				"url": "http://example.invalid/mcp",
				"oauth": map[string]any{
					"client_secret": "[REDACTED]",
					"scopes":        []any{"drive", " gmail ", "drive"},
				},
			},
		},
	}
}

func TestPreviewNormalizesScopeDiff(t *testing.T) {
	b := &fakeBackend{doc: configFixture()}
	s := Service{Backend: b}
	got, err := s.Preview(context.Background(), "google", []string{"calendar", "gmail", "calendar"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Current, []string{"drive", "gmail"}) {
		t.Fatalf("current=%v", got.Current)
	}
	if !reflect.DeepEqual(got.Desired, []string{"calendar", "gmail"}) {
		t.Fatalf("desired=%v", got.Desired)
	}
	if !reflect.DeepEqual(got.Added, []string{"calendar"}) || !reflect.DeepEqual(got.Removed, []string{"drive"}) {
		t.Fatalf("diff=%#v", got)
	}
	if !got.ReauthRequired {
		t.Fatal("scope change must require reauth")
	}
}

func TestPreviewNoopDoesNotRequireReauth(t *testing.T) {
	b := &fakeBackend{doc: configFixture()}
	s := Service{Backend: b}
	got, err := s.Preview(context.Background(), "google", []string{"gmail", "drive"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ReauthRequired || len(got.Added) != 0 || len(got.Removed) != 0 {
		t.Fatalf("noop diff=%#v", got)
	}
}

func TestPreviewRejectsUnknownServer(t *testing.T) {
	s := Service{Backend: &fakeBackend{doc: configFixture()}}
	if _, err := s.Preview(context.Background(), "missing", []string{"gmail"}); err == nil {
		t.Fatal("expected unknown server error")
	}
}

func TestApplyPreservesMaskedSecretsAndVerifiesScopes(t *testing.T) {
	b := &fakeBackend{doc: configFixture()}
	s := Service{Backend: b}
	got, err := s.Apply(context.Background(), "google", []string{"gmail", "calendar"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.ReauthRequired {
		t.Fatal("expected reauth")
	}
	if b.validated == nil || b.applied == nil {
		t.Fatal("validate/apply not called")
	}
	if b.applied["api_key"] != "[REDACTED]" {
		t.Fatalf("api key mask changed: %#v", b.applied["api_key"])
	}
	server := b.applied["mcpServers"].(map[string]any)["google"].(map[string]any)
	oauth := server["oauth"].(map[string]any)
	if oauth["client_secret"] != "[REDACTED]" {
		t.Fatalf("client secret mask changed: %#v", oauth["client_secret"])
	}
	if !reflect.DeepEqual(oauth["scopes"], []string{"calendar", "gmail"}) {
		t.Fatalf("scopes=%#v", oauth["scopes"])
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch x := v.(type) {
		case map[string]any:
			out[k] = cloneMap(x)
		case []any:
			out[k] = append([]any(nil), x...)
		case []string:
			out[k] = append([]string(nil), x...)
		default:
			out[k] = v
		}
	}
	return out
}

func TestEnsureRedirectURIAppliesAndIsIdempotent(t *testing.T) {
	b := &fakeBackend{doc: configFixture()}
	s := Service{Backend: b}
	changed, err := s.EnsureRedirectURI(context.Background(), "google", "http://127.0.0.1:54108/oauth/callback")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected redirect URI change")
	}
	server := b.doc["mcpServers"].(map[string]any)["google"].(map[string]any)
	oauth := server["oauth"].(map[string]any)
	if oauth["redirect_uri"] != "http://127.0.0.1:54108/oauth/callback" {
		t.Fatalf("redirect_uri=%v", oauth["redirect_uri"])
	}
	changed, err = s.EnsureRedirectURI(context.Background(), "google", "http://127.0.0.1:54108/oauth/callback")
	if err != nil || changed {
		t.Fatalf("idempotent changed=%v err=%v", changed, err)
	}
}
