package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	ListenAddr           string
	MCPProxyBaseURL      string
	MCPProxyAdminKeyFile string
	PublicBaseURL        string
	DBPath               string
	AllowedHosts         []string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:           envOr("SIDEKICK_LISTEN_ADDR", ":8081"),
		MCPProxyBaseURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("SIDEKICK_MCPPROXY_URL")), "/"),
		MCPProxyAdminKeyFile: strings.TrimSpace(os.Getenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE")),
		PublicBaseURL:        strings.TrimRight(strings.TrimSpace(os.Getenv("SIDEKICK_PUBLIC_BASE_URL")), "/"),
		DBPath:               envOr("SIDEKICK_DB_PATH", "/data/sidekick.db"),
		AllowedHosts:         splitCSV(os.Getenv("SIDEKICK_ALLOWED_HOSTS")),
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
