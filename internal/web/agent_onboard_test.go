package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/auth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/tokens"
)

func TestAgentOnboardCreatesProfilePinnedOneTimeCredential(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var created mcpproxy.CreateAgentTokenRequest
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"version": "v0.69.0"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"profiles": []mcpproxy.Profile{{Name: "personal", Servers: []string{"github"}}}}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tokens":
			if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
				"name": created.Name, "token": "mcp_agt_once", "allowed_servers": created.AllowedServers,
				"permissions": created.Permissions, "profile_pin": created.ProfilePin,
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()

	proxy, err := mcpproxy.NewClient(fake.URL, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	authManager, err := auth.NewManager(keyFile, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	app := (&Server{
		Cfg: config.Config{
			AllowedHosts:  []string{"mcp.example.com"},
			PublicBaseURL: "https://mcp.example.com/control",
		},
		Auth: authManager, Proxy: proxy, Tokens: tokens.Service{Backend: proxy},
	}).Handler()

	login := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString("{\"key\":\"admin-key\"}"))
	login.Header.Set("Content-Type", "application/json")
	lw := httptest.NewRecorder()
	app.ServeHTTP(lw, login)
	if lw.Code != http.StatusOK {
		t.Fatalf("login=%d %s", lw.Code, lw.Body.String())
	}
	var authBody struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(lw.Body.Bytes(), &authBody); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/agents/onboard", strings.NewReader(
		"{\"name\":\"archie-standby\",\"profile\":\"personal\",\"permissions\":[\"read\",\"write\"],\"expires_in\":\"30d\",\"target\":\"n8n\"}",
	))
	req.AddCookie(lw.Result().Cookies()[0])
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", authBody.CSRF)
	req.Header.Set("Origin", "https://mcp.example.com")
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusCreated {
		t.Fatalf("onboard=%d %s", rw.Code, rw.Body.String())
	}
	if created.ProfilePin != "personal" || len(created.AllowedServers) != 1 || created.AllowedServers[0] != "*" {
		t.Fatalf("created=%#v", created)
	}
	body := rw.Body.String()
	for _, want := range []string{"mcp_agt_once", "/mcp/p/personal", "archie-standby", "personal", "n8n"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, "admin-key") {
		t.Fatalf("admin key leaked: %s", body)
	}
}

func TestAgentOnboardRejectsUnknownProfileBeforeMinting(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	created := false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"version": "v0.69.0"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": []mcpproxy.Profile{{Name: "personal"}}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tokens":
			created = true
			http.Error(w, "must not mint", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()

	proxy, err := mcpproxy.NewClient(fake.URL, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	authManager, err := auth.NewManager(keyFile, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := authManager.Login("admin-key")
	if err != nil {
		t.Fatal(err)
	}
	app := (&Server{
		Cfg:  config.Config{AllowedHosts: []string{"mcp.example.com"}},
		Auth: authManager, Proxy: proxy, Tokens: tokens.Service{Backend: proxy},
	}).Handler()
	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/agents/onboard", strings.NewReader(
		"{\"name\":\"bad\",\"profile\":\"missing\",\"permissions\":[\"read\"]}",
	))
	req.AddCookie(&http.Cookie{Name: "sidekick_session", Value: sess.ID})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	req.Header.Set("Origin", "https://mcp.example.com")
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
	if created {
		t.Fatal("token was minted for unknown profile")
	}
}
