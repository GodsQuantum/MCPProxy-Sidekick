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
		map[string]any{"name": "new-http", "protocol": "streamable-http", "url": "https://mcp.example.com/mcp"},
		map[string]any{"name": "new-stdio", "protocol": "stdio", "args": []any{"-e", "SERVICE_API_KEY", "-e", "SERVICE_CLIENT_ID", "-e", "SERVICE_CLIENT_SECRET"}},
		map[string]any{"name": "http-with-metadata", "protocol": "http", "headers": map[string]any{"User-Agent": "Sidekick"}},
		map[string]any{"name": "api-key-stdio", "protocol": "stdio", "env": map[string]any{"API_KEY": "DONT_LEAK", "PORT": "8080"}},
	}}
	ebay := Describe("ebay-api", cfg)
	if ebay.Kind != "env" || len(ebay.Fields) != 2 || ebay.Fields[0].Name != "EBAY_CLIENT_ID" || ebay.Fields[1].Name != "EBAY_CLIENT_SECRET" {
		t.Fatalf("ebay %+v", ebay)
	}
	other := []FormSchema{ebay, Describe("browser-sandbox-demo", cfg), Describe("my-api", cfg), Describe("remote-oauth", cfg)}
	if other[1].Kind != "none" || other[2].DefaultMode != "x-api-key" || other[3].Kind != "oauth" {
		t.Fatalf("types %+v", other)
	}
	newHTTP := Describe("new-http", cfg)
	if newHTTP.Kind != "undetermined" || newHTTP.Source != "not-published" {
		t.Fatalf("generic bearer guess: %+v", newHTTP)
	}
	if Describe("http-with-metadata", cfg).Kind != "undetermined" {
		t.Fatal("non-auth headers misclassified")
	}
	stdio := Describe("new-stdio", cfg)
	if stdio.Kind != "env" || len(stdio.Fields) != 3 {
		t.Fatalf("stdio detection %+v", stdio)
	}
	literal := Describe("api-key-stdio", cfg)
	if literal.Kind != "env" || len(literal.Fields) != 1 || literal.Fields[0].Name != "API_KEY" {
		t.Fatalf("literal API_KEY not detected %+v", literal)
	}
	other = append(other, newHTTP, stdio, literal)
	for _, form := range other {
		raw, _ := json.Marshal(form)
		for _, secret := range []string{"VERY_SECRET", "SUPER_SECRET_NEVER_RETURN", "client_secret"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("leak %s", raw)
			}
		}
	}
}
