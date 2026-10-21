package web

import (
	"os"
	"strings"
	"testing"
)

func TestNativeOAuthPageURL(t *testing.T) {
	got, err := nativeOAuthPageURL("https://mcp.example.org/control/", "moneymatter-fella-oauth")
	if err != nil {
		t.Fatal(err)
	}
	const want = "https://mcp.example.org/ui/servers/moneymatter-fella-oauth"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNativeOAuthPageURLRejectsInvalidOriginsAndServers(t *testing.T) {
	for _, s := range []string{"http://mcp.example.org/control/", "javascript:alert(1)", "https://name:password@mcp.example.org/control/", "https://mcp.example.org/control/?token=secret"} {
		if _, err := nativeOAuthPageURL(s, "foo"); err == nil {
			t.Errorf("unsafe origin accepted: %q", s)
		}
	}
	for _, name := range []string{"", "../admin", "evil/../name", "bad?foo", "name\nextra"} {
		_, err := nativeOAuthPageURL("https://mcp.example.org/control/", name)
		if err == nil {
			t.Errorf("unsafe server name accepted: %q", name)
		}
	}
}

func TestOAuthStartUsesNativeMCPProxyUI(t *testing.T) {
	// Regression: sidekick must not reapply the entire mcpServers document or
	// start a competing loopback OAuth flow in the ordinary upstream handler.
	// The backend instead opens MCPProxy's server-detail page.
	src, err := os.ReadFile("handlers.go")
	if err != nil {
		t.Fatal(err)
	}
	v := string(src)
	start := strings.Index(v, "func (s *Server) handleOAuthStart")
	end := strings.Index(v[start:], "func (s *Server) profileV3Available")
	if start < 0 || end < 0 {
		t.Fatal("OAuth handler missing")
	}
	handler := v[start : start+end]
	if strings.Contains(handler, "EnsureCallbackBaseURL") || strings.Contains(handler, "EnsureRedirectURI") || strings.Contains(handler, "StartWithBrowser") {
		t.Fatal("OAuth handler still contains forced configuration changes or competing OAuth start")
	}
	if !strings.Contains(handler, "nativeOAuthPageURL") {
		t.Fatal("native login missing")
	}
}
