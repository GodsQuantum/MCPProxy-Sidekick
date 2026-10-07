package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsMissingMCPProxyURL(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing MCPProxy URL to fail")
	}
}

func TestLoadRejectsMissingAdminKeyFile(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing MCPProxy admin key file to fail")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_LISTEN_ADDR", "")
	t.Setenv("SIDEKICK_DB_PATH", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ListenAddr != ":8081" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.DBPath != "/data/sidekick.db" {
		t.Fatalf("DBPath = %q", cfg.DBPath)
	}
}
func TestLoadMountPath(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_PUBLIC_BASE_URL", "https://mcp.example.com/control/")
	t.Setenv("SIDEKICK_MOUNT_PATH", "/command/")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MountPath != "/command" {
		t.Fatalf("MountPath = %q", cfg.MountPath)
	}
}

func TestLoadReadsAdapterSettings(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_POSTIZ_BASE_URL", "https://social.example.com/api/mcp")
	t.Setenv("SIDEKICK_PAPERLESS_ENDPOINT", "http://paperless-mcp:3000/mcp")
	t.Setenv("SIDEKICK_IMMICH_KEY_DIR", "/run/immich-keys")
	t.Setenv("SIDEKICK_OMNIROUTE_DB", "/run/omniroute/storage.sqlite")
	t.Setenv("SIDEKICK_OPENALEX_KEY_FILE", "/run/openalex/openalex_api_key")
	t.Setenv("SIDEKICK_YOUTUBE_OAUTH_CONTROL_URL", "http://127.0.0.1:8767/")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PostizBaseURL == "" || cfg.PaperlessEndpoint == "" || cfg.ImmichKeyDir == "" || cfg.OmniRouteDB == "" || cfg.OpenAlexKeyFile != "/run/openalex/openalex_api_key" || cfg.YouTubeOAuthControlURL != "http://127.0.0.1:8767" {
		t.Fatalf("adapter settings missing: %#v", cfg)
	}
}
func TestLoadCanReadAdminKeyFromMCPProxyConfig(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "")
	p := filepath.Join(t.TempDir(), "mcp_config.json")
	if err := os.WriteFile(p, []byte(`{"api_key":"config-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SIDEKICK_MCPPROXY_CONFIG_FILE", p)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPProxyAdminKey != "config-secret" {
		t.Fatal("admin key was not loaded from config")
	}
}

func TestOAuthBrowserDefaultsUnderMountPath(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_MOUNT_PATH", "/control")
	t.Setenv("SIDEKICK_OAUTH_BROWSER_URL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OAuthBrowserURL != "/control/oauth-browser/" {
		t.Fatalf("OAuthBrowserURL=%q", cfg.OAuthBrowserURL)
	}
}

func TestLoadBrowserProviderAndBitwardenDefaults(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_BROWSER_PROVIDER", "")
	t.Setenv("SIDEKICK_BITWARDEN_MODE", "")
	t.Setenv("SIDEKICK_BITWARDEN_BASE_URL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BrowserProvider != "chromium" {
		t.Fatalf("BrowserProvider=%q", cfg.BrowserProvider)
	}
	if cfg.BitwardenMode != "off" {
		t.Fatalf("BitwardenMode=%q", cfg.BitwardenMode)
	}
	if cfg.BitwardenBaseURL != "" {
		t.Fatalf("BitwardenBaseURL=%q", cfg.BitwardenBaseURL)
	}
}

func TestLoadRejectsUnknownBrowserProvider(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_BROWSER_PROVIDER", "firefox")
	if _, err := Load(); err == nil {
		t.Fatal("expected unknown browser provider to fail")
	}
}

func TestLoadRejectsUnknownBitwardenMode(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_BROWSER_PROVIDER", "brave")
	t.Setenv("SIDEKICK_BITWARDEN_MODE", "magic")
	if _, err := Load(); err == nil {
		t.Fatal("expected unknown Bitwarden mode to fail")
	}
}

func TestLoadReadsBraveAndManagedBitwarden(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_BROWSER_PROVIDER", "brave")
	t.Setenv("SIDEKICK_BITWARDEN_MODE", "managed-extension")
	t.Setenv("SIDEKICK_BITWARDEN_BASE_URL", "https://vault.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BrowserProvider != "brave" || cfg.BitwardenMode != "managed-extension" || cfg.BitwardenBaseURL != "https://vault.example.com" {
		t.Fatalf("browser settings=%#v", cfg)
	}
}

func TestLoadBrowserInstances(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_BROWSER_INSTANCE", "cloak-primary")
	t.Setenv("SIDEKICK_BROWSER_INSTANCES_JSON", `[{"id":"cloak-primary","label":"Primary","cdp_url":"http://manager:8080/api/profiles/a/cdp","browser_url":"https://browser.example.com/","launch_url":"http://manager:8080/api/profiles/a/launch"},{"id":"cloak-secondary","label":"Secondary","cdp_url":"http://manager:8080/api/profiles/b/cdp","browser_url":"https://browser.example.com/","launch_url":"http://manager:8080/api/profiles/b/launch"}]`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BrowserInstance != "cloak-primary" || len(cfg.BrowserInstances) != 2 {
		t.Fatalf("browser registry=%#v selected=%q", cfg.BrowserInstances, cfg.BrowserInstance)
	}
	if cfg.OAuthCDPURL != "http://manager:8080/api/profiles/a/cdp" || cfg.OAuthBrowserURL != "https://browser.example.com/" {
		t.Fatalf("selected browser not applied: %#v", cfg)
	}
	if cfg.BrowserInstances[0].LaunchURL != "http://manager:8080/api/profiles/a/launch" {
		t.Fatalf("launch URL not applied: %#v", cfg.BrowserInstances[0])
	}
}

func TestLoadRejectsUnknownBrowserInstance(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_BROWSER_INSTANCE", "missing")
	t.Setenv("SIDEKICK_BROWSER_INSTANCES_JSON", `[{"id":"playwright-primary","cdp_url":"http://primary:9222","browser_url":"/control/primary/"}]`)
	if _, err := Load(); err == nil {
		t.Fatal("expected unknown browser instance to fail")
	}
}

func TestLoadDifyBindings(t *testing.T) {
	t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
	t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
	t.Setenv("SIDEKICK_DIFY_BINDINGS_JSON", `{"research":{"provider_id":"provider-read","permissions":["READ"]},"actions":{"provider_id":" provider-actions ","permissions":["read","write"]}}`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.DifyBindings["research"]; got.ProviderID != "provider-read" || len(got.Permissions) != 1 || got.Permissions[0] != "read" {
		t.Fatalf("research binding=%#v", got)
	}
	if got := cfg.DifyBindings["actions"]; got.ProviderID != "provider-actions" || len(got.Permissions) != 2 || got.Permissions[0] != "read" || got.Permissions[1] != "write" {
		t.Fatalf("actions binding=%#v", got)
	}
}

func TestLoadRejectsInvalidDifyBindings(t *testing.T) {
	for name, raw := range map[string]string{
		"invalid json":         `{`,
		"missing permissions":  `{"profile":{"provider_id":"provider","permissions":[]}}`,
		"invalid permission":   `{"profile":{"provider_id":"provider","permissions":["admin"]}}`,
		"duplicate permission": `{"profile":{"provider_id":"provider","permissions":["read","READ"]}}`,
		"normalized duplicate": `{"profile":{"provider_id":"one","permissions":["read"]}," profile ":{"provider_id":"two","permissions":["read"]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("SIDEKICK_MCPPROXY_URL", "http://mcpproxy:8080")
			t.Setenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE", "/run/secrets/mcpproxy_admin_key")
			t.Setenv("SIDEKICK_DIFY_BINDINGS_JSON", raw)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid Dify bindings to fail")
			}
		})
	}
}
