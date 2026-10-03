package web

import (
	"bytes"
	"encoding/json"
	"io"
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
)

func eventWebApp(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"version": "v0.69.0"}})
		case r.Method == http.MethodGet && r.URL.Path == "/events":
			if r.Header.Get("X-API-Key") != "admin-key" {
				t.Fatalf("missing upstream API key")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w,
				"event: servers.changed\n"+
					"data: {\"payload\":{\"headers\":{\"Authorization\":\"Bearer DO_NOT_LEAK_SSE_SECRET\"}}}\n\n"+
					"event: ping\ndata: {}\n\n")
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
		Cfg:  config.Config{AllowedHosts: []string{"mcp.example.com"}},
		Auth: authManager, Proxy: proxy,
	}).Handler()
	login := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/login", bytes.NewBufferString("{\"key\":\"admin-key\"}"))
	login.Header.Set("Content-Type", "application/json")
	lw := httptest.NewRecorder()
	app.ServeHTTP(lw, login)
	if lw.Code != http.StatusOK {
		t.Fatalf("login=%d %s", lw.Code, lw.Body.String())
	}
	return app, lw.Result().Cookies()[0]
}

func TestEventStreamRequiresSession(t *testing.T) {
	app, _ := eventWebApp(t)
	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/events", nil)
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
}

func TestEventStreamRelaysOnlyNormalizedInvalidations(t *testing.T) {
	app, cookie := eventWebApp(t)
	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/events", nil)
	req.AddCookie(cookie)
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
	if got := rw.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content type=%q", got)
	}
	body := rw.Body.String()
	if !strings.Contains(body, `"kind":"connections"`) || !strings.Contains(body, `"event":"servers.changed"`) {
		t.Fatalf("normalized event missing: %s", body)
	}
	for _, forbidden := range []string{"DO_NOT_LEAK_SSE_SECRET", "Authorization", "payload", "Bearer"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("upstream payload leaked %q: %s", forbidden, body)
		}
	}
	if strings.Contains(body, "ping") {
		t.Fatalf("heartbeat should not be relayed: %s", body)
	}
	if !strings.Contains(body, `"kind":"fallback"`) || !strings.Contains(body, `"event":"mcpproxy.events.closed"`) {
		t.Fatalf("unexpected upstream EOF must advertise fallback: %s", body)
	}
}
