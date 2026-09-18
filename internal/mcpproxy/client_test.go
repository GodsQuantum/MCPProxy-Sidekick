package mcpproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeAdminKey(t *testing.T, key string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(p, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListServersUsesAdminKeyAndDecodesArray(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		if r.URL.Path != "/api/v1/servers" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]Server{{
			Name:      "github",
			Enabled:   true,
			Status:    "ready",
			ToolCount: 94,
		}})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, writeAdminKey(t, "admin-secret"))
	if err != nil {
		t.Fatal(err)
	}
	servers, err := c.ListServers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "admin-secret" {
		t.Fatalf("X-API-Key = %q", gotKey)
	}
	if len(servers) != 1 || servers[0].Name != "github" || servers[0].ToolCount != 94 {
		t.Fatalf("servers = %#v", servers)
	}
}

func TestListServersDecodesWrappedServers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"servers": []Server{{Name: "notion", Status: "pending_auth"}},
		})
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, writeAdminKey(t, "k"))
	if err != nil {
		t.Fatal(err)
	}
	servers, err := c.ListServers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != "notion" {
		t.Fatalf("servers = %#v", servers)
	}
}

func TestPatchServerDoesNotRequireReadingResponseBody(t *testing.T) {
	var gotMethod string
	var got ServerPatch
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, writeAdminKey(t, "k"))
	if err != nil {
		t.Fatal(err)
	}
	err = c.PatchServer(context.Background(), "github", ServerPatch{
		Headers: map[string]string{"Authorization": "Bearer redacted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q", gotMethod)
	}
	if got.Headers["Authorization"] != "Bearer redacted" {
		t.Fatalf("patch = %#v", got)
	}
}

func TestStartOAuthReturnsAuthURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":        true,
			"auth_url":       "https://provider.example/authorize",
			"correlation_id": "abc",
		})
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, writeAdminKey(t, "k"))
	if err != nil {
		t.Fatal(err)
	}
	start, err := c.StartOAuth(context.Background(), "canva")
	if err != nil {
		t.Fatal(err)
	}
	if start.AuthURL != "https://provider.example/authorize" {
		t.Fatalf("AuthURL = %q", start.AuthURL)
	}
}
