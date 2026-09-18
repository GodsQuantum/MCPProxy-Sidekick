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
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PostizBaseURL == "" || cfg.PaperlessEndpoint == "" || cfg.ImmichKeyDir == "" || cfg.OmniRouteDB == "" {
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
