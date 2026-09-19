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
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/storage"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/tokens"
)

func TestStateNeverReturnsFullUpstreamSecret(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/servers":
			_ = json.NewEncoder(w).Encode([]mcpproxy.Server{{
				Name: "github", Enabled: true, Status: "ready", ToolCount: 94,
				Headers: map[string]string{"Authorization": "Bearer super-secret-value"},
			}})
		case "/api/v1/tokens":
			_ = json.NewEncoder(w).Encode(map[string]any{"tokens": []mcpproxy.AgentToken{}})
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
	store, err := storage.Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	app := &Server{
		Cfg:  config.Config{AllowedHosts: []string{"mcp.example.com"}},
		Auth: authManager, Proxy: proxy, Store: store,
		Credentials: credentials.Service{Editor: proxy},
		Tokens:      tokens.Service{Backend: proxy},
	}
	h := app.Handler()

	login := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString(`{"key":"admin-key"}`))
	login.Header.Set("Content-Type", "application/json")
	lw := httptest.NewRecorder()
	h.ServeHTTP(lw, login)
	if lw.Code != 200 {
		t.Fatalf("login status=%d body=%s", lw.Code, lw.Body.String())
	}
	cookies := lw.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing session cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/state", nil)
	req.AddCookie(cookies[0])
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != 200 {
		t.Fatalf("state status=%d body=%s", rw.Code, rw.Body.String())
	}
	body := rw.Body.String()
	if strings.Contains(body, "super-secret-value") {
		t.Fatalf("full secret leaked: %s", body)
	}
	if !strings.Contains(body, "supe••••alue") {
		t.Fatalf("masked preview missing: %s", body)
	}
}

func TestOAuthBrowserAuthCheckRejectsAnonymous(t *testing.T) {
	app := &Server{Cfg: config.Config{}}
	rw := httptest.NewRecorder()
	app.Handler().ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "https://mcp.example.com/auth/check", nil))
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rw.Code)
	}
}

func TestOAuthBrowserAuthCheckAcceptsPrimarySessionCookie(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
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
	app := &Server{Cfg: config.Config{}, Auth: authManager}
	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/auth/check", nil)
	req.AddCookie(&http.Cookie{Name: "sidekick_session", Value: sess.ID, Path: "/"})
	rw := httptest.NewRecorder()
	app.Handler().ServeHTTP(rw, req)
	if rw.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
}

func TestStateDegradesInsteadOfReturning502(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failServers bool
		failTokens  bool
		wantWarning string
	}{
		{name: "tokens unavailable", failTokens: true, wantWarning: "Agent Tokens are temporarily unavailable"},
		{name: "servers unavailable", failServers: true, wantWarning: "server inventory is temporarily unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keyFile := filepath.Join(t.TempDir(), "admin-key")
			if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/servers":
					if tc.failServers {
						http.Error(w, "boom", http.StatusBadGateway)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"servers": []mcpproxy.Server{}})
				case "/api/v1/tokens":
					if tc.failTokens {
						http.Error(w, "boom", http.StatusBadGateway)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"tokens": []mcpproxy.AgentToken{}})
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
			store, err := storage.Open(filepath.Join(t.TempDir(), "sidekick.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()

			app := &Server{
				Cfg:  config.Config{AllowedHosts: []string{"mcp.example.com"}},
				Auth: authManager, Proxy: proxy, Store: store,
				Credentials: credentials.Service{Editor: proxy},
				Tokens:      tokens.Service{Backend: proxy},
			}
			h := app.Handler()
			login := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString(`{"key":"admin-key"}`))
			login.Header.Set("Content-Type", "application/json")
			lw := httptest.NewRecorder()
			h.ServeHTTP(lw, login)
			if lw.Code != http.StatusOK {
				t.Fatalf("login status=%d body=%s", lw.Code, lw.Body.String())
			}
			req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/state", nil)
			req.AddCookie(lw.Result().Cookies()[0])
			rw := httptest.NewRecorder()
			h.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK {
				t.Fatalf("state status=%d body=%s", rw.Code, rw.Body.String())
			}
			if !strings.Contains(rw.Body.String(), tc.wantWarning) {
				t.Fatalf("missing degraded warning in body: %s", rw.Body.String())
			}
		})
	}
}

