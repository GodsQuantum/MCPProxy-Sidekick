package connections

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/storage"
)

func TestBuildNormalizesConnectionWithoutLeakingSecrets(t *testing.T) {
	server := mcpproxy.Server{
		Name:          "github",
		Enabled:       true,
		Status:        "ready",
		Protocol:      "streamable-http",
		URL:           "https://u:p@[2001:db8::1]/api/mcp/path-secret?token=query-secret#fragment-secret",
		ToolCount:     94,
		Authenticated: true,
		Headers: map[string]string{
			"Authorization": "Bearer super-secret-token",
			"X-API-Key":     "${keyring:github}",
		},
		OAuth: map[string]any{
			"scopes": []any{"repo", "read:user"},
		},
	}
	profiles := []mcpproxy.Profile{
		{Name: "archie", Servers: []string{"github"}},
		{Name: "coding", Servers: []string{"github"}},
	}
	meta := storage.CredentialMeta{ServerName: "github", MaskedPreview: "supe••••oken"}

	got := Build(server, profiles, meta, true)
	if got.Name != "github" || got.ToolCount != 94 || !got.Ready {
		t.Fatalf("detail=%#v", got)
	}
	if got.CredentialPreview != "supe••••oken" {
		t.Fatalf("preview=%q", got.CredentialPreview)
	}
	if got.URL != "https://[2001:db8::1]/…" {
		t.Fatalf("safe endpoint=%q", got.URL)
	}
	if strings.Join(got.Profiles, ",") != "archie,coding" {
		t.Fatalf("profiles=%v", got.Profiles)
	}
	if strings.Join(got.OAuthScopes, ",") != "read:user,repo" {
		t.Fatalf("scopes=%v", got.OAuthScopes)
	}
	if !contains(got.Actions, "reconnect") || !contains(got.Actions, "disable") {
		t.Fatalf("actions=%v", got.Actions)
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, secret := range []string{"super-secret-token", "${keyring:github}", "u:p@", "path-secret", "query-secret", "fragment-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("secret leaked in detail: %s", body)
		}
	}
}

func TestBuildQuarantinedConnectionShowsSafeAction(t *testing.T) {
	got := Build(mcpproxy.Server{
		Name: "new-mcp", Enabled: false, Status: "quarantined", Quarantined: true,
	}, nil, storage.CredentialMeta{}, false)
	if got.Ready || !got.Quarantined || !contains(got.Actions, "enable") {
		t.Fatalf("detail=%#v", got)
	}
}

func TestBuildManagedReferenceDoesNotExposeReference(t *testing.T) {
	got := Build(mcpproxy.Server{
		Name: "api",
		Headers: map[string]string{
			"Authorization": "Bearer ${env:API_TOKEN}",
		},
	}, nil, storage.CredentialMeta{}, false)
	if !got.CredentialConfigured || got.CredentialPreview != "Managed reference" {
		t.Fatalf("detail=%#v", got)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "API_TOKEN") {
		t.Fatalf("managed reference leaked: %s", raw)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
