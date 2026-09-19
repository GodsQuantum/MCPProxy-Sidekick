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

func newProfileTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	key := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(key, []byte("admin-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(srv.URL, key)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestListProfilesDecodesMCPProxyV067Envelope(t *testing.T) {
	c := newProfileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/profiles" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"profiles": []Profile{{Name: "project-full", Servers: []string{"n8n"}, ToolCount: 39}},
			},
		})
	})
	got, err := c.ListProfiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "project-full" || got[0].ToolCount != 39 {
		t.Fatalf("profiles=%#v", got)
	}
}

func TestCreateProfilePatchesNativeMCPProxyProfiles(t *testing.T) {
	var patched []Profile
	c := newProfileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"profiles": []Profile{{Name: "web", Servers: []string{"searxng"}}},
			}})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/config":
			var body struct {
				Profiles []Profile `json:"profiles"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			patched = body.Profiles
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			http.NotFound(w, r)
		}
	})
	if err := c.CreateProfile(context.Background(), "coding"); err != nil {
		t.Fatal(err)
	}
	if len(patched) != 2 || patched[0].Name != "web" || patched[1].Name != "coding" {
		t.Fatalf("patched=%#v", patched)
	}
}

func TestAssignProfileServerValidatesUpstream(t *testing.T) {
	var patched []Profile
	c := newProfileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"profiles": []Profile{{Name: "coding", Servers: []string{"github"}}},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/servers":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"servers": []Server{{Name: "github"}, {Name: "filesystem"}},
			}})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/config":
			var body struct {
				Profiles []Profile `json:"profiles"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			patched = body.Profiles
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			http.NotFound(w, r)
		}
	})
	if err := c.AssignProfileServer(context.Background(), "coding", "filesystem"); err != nil {
		t.Fatal(err)
	}
	if len(patched) != 1 || len(patched[0].Servers) != 2 || patched[0].Servers[1] != "filesystem" {
		t.Fatalf("patched=%#v", patched)
	}
}

func TestProfileNameMatchesMCPProxyRules(t *testing.T) {
	for _, ok := range []string{"a", "project-full", "family_1"} {
		if err := ValidateProfileName(ok); err != nil {
			t.Fatalf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Upper", "has space", "all", "code", "call", "p"} {
		if err := ValidateProfileName(bad); err == nil {
			t.Fatalf("%q unexpectedly valid", bad)
		}
	}
}
