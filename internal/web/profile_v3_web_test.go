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
)

func profileV3WebApp(t *testing.T, supported bool) (http.Handler, *http.Cookie, string, *mcpproxy.ProfileV3) {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "admin-key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stored := mcpproxy.ProfileV3{Name: "research", Title: "Research", Servers: []string{"github"}, MaxTier: "read"}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "admin-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"version": "v-next"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/profiles/try":
			if supported {
				w.WriteHeader(http.StatusMethodNotAllowed)
			} else {
				http.NotFound(w, r)
			}
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/clients" || r.URL.Path == "/api/v1/attention" || r.URL.Path == "/api/v1/access/explain"):
			http.NotFound(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/profiles/research":
			if !supported {
				t.Fatal("advanced profile read must not be attempted when capability is absent")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": stored})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/profiles/research":
			if !supported {
				t.Fatal("advanced profile update must not be attempted when capability is absent")
			}
			if err := json.NewDecoder(r.Body).Decode(&stored); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"profile": stored}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/profiles/try":
			if !supported {
				t.Fatal("profile try must not be attempted when capability is absent")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"hidden_by_profile": 3}})
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
	app := (&Server{Cfg: config.Config{AllowedHosts: []string{"mcp.example.com"}}, Auth: authManager, Proxy: proxy}).Handler()
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
	return app, lw.Result().Cookies()[0], authBody.CSRF, &stored
}

func TestProfileV3EndpointsAreCapabilityGated(t *testing.T) {
	app, cookie, _, _ := profileV3WebApp(t, false)
	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/profiles/research/advanced", nil)
	req.AddCookie(cookie)
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusNotFound || !strings.Contains(rw.Body.String(), "not supported") {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
}

func TestProfileV3ReadUpdateAndTryThroughSidekick(t *testing.T) {
	app, cookie, csrf, stored := profileV3WebApp(t, true)

	read := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/profiles/research/advanced", nil)
	read.AddCookie(cookie)
	readW := httptest.NewRecorder()
	app.ServeHTTP(readW, read)
	if readW.Code != http.StatusOK || !strings.Contains(readW.Body.String(), "\"max_tier\":\"read\"") {
		t.Fatalf("read=%d %s", readW.Code, readW.Body.String())
	}

	updateBody := "{\"name\":\"research\",\"title\":\"Research\",\"servers\":[\"github\"],\"max_tier\":\"write\",\"unannotated\":\"deny\"}"
	update := httptest.NewRequest(http.MethodPut, "https://mcp.example.com/api/profiles/research/advanced", strings.NewReader(updateBody))
	update.AddCookie(cookie)
	update.Header.Set("Content-Type", "application/json")
	update.Header.Set("X-CSRF-Token", csrf)
	update.Header.Set("Origin", "https://mcp.example.com")
	updateW := httptest.NewRecorder()
	app.ServeHTTP(updateW, update)
	if updateW.Code != http.StatusNoContent {
		t.Fatalf("update=%d %s", updateW.Code, updateW.Body.String())
	}
	if stored.MaxTier != "write" || stored.Unannotated != "deny" {
		t.Fatalf("stored=%#v", *stored)
	}

	tryReq := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/profiles/try", strings.NewReader(
		"{\"profile\":{\"name\":\"research\",\"servers\":[\"github\"],\"max_tier\":\"write\"},\"query\":\"issue\",\"limit\":10}",
	))
	tryReq.AddCookie(cookie)
	tryReq.Header.Set("Content-Type", "application/json")
	tryReq.Header.Set("X-CSRF-Token", csrf)
	tryReq.Header.Set("Origin", "https://mcp.example.com")
	tryW := httptest.NewRecorder()
	app.ServeHTTP(tryW, tryReq)
	if tryW.Code != http.StatusOK || !strings.Contains(tryW.Body.String(), "\"hidden_by_profile\":3") {
		t.Fatalf("try=%d %s", tryW.Code, tryW.Body.String())
	}
}
