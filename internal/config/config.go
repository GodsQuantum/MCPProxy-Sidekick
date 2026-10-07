package config

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/browser"
)

type BrowserInstance struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	CDPURL     string `json:"cdp_url"`
	BrowserURL string `json:"browser_url"`
	LaunchURL  string `json:"launch_url,omitempty"`
}

type DifyBinding struct {
	ProviderID  string   `json:"provider_id"`
	Permissions []string `json:"permissions"`
}

type Config struct {
	ListenAddr                        string
	MCPProxyBaseURL                   string
	MCPProxyAdminKeyFile              string
	MCPProxyConfigFile                string
	MCPProxyAdminKey                  string
	PublicBaseURL                     string
	MountPath                         string
	DBPath                            string
	AllowedHosts                      []string
	SessionLifetime                   time.Duration
	OAuthCDPURL                       string
	OAuthBrowserURL                   string
	BrowserProvider                   string
	BrowserInstance                   string
	BrowserInstances                  []BrowserInstance
	BitwardenMode                     string
	BitwardenBaseURL                  string
	PostizBaseURL                     string
	PaperlessEndpoint                 string
	ImmichKeyDir                      string
	OmniRouteDB                       string
	OpenAlexKeyFile                   string
	YouTubeOAuthControlURL            string
	CredentialPendingHostDir          string
	CredentialSSHTransferRoot         string
	CredentialSourceSSHProfile        string
	CryptoComAppRemoteEnvPath         string
	CryptoComAppRemoteComposeDir      string
	CryptoComAppRemoteService         string
	CryptoComExchangeRemoteEnvPath    string
	CryptoComExchangeRemoteComposeDir string
	CryptoComExchangeRemoteService    string
	CryptoComRemoteUser               string
	CryptoComRemoteSSHProfile         string
	DifyRemoteSSHProfile              string
	DifyApplyHelperPath               string
	DifyBindings                      map[string]DifyBinding
	DemoMode                          bool
}

