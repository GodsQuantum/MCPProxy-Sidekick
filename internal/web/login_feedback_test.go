package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/auth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

func loginTestApp(t *testing.T, upstream http.HandlerFunc) (http.Handler, *int32) {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var hits int32
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
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
	return (&Server{
		Cfg:   config.Config{AllowedHosts: []string{"mcp.example.com"}},
		Auth:  authManager,
		Proxy: proxy,
	}).Handler(), &hits
}

func TestLoginVerifiesMCPProxyBeforeCreatingSession(t *testing.T) {
	app, _ := loginTestApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/info" {
			http.Error(w, "mcpproxy unavailable", http.StatusServiceUnavailable)
			return
		}
		http.NotFound(w, r)
	})
	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString("{\"key\":\"admin-key\"}"))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
	if got := rw.Result().Cookies(); len(got) != 0 {
		t.Fatalf("unexpected session cookie: %#v", got)
	}
	if !strings.Contains(strings.ToLower(rw.Body.String()), "mcpproxy") {
		t.Fatalf("error is not actionable: %s", rw.Body.String())
	}
}

func TestLoginReturnsConnectedMCPProxyVersion(t *testing.T) {
	app, _ := loginTestApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/info" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-API-Key") != "admin-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"version": "v0.69.0"},
		})
	})
	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString("{\"key\":\"admin-key\"}"))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
	if len(rw.Result().Cookies()) != 1 {
		t.Fatalf("cookies=%#v", rw.Result().Cookies())
	}
	if !strings.Contains(rw.Body.String(), "\"mcpproxy_version\":\"v0.69.0\"") {
		t.Fatalf("version missing: %s", rw.Body.String())
	}
}

func TestLoginWrongKeyDoesNotProbeMCPProxy(t *testing.T) {
	app, hits := loginTestApp(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "v0.69.0"})
	})
	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString("{\"key\":\"wrong\"}"))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Fatalf("MCPProxy probed for wrong local key: hits=%d", atomic.LoadInt32(hits))
	}
}
