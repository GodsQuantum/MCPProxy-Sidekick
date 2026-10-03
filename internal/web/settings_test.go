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
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
	if strings.Contains(body, "vault.example.test") {
		t.Fatalf("vault URL should not be returned: %s", body)
	}
}