func Load() (Config, error) {
	lifetime, err := time.ParseDuration(envOr("SIDEKICK_SESSION_LIFETIME", "720h"))
	if err != nil {
		return Config{}, errors.New("invalid SIDEKICK_SESSION_LIFETIME")
	}
	difyBindings, err := loadDifyBindings(os.Getenv("SIDEKICK_DIFY_BINDINGS_JSON"))
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		ListenAddr:                        envOr("SIDEKICK_LISTEN_ADDR", ":8081"),
		MCPProxyBaseURL:                   strings.TrimRight(strings.TrimSpace(os.Getenv("SIDEKICK_MCPPROXY_URL")), "/"),
		MCPProxyAdminKeyFile:              strings.TrimSpace(os.Getenv("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE")),
		MCPProxyConfigFile:                strings.TrimSpace(os.Getenv("SIDEKICK_MCPPROXY_CONFIG_FILE")),
		PublicBaseURL:                     strings.TrimRight(strings.TrimSpace(os.Getenv("SIDEKICK_PUBLIC_BASE_URL")), "/"),
		DBPath:                            envOr("SIDEKICK_DB_PATH", "/data/sidekick.db"),
		AllowedHosts:                      splitCSV(os.Getenv("SIDEKICK_ALLOWED_HOSTS")),
		SessionLifetime:                   lifetime,
		OAuthCDPURL:                       envOr("SIDEKICK_OAUTH_CDP_URL", "http://127.0.0.1:9222"),
		OAuthBrowserURL:                   strings.TrimSpace(os.Getenv("SIDEKICK_OAUTH_BROWSER_URL")),
		BrowserProvider:                   strings.ToLower(envOr("SIDEKICK_BROWSER_PROVIDER", "chromium")),
		BitwardenMode:                     strings.ToLower(envOr("SIDEKICK_BITWARDEN_MODE", "off")),
		BitwardenBaseURL:                  strings.TrimRight(strings.TrimSpace(os.Getenv("SIDEKICK_BITWARDEN_BASE_URL")), "/"),
		PostizBaseURL:                     strings.TrimSpace(os.Getenv("SIDEKICK_POSTIZ_BASE_URL")),
		PaperlessEndpoint:                 strings.TrimSpace(os.Getenv("SIDEKICK_PAPERLESS_ENDPOINT")),
		ImmichKeyDir:                      strings.TrimSpace(os.Getenv("SIDEKICK_IMMICH_KEY_DIR")),
		OmniRouteDB:                       strings.TrimSpace(os.Getenv("SIDEKICK_OMNIROUTE_DB")),
		OpenAlexKeyFile:                   strings.TrimSpace(os.Getenv("SIDEKICK_OPENALEX_KEY_FILE")),
		YouTubeOAuthControlURL:            strings.TrimRight(strings.TrimSpace(os.Getenv("SIDEKICK_YOUTUBE_OAUTH_CONTROL_URL")), "/"),
		CredentialPendingHostDir:          strings.TrimSpace(os.Getenv("SIDEKICK_CREDENTIAL_PENDING_HOST_DIR")),
		CredentialSSHTransferRoot:         strings.TrimSpace(os.Getenv("SIDEKICK_CREDENTIAL_SSH_TRANSFER_ROOT")),
		CredentialSourceSSHProfile:        strings.TrimSpace(os.Getenv("SIDEKICK_CREDENTIAL_SOURCE_SSH_PROFILE")),
		CryptoComAppRemoteEnvPath:         strings.TrimSpace(envOr("SIDEKICK_CRYPTOCOM_APP_REMOTE_ENV_PATH", os.Getenv("SIDEKICK_CRYPTOCOM_REMOTE_ENV_PATH"))),
		CryptoComAppRemoteComposeDir:      strings.TrimSpace(envOr("SIDEKICK_CRYPTOCOM_APP_REMOTE_COMPOSE_DIR", os.Getenv("SIDEKICK_CRYPTOCOM_REMOTE_COMPOSE_DIR"))),
		CryptoComAppRemoteService:         envOr("SIDEKICK_CRYPTOCOM_APP_REMOTE_SERVICE", envOr("SIDEKICK_CRYPTOCOM_REMOTE_SERVICE", "cryptocom-app")),
		CryptoComExchangeRemoteEnvPath:    strings.TrimSpace(os.Getenv("SIDEKICK_CRYPTOCOM_EXCHANGE_REMOTE_ENV_PATH")),
		CryptoComExchangeRemoteComposeDir: strings.TrimSpace(os.Getenv("SIDEKICK_CRYPTOCOM_EXCHANGE_REMOTE_COMPOSE_DIR")),
		CryptoComExchangeRemoteService:    envOr("SIDEKICK_CRYPTOCOM_EXCHANGE_REMOTE_SERVICE", "cryptocom-exchange"),
		CryptoComRemoteUser:               strings.TrimSpace(os.Getenv("SIDEKICK_CRYPTOCOM_REMOTE_USER")),
		CryptoComRemoteSSHProfile:         strings.TrimSpace(os.Getenv("SIDEKICK_CRYPTOCOM_REMOTE_SSH_PROFILE")),
		DifyRemoteSSHProfile:              strings.TrimSpace(os.Getenv("SIDEKICK_DIFY_REMOTE_SSH_PROFILE")),
		DifyApplyHelperPath:               strings.TrimSpace(os.Getenv("SIDEKICK_DIFY_APPLY_HELPER_PATH")),
		DifyBindings:                      difyBindings,
		DemoMode:                          parseBool(os.Getenv("SIDEKICK_DEMO_MODE")),
	}
	if _, err := browser.Resolve(cfg.BrowserProvider); err != nil {
		return Config{}, errors.New("invalid SIDEKICK_BROWSER_PROVIDER")
	}
	switch cfg.BitwardenMode {
	case "off", "assist", "managed-extension":
	default:
		return Config{}, errors.New("invalid SIDEKICK_BITWARDEN_MODE")
	}
	mountPath := strings.TrimSpace(os.Getenv("SIDEKICK_MOUNT_PATH"))
	if mountPath == "" && cfg.PublicBaseURL != "" {
		if parsed, parseErr := url.Parse(cfg.PublicBaseURL); parseErr == nil {
			mountPath = parsed.Path
		}
	}
	cfg.MountPath = normalizeMountPath(mountPath)
	if cfg.OAuthBrowserURL == "" {
		cfg.OAuthBrowserURL = "/oauth-browser/"
		if cfg.MountPath != "" {
			cfg.OAuthBrowserURL = cfg.MountPath + "/oauth-browser/"
		}
	}
	instances, selected, err := loadBrowserInstances(cfg)
	if err != nil {
		return Config{}, err
	}
	cfg.BrowserInstances = instances
	cfg.BrowserInstance = selected
	if instance, ok := cfg.BrowserInstanceByID(selected); ok {
		cfg.OAuthCDPURL = instance.CDPURL
		cfg.OAuthBrowserURL = instance.BrowserURL
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
	if cfg.MCPProxyAdminKeyFile == "" && cfg.MCPProxyConfigFile != "" {
		raw, err := os.ReadFile(cfg.MCPProxyConfigFile)
		if err != nil {
			return Config{}, errors.New("cannot read SIDEKICK_MCPPROXY_CONFIG_FILE")
		}
		var parsed struct {
			APIKey string `json:"api_key"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return Config{}, errors.New("invalid MCPProxy config JSON")
		}
		cfg.MCPProxyAdminKey = strings.TrimSpace(parsed.APIKey)
	}
	if cfg.MCPProxyAdminKeyFile == "" && cfg.MCPProxyAdminKey == "" {
		return Config{}, errors.New("SIDEKICK_MCPPROXY_ADMIN_KEY_FILE or SIDEKICK_MCPPROXY_CONFIG_FILE is required")
	}
	return cfg, nil
}

func loadDifyBindings(raw string) (map[string]DifyBinding, error) {
	bindings := map[string]DifyBinding{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return bindings, nil
	}
	var parsed map[string]DifyBinding
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, errors.New("invalid SIDEKICK_DIFY_BINDINGS_JSON")
	}
	validPermission := map[string]bool{"read": true, "write": true, "destructive": true}
	for rawProfile, binding := range parsed {
		profile := strings.TrimSpace(rawProfile)
		binding.ProviderID = strings.TrimSpace(binding.ProviderID)
		if profile == "" || binding.ProviderID == "" || len(binding.Permissions) == 0 {
			return nil, errors.New("SIDEKICK_DIFY_BINDINGS_JSON entries require profile, provider_id and permissions")
		}
		if _, exists := bindings[profile]; exists {
			return nil, errors.New("SIDEKICK_DIFY_BINDINGS_JSON contains duplicate profile names after normalization")
		}
		seen := map[string]bool{}
		for i, permission := range binding.Permissions {
			permission = strings.ToLower(strings.TrimSpace(permission))
			if !validPermission[permission] || seen[permission] {
				return nil, errors.New("SIDEKICK_DIFY_BINDINGS_JSON contains invalid or duplicate permissions")
			}
			seen[permission] = true
			binding.Permissions[i] = permission
		}
		bindings[profile] = binding
	}
	return bindings, nil
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

func normalizeMountPath(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "/" {
		return ""
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	return strings.TrimRight(v, "/")
}

func loadBrowserInstances(cfg Config) ([]BrowserInstance, string, error) {
	selected := strings.TrimSpace(os.Getenv("SIDEKICK_BROWSER_INSTANCE"))
	raw := strings.TrimSpace(os.Getenv("SIDEKICK_BROWSER_INSTANCES_JSON"))
	instances := []BrowserInstance{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &instances); err != nil {
			return nil, "", errors.New("invalid SIDEKICK_BROWSER_INSTANCES_JSON")
		}
	} else {
		id := selected
		if id == "" {
			id = "default"
		}
		label := strings.TrimSpace(os.Getenv("SIDEKICK_BROWSER_INSTANCE_LABEL"))
		if label == "" {
			label = id
		}
		instances = []BrowserInstance{{
			ID: id, Label: label, CDPURL: cfg.OAuthCDPURL, BrowserURL: cfg.OAuthBrowserURL,
		}}
	}
	if len(instances) == 0 {
		return nil, "", errors.New("at least one browser instance is required")
	}
	seen := map[string]bool{}
	for i := range instances {
		instances[i].ID = strings.TrimSpace(instances[i].ID)
		instances[i].Label = strings.TrimSpace(instances[i].Label)
		instances[i].CDPURL = strings.TrimRight(strings.TrimSpace(instances[i].CDPURL), "/")
		instances[i].BrowserURL = strings.TrimSpace(instances[i].BrowserURL)
		instances[i].LaunchURL = strings.TrimSpace(instances[i].LaunchURL)
		if instances[i].ID == "" || instances[i].CDPURL == "" || instances[i].BrowserURL == "" {
			return nil, "", errors.New("browser instance requires id, cdp_url and browser_url")
		}
		if instances[i].LaunchURL != "" {
			launch, err := url.Parse(instances[i].LaunchURL)
			if err != nil || launch.Host == "" || (launch.Scheme != "http" && launch.Scheme != "https") {
				return nil, "", errors.New("browser instance launch_url must be an absolute http(s) URL")
			}
		}
		if seen[instances[i].ID] {
			return nil, "", errors.New("duplicate browser instance id")
		}
		seen[instances[i].ID] = true
		if instances[i].Label == "" {
			instances[i].Label = instances[i].ID
		}
	}
	if selected == "" {
		selected = instances[0].ID
	}
	if !seen[selected] {
		return nil, "", errors.New("SIDEKICK_BROWSER_INSTANCE does not match a configured browser instance")
	}
	return instances, selected, nil
}

func (c Config) BrowserInstanceByID(id string) (BrowserInstance, bool) {
	id = strings.TrimSpace(id)
	for _, instance := range c.BrowserInstances {
		if instance.ID == id {
			return instance, true
		}
	}
	return BrowserInstance{}, false
}
