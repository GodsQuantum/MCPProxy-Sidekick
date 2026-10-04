package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
)

func TestDemoStateReturnsSafeBrowserSettings(t *testing.T) {
	app := (&Server{Cfg: config.Config{
		DemoMode:         true,
		BrowserProvider:  "brave",
		BitwardenMode:    "managed-extension",
		BitwardenBaseURL: "https://vault.example.test",
		BrowserInstance:  "playwright-primary",
		BrowserInstances: []config.BrowserInstance{{ID: "playwright-primary", Label: "Primary", CDPURL: "http://secret-cdp:9222", BrowserURL: "/control/oauth-browser/"}},
	}}).Handler()
	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api/state", nil)
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rw.Code, rw.Body.String())
	}
	body := rw.Body.String()
	for _, want := range []string{
		"\"browser_provider\":\"brave\"",
		"\"bitwarden_mode\":\"managed-extension\"",
		"\"bitwarden_base_url_configured\":true",
		"\"browser_switch_host_side\":true",
		"\"browser_instance\":\"playwright-primary\"",
		"\"browser_open_supported\":true",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
	if strings.Contains(body, "vault.example.test") || strings.Contains(body, "secret-cdp") {
		t.Fatalf("sensitive browser/vault URLs should not be returned: %s", body)
	}
}
