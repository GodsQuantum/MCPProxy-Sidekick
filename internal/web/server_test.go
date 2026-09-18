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
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/profiles"
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
	store, err := profiles.Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	app := &Server{
		Cfg:  config.Config{AllowedHosts: []string{"mcp.example.com"}},
		Auth: authManager, Proxy: proxy, Profiles: store,
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
