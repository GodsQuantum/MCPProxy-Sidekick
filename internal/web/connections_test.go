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

type connectionHarness struct {
	handler http.Handler
	server  *httptest.Server
	cookie  *http.Cookie
	csrf    string
}

func newConnectionHarness(t *testing.T, upstream http.HandlerFunc) connectionHarness {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/info" {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"version": "v0.69.0"}})
			return
		}
		upstream(w, r)
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
	store, err := storage.Open(filepath.Join(t.TempDir(), "sidekick.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	app := &Server{
		Cfg:         config.Config{AllowedHosts: []string{"mcp.example.com"}},
		Auth:        authManager,
		Proxy:       proxy,
		Store:       store,
		Credentials: credentials.Service{Editor: proxy},
		Tokens:      tokens.Service{Backend: proxy},
	}
	h := app.Handler()
	login := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString(`{"key":"admin-key"}`))
	login.Header.Set("Content-Type", "application/json")
	lw := httptest.NewRecorder()
	h.ServeHTTP(lw, login)
	if lw.Code != http.StatusOK {
		t.Fatalf("login=%d %s", lw.Code, lw.Body.String())
	}
	var body struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(lw.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return connectionHarness{handler: h, server: fake, cookie: lw.Result().Cookies()[0], csrf: body.CSRF}
}

func TestConnectionsEndpointReturnsSafeNormalizedDetail(t *testing.T) {
	h := newConnectionHarness(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/servers":
			_ = json.NewEncoder(w).Encode([]mcpproxy.Server{{
				Name: "github", Enabled: true, Status: "ready", ToolCount: 94,
				Headers: map[string]string{"Authorization": "Bearer super-secret-value"},
			}})
		case "/api/v1/profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{"profiles": []mcpproxy.Profile{{Name: "archie", Servers: []string{"github"}}}})
		default:
			http.NotFound(w, r)
		}
	})
	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/connections/github", nil)
	req.AddCookie(h.cookie)
	rw := httptest.NewRecorder()
	h.handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
	if strings.Contains(rw.Body.String(), "super-secret-value") {
		t.Fatalf("secret leaked: %s", rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "\"name\":\"github\"") {
		t.Fatalf("missing connection: %s", rw.Body.String())
	}
}

func TestConnectionPatchRequiresCSRF(t *testing.T) {
	h := newConnectionHarness(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	req := httptest.NewRequest(http.MethodPatch, "https://mcp.example.com/api/connections/github", strings.NewReader("{\"enabled\":false}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://mcp.example.com")
	req.AddCookie(h.cookie)
	rw := httptest.NewRecorder()
	h.handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
}

func TestConnectionPatchRejectsSecretFieldsAndMapsEnable(t *testing.T) {
	var enabled bool
	h := newConnectionHarness(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/servers/github/enable":
			enabled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	bad := httptest.NewRequest(http.MethodPatch, "https://mcp.example.com/api/connections/github", strings.NewReader("{\"headers\":{\"Authorization\":\"MASKED\"}}"))
	bad.Header.Set("Content-Type", "application/json")
	bad.Header.Set("Origin", "https://mcp.example.com")
	bad.Header.Set("X-CSRF-Token", h.csrf)
	bad.AddCookie(h.cookie)
	bw := httptest.NewRecorder()
	h.handler.ServeHTTP(bw, bad)
	if bw.Code != http.StatusBadRequest {
		t.Fatalf("secret field status=%d body=%s", bw.Code, bw.Body.String())
	}

	ok := httptest.NewRequest(http.MethodPatch, "https://mcp.example.com/api/connections/github", strings.NewReader("{\"action\":\"enable\"}"))
	ok.Header.Set("Content-Type", "application/json")
	ok.Header.Set("Origin", "https://mcp.example.com")
	ok.Header.Set("X-CSRF-Token", h.csrf)
	ok.AddCookie(h.cookie)
	ow := httptest.NewRecorder()
	h.handler.ServeHTTP(ow, ok)
	if ow.Code != http.StatusNoContent || !enabled {
		t.Fatalf("status=%d enabled=%v body=%s", ow.Code, enabled, ow.Body.String())
	}
}
