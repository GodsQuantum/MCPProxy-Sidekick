package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr           string
	MCPProxyBaseURL      string
	MCPProxyAdminKeyFile string
	PublicBaseURL        string
	DBPath               string
	AllowedHosts         []string
	SessionLifetime      time.Duration
	OAuthCDPURL          string
	OAuthBrowserURL      string
	PostizBaseURL        string
	PaperlessEndpoint    string
	ImmichKeyDir         string
	DemoMode             bool
}

func Load() (Config, error) {
	lifetime, err := time.ParseDuration(envOr("SIDEKICK_SESSION_LIFETIME", "720h"))
	if err != nil {
		return Config{}, errors.New("invalid SIDEKICK_SESSION_LIFETIME")
	}
	cfg := Config{
		ListenAddr:           envOr("SIDEKICK_LISTEN_ADDR", ":8081"),
		MCPProxyBaseURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("SIDEKICK_MCPPROXY_URL")), "/"),
		MCPProxyAdminKeyFile: strings.TrimSpace(os.Getenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE")),
		PublicBaseURL:        strings.TrimRight(strings.TrimSpace(os.Getenv("SIDEKICK_PUBLIC_BASE_URL")), "/"),
		DBPath:               envOr("SIDEKICK_DB_PATH", "/data/sidekick.db"),
		AllowedHosts:         splitCSV(os.Getenv("SIDEKICK_ALLOWED_HOSTS")),
		SessionLifetime:      lifetime,
		OAuthCDPURL:          envOr("SIDEKICK_OAUTH_CDP_URL", "http://127.0.0.1:9222"),
		OAuthBrowserURL:      envOr("SIDEKICK_OAUTH_BROWSER_URL", "/oauth-browser/"),
		DemoMode:             parseBool(os.Getenv("SIDEKICK_DEMO_MODE")),
	}
	if cfg.DemoMode {
		if cfg.MCPProxyBaseURL == "" {
			cfg.MCPProxyBaseURL = "http://demo.invalid"
		}
		if cfg.MCPProxyAdminKeyFile == "" {
			cfg.MCPProxyAdminKeyFile = "/run/secrets/mcpproxy_admin_key"
		}
		return cfg, nil
	}
	if cfg.MCPProxyBaseURL == "" {
		return Config{}, errors.New("SIDEKICK_MCPPROXY_URL is required")
	}
	if cfg.MCPProxyAdminKeyFile == "" {
		return Config{}, errors.New("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
func splitCSV(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
func parseBool(v string) bool { b, _ := strconv.ParseBool(strings.TrimSpace(v)); return b }
