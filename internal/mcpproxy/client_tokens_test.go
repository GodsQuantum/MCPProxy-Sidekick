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
