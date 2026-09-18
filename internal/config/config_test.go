package config

import "testing"

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
