package credentials

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDescribeCredentialsNoLeak(t *testing.T) {
	cfg := map[string]any{"mcpServers": []any{
		map[string]any{"name": "ebay-api", "protocol": "stdio", "args": []any{"-e", "EBAY_CLIENT_ID", "-e", "EBAY_CLIENT_SECRET"}},
		map[string]any{"name": "browser-sandbox-demo", "protocol": "stdio", "args": []any{"-e", "AGENT_BROWSER_MAX_OUTPUT=50000"}},
		map[string]any{"name": "my-api", "protocol": "http", "headers": map[string]any{"X-API-Key": "SUPER_SECRET_NEVER_RETURN"}},
		map[string]any{"name": "remote-oauth", "protocol": "http", "oauth": map[string]any{"client_secret": "VERY_SECRET"}},
	}}
	ebay := Describe("ebay-api", cfg)
	if ebay.Kind != "env" || len(ebay.Fields) != 2 || ebay.Fields[0].Name != "EBAY_CLIENT_ID" || ebay.Fields[1].Name != "EBAY_CLIENT_SECRET" {
		t.Fatalf("ebay %+v", ebay)
	}
	other := []FormSchema{ebay, Describe("browser-sandbox-demo", cfg), Describe("my-api", cfg), Describe("remote-oauth", cfg)}
	if other[1].Kind != "none" || other[2].DefaultMode != "x-api-key" || other[3].Kind != "oauth" {
		t.Fatalf("types %+v", other)
	}
	for _, form := range other {
		raw, _ := json.Marshal(form)
		for _, secret := range []string{"VERY_SECRET", "SUPER_SECRET_NEVER_RETURN", "client_secret"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("leak %s", raw)
			}
		}
	}
}
