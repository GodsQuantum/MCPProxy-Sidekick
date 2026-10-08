package mcpproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestConfigReadValidateApplyUsesOfficialEndpoints(t *testing.T) {
	var validated, applied bool
	client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/config":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"mcpServers": map[string]any{
					"google": map[string]any{"oauth": map[string]any{"scopes": []any{"a"}}},
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/validate":
			validated = true
			_ = json.NewEncoder(w).Encode(map[string]any{"valid": true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/apply":
			applied = true
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			http.NotFound(w, r)
		}
	})
	doc, err := client.GetConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["mcpServers"]; !ok {
		t.Fatalf("doc=%#v", doc)
	}
	if err := client.ValidateConfig(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	if err := client.ApplyConfig(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	if !validated || !applied {
		t.Fatalf("validated=%v applied=%v", validated, applied)
	}
}

func TestDecodeConfigDocumentUnwrapsNestedDataConfig(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"config": map[string]any{
				"mcpServers": []any{
					map[string]any{"name": "google", "oauth": map[string]any{}},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := decodeConfigDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["mcpServers"]; !ok {
		t.Fatalf("doc=%#v", doc)
	}
}
