package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/auth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

func newOAuthScopesTestApp(t *testing.T) (http.Handler, *http.Cookie, string, *map[string]any) {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"api_key": "server-secret-admin",
		"mcpServers": map[string]any{
			"google": map[string]any{
				"oauth": map[string]any{
					"client_secret": "client-secret",
					"scopes":        []any{"drive", "gmail"},
				},
			},
		},
	}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "admin-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"version": "v0.69.0"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/config":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "config": doc})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/validate":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "valid": true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/apply":
			var next map[string]any
			if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
				t.Fatalf("decode apply: %v", err)
			}
			doc = next
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fake.Close)

	proxy, err := mcpproxy.NewClient(fake.URL, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	authManager, err := auth.NewManager(keyFile, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	app := (&Server{
		Cfg:   config.Config{AllowedHosts: []string{"mcp.example.com"}},
		Auth:  authManager,
		Proxy: proxy,
	}).Handler()

	login := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString("{\"key\":\"admin-key\"}"))
	login.Header.Set("Content-Type", "application/json")
	lw := httptest.NewRecorder()
	app.ServeHTTP(lw, login)
	if lw.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", lw.Code, lw.Body.String())
	}
	var loginBody struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(lw.Body.Bytes(), &loginBody); err != nil {
		t.Fatal(err)
	}
	return app, lw.Result().Cookies()[0], loginBody.CSRF, &doc
}

func TestOAuthScopesReadDoesNotLeakConfigSecrets(t *testing.T) {
	app, session, _, _ := newOAuthScopesTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/connections/google/oauth-scopes", nil)
	req.AddCookie(session)
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
	body := rw.Body.String()
	if !strings.Contains(body, "\"scopes\":[\"drive\",\"gmail\"]") {
		t.Fatalf("scopes missing: %s", body)
	}
	for _, secret := range []string{"server-secret-admin", "client-secret", "mcpServers"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response leaked %q: %s", secret, body)
		}
	}
}

func TestOAuthScopesPreviewRequiresMutationGuards(t *testing.T) {
	app, session, _, _ := newOAuthScopesTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/connections/google/oauth-scopes/preview", strings.NewReader("{\"scopes\":[\"gmail\"]}"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(session)
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
}

func TestOAuthScopesPreviewAndApply(t *testing.T) {
	app, session, csrf, doc := newOAuthScopesTestApp(t)
	mutate := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("Origin", "https://mcp.example.com")
		req.AddCookie(session)
		rw := httptest.NewRecorder()
		app.ServeHTTP(rw, req)
		return rw
	}

	preview := mutate("/api/connections/google/oauth-scopes/preview", "{\"scopes\":[\"gmail\",\"calendar\",\"calendar\"]}")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "\"reauth_required\":true") {
		t.Fatalf("preview status=%d body=%s", preview.Code, preview.Body.String())
	}

	bad := mutate("/api/connections/google/oauth-scopes/preview", "{\"scopes\":[\"gmail\"],\"secret\":\"nope\"}")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status=%d body=%s", bad.Code, bad.Body.String())
	}

	applied := mutate("/api/connections/google/oauth-scopes/apply", "{\"scopes\":[\"gmail\",\"calendar\"]}")
	if applied.Code != http.StatusOK || !strings.Contains(applied.Body.String(), "\"reauth_required\":true") {
		t.Fatalf("apply status=%d body=%s", applied.Code, applied.Body.String())
	}
	server := (*doc)["mcpServers"].(map[string]any)["google"].(map[string]any)
	oauth := server["oauth"].(map[string]any)
	if !reflect.DeepEqual(oauth["scopes"], []any{"calendar", "gmail"}) && !reflect.DeepEqual(oauth["scopes"], []string{"calendar", "gmail"}) {
		t.Fatalf("scopes=%#v", oauth["scopes"])
	}
	if strings.Contains(applied.Body.String(), "client-secret") {
		t.Fatalf("apply leaked config: %s", applied.Body.String())
	}
}
