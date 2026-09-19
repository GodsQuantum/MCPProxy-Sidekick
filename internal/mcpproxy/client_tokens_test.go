package mcpproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAgentTokensDecodesCurrentMCPProxyEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"tokens": []AgentToken{{Name: "openwebui-primary"}},
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, writeAdminKey(t, "k"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.ListAgentTokens(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "openwebui-primary" {
		t.Fatalf("tokens = %#v", got)
	}
}

func TestCreateAgentTokenDecodesCurrentMCPProxyEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"name": "dify-primary", "token": "mcp_agt_once"},
		})
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, writeAdminKey(t, "k"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.CreateAgentToken(context.Background(), CreateAgentTokenRequest{Name: "dify-primary"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "dify-primary" || got.Token != "mcp_agt_once" {
		t.Fatalf("created=%#v", got)
	}
}

func TestRegenerateAgentTokenDecodesCurrentMCPProxyEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"name": "cli-primary", "token": "mcp_agt_rotated"},
		})
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL, writeAdminKey(t, "k"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.RegenerateAgentToken(context.Background(), "cli-primary")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "cli-primary" || got.Token != "mcp_agt_rotated" {
		t.Fatalf("regenerated=%#v", got)
	}
}
