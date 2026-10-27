package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCredentialSchemaAndEnvironmentPatch(t *testing.T) {
	patches := 0
	enables := 0
	var env map[string]string
	h := newConnectionHarness(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/config":
			_ = json.NewEncoder(w).Encode(map[string]any{"mcpServers": []any{
				map[string]any{"name": "ebay-api", "protocol": "stdio", "args": []any{"docker", "run", "-e", "EBAY_CLIENT_ID", "-e", "EBAY_CLIENT_SECRET"}},
				map[string]any{"name": "browser-sandbox-demo", "protocol": "stdio", "args": []any{"docker", "exec"}},
				map[string]any{"name": "oauth-test", "protocol": "http", "oauth": map[string]any{"secret": "NOT_FOR_BROWSER"}},
				map[string]any{"name": "github", "protocol": "http", "headers": map[string]string{"Authorization": "Bearer NOT_FOR_BROWSER"}},
			}})
		case r.Method == "PATCH" && r.URL.Path == "/api/v1/servers/ebay-api":
			patches++
			var body struct {
				Env map[string]string `json:"env"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			env = body.Env
			w.WriteHeader(200)
		case r.Method == "POST" && r.URL.Path == "/api/v1/servers/ebay-api/enable":
			enables++
			w.WriteHeader(200)
		default:
			http.NotFound(w, r)
		}
	})
	request := func(method, path, payload string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "https://mcp.example.com"+path, strings.NewReader(payload))
		req.AddCookie(h.cookie)
		if method != "GET" {
			req.Header.Set("Origin", "https://mcp.example.com")
			req.Header.Set("X-CSRF-Token", h.csrf)
			req.Header.Set("Content-Type", "application/json")
		}
		rw := httptest.NewRecorder()
		h.handler.ServeHTTP(rw, req)
		return rw
	}
	schema := request("GET", "/api/upstreams/ebay-api/credential/schema", "")
	if schema.Code != 200 || !strings.Contains(schema.Body.String(), "EBAY_CLIENT_SECRET") || strings.Contains(schema.Body.String(), "NOT_FOR_BROWSER") {
		t.Fatalf("wrong eBay schema %d", schema.Code)
	}
	browser := request("GET", "/api/upstreams/browser-sandbox-demo/credential/schema", "")
	if browser.Code != 200 || !strings.Contains(browser.Body.String(), `"kind":"none"`) {
		t.Fatalf("wrong browser schema %d", browser.Code)
	}
	oauth := request("GET", "/api/upstreams/oauth-test/credential/schema", "")
	if oauth.Code != 200 || !strings.Contains(oauth.Body.String(), `"kind":"oauth"`) || strings.Contains(oauth.Body.String(), "NOT_FOR_BROWSER") {
		t.Fatalf("wrong OAuth schema %d", oauth.Code)
	}
	invalid := request("POST", "/api/upstreams/ebay-api/credential", `{"mode":"stdio-env","values":{"EBAY_CLIENT_ID":"public-test"}}`)
	if invalid.Code != 400 || patches != 0 {
		t.Fatalf("incomplete pair accepted %d", invalid.Code)
	}
	wrongMode := request("POST", "/api/upstreams/ebay-api/credential", `{"mode":"bearer","value":"bad-choice"}`)
	if wrongMode.Code != 400 || patches != 0 {
		t.Fatalf("header used on stdio %d", wrongMode.Code)
	}
	success := request("POST", "/api/upstreams/ebay-api/credential", `{"mode":"stdio-env","values":{"EBAY_CLIENT_ID":"public-test","EBAY_CLIENT_SECRET":"test-private-secret"}}`)
	if success.Code != 200 || patches != 1 || enables != 1 {
		t.Fatalf("patch failed %d patches=%d enables=%d", success.Code, patches, enables)
	}
	if env["EBAY_CLIENT_ID"] != "public-test" || env["EBAY_CLIENT_SECRET"] != "test-private-secret" {
		t.Fatalf("wrong fields forwarded")
	}
	if strings.Contains(success.Body.String(), "test-private-secret") || strings.Contains(success.Body.String(), "public-test") {
		t.Fatalf("credential leaked in response")
	}
}