func TestNativeProfileCRUDAndPinnedTokenGuard(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	profiles := []mcpproxy.Profile{{Name: "web", Servers: []string{"github"}}}
	tokensState := []mcpproxy.AgentToken{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"profiles": profiles}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/servers":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
				"servers": []mcpproxy.Server{{Name: "github"}, {Name: "filesystem"}},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tokens":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"tokens": tokensState}})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/config":
			var body struct {
				Profiles []mcpproxy.Profile `json:"profiles"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			profiles = append([]mcpproxy.Profile(nil), body.Profiles...)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
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
	store, err := storage.Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	app := &Server{
		Cfg:  config.Config{AllowedHosts: []string{"mcp.example.com"}},
		Auth: authManager, Proxy: proxy, Store: store,
		Credentials: credentials.Service{Editor: proxy},
		Tokens:      tokens.Service{Backend: proxy},
	}
	h := app.Handler()

	login := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString(`{"key":"admin-key"}`))
	login.Header.Set("Content-Type", "application/json")
	lw := httptest.NewRecorder()
	h.ServeHTTP(lw, login)
	if lw.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", lw.Code, lw.Body.String())
	}
	var loginBody struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(lw.Body.Bytes(), &loginBody); err != nil {
		t.Fatal(err)
	}
	session := lw.Result().Cookies()[0]

	mutate := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "https://mcp.example.com"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", loginBody.CSRF)
		req.Header.Set("Origin", "https://mcp.example.com")
		req.AddCookie(session)
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)
		return rw
	}

	if rw := mutate(http.MethodPost, "/api/profiles", `{"name":"coding"}`); rw.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rw.Code, rw.Body.String())
	}
	if len(profiles) != 2 || profiles[1].Name != "coding" {
		t.Fatalf("profiles after create=%#v", profiles)
	}

	if rw := mutate(http.MethodPost, "/api/profiles/coding/servers", `{"server":"filesystem"}`); rw.Code != http.StatusNoContent {
		t.Fatalf("assign status=%d body=%s", rw.Code, rw.Body.String())
	}
	if len(profiles[1].Servers) != 1 || profiles[1].Servers[0] != "filesystem" {
		t.Fatalf("profiles after assign=%#v", profiles)
	}

	if rw := mutate(http.MethodDelete, "/api/profiles/coding/servers/filesystem", `{}`); rw.Code != http.StatusNoContent {
		t.Fatalf("remove status=%d body=%s", rw.Code, rw.Body.String())
	}
	if len(profiles[1].Servers) != 0 {
		t.Fatalf("profiles after remove=%#v", profiles)
	}

	tokensState = []mcpproxy.AgentToken{{Name: "active-pin", ProfilePin: "coding"}}
	if rw := mutate(http.MethodDelete, "/api/profiles/coding", `{}`); rw.Code != http.StatusConflict {
		t.Fatalf("active pinned delete status=%d body=%s", rw.Code, rw.Body.String())
	}
	if len(profiles) != 2 {
		t.Fatalf("active pin unexpectedly deleted profile: %#v", profiles)
	}

	tokensState = []mcpproxy.AgentToken{{Name: "expired-pin", ProfilePin: "coding", ExpiresAt: time.Now().Add(-time.Hour)}}
	if rw := mutate(http.MethodDelete, "/api/profiles/coding", `{}`); rw.Code != http.StatusNoContent {
		t.Fatalf("expired pinned delete status=%d body=%s", rw.Code, rw.Body.String())
	}
	if len(profiles) != 1 || profiles[0].Name != "web" {
		t.Fatalf("profiles after expired-token delete=%#v", profiles)
	}
}
