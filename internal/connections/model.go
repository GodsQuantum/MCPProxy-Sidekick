package connections

import (
	"net/url"
	"sort"
	"strings"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/storage"
)

type Detail struct {
	Name                 string   `json:"name"`
	Enabled              bool     `json:"enabled"`
	Status               string   `json:"status"`
	Protocol             string   `json:"protocol,omitempty"`
	URL                  string   `json:"url,omitempty"`
	ToolCount            int      `json:"tool_count"`
	Ready                bool     `json:"ready"`
	Authenticated        bool     `json:"authenticated"`
	Quarantined          bool     `json:"quarantined"`
	OAuth                bool     `json:"oauth"`
	OAuthScopes          []string `json:"oauth_scopes,omitempty"`
	CredentialConfigured bool     `json:"credential_configured"`
	CredentialPreview    string   `json:"credential_preview,omitempty"`
	Profiles             []string `json:"profiles,omitempty"`
	Actions              []string `json:"actions"`
}

func Build(server mcpproxy.Server, profiles []mcpproxy.Profile, meta storage.CredentialMeta, hasMeta bool) Detail {
	status := strings.TrimSpace(server.Status)
	if status == "" {
		status = strings.TrimSpace(server.Health.Summary)
	}
	lowerStatus := strings.ToLower(status)
	ready := strings.Contains(lowerStatus, "ready") || strings.Contains(lowerStatus, "connected")

	detail := Detail{
		Name:          server.Name,
		Enabled:       server.Enabled,
		Status:        status,
		Protocol:      server.Protocol,
		URL:           safeEndpoint(server.URL),
		ToolCount:     server.ToolCount,
		Ready:         ready,
		Authenticated: server.Authenticated,
		Quarantined:   server.Quarantined,
		OAuth:         len(server.OAuth) > 0,
	}

	detail.Profiles = profileNames(server.Name, profiles)
	detail.OAuthScopes = oauthScopes(server.OAuth)
	detail.CredentialConfigured, detail.CredentialPreview = credentialState(server, meta, hasMeta)
	detail.Actions = actions(detail)
	return detail
}

func safeEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	origin := scheme + "://" + u.Host
	if u.Path != "" && u.Path != "/" {
		return origin + "/…"
	}
	if u.Path == "/" {
		return origin + "/"
	}
	return origin
}

func profileNames(server string, profiles []mcpproxy.Profile) []string {
	var names []string
	for _, profile := range profiles {
		for _, member := range profile.Servers {
			if member == server {
				names = append(names, profile.Name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

func oauthScopes(raw map[string]any) []string {
	v, ok := raw["scopes"]
	if !ok {
		return nil
	}
	var scopes []string
	switch got := v.(type) {
	case []string:
		scopes = append(scopes, got...)
	case []any:
		for _, item := range got {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				scopes = append(scopes, strings.TrimSpace(s))
			}
		}
	case string:
		scopes = strings.Fields(got)
	}
	sort.Strings(scopes)
	return scopes
}

func credentialState(server mcpproxy.Server, meta storage.CredentialMeta, hasMeta bool) (bool, string) {
	if server.Name == "cryptocom-live" && hasMeta && !strings.Contains(meta.MaskedPreview, " + secret set · valid until ") && !strings.HasPrefix(meta.MaskedPreview, "Crypto.com from Vaultwarden ") {
		return false, "Crypto.com credentials required"
	}
	if hasMeta {
		return true, meta.MaskedPreview
	}
	if server.Authenticated && len(server.OAuth) > 0 {
		return true, "OAuth connected"
	}
	for _, value := range server.Headers {
		value = strings.TrimSpace(value)
		bare := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, "Bearer "), "bearer "))
		if strings.HasPrefix(bare, "${env:") || strings.HasPrefix(bare, "${keyring:") {
			return true, "Managed reference"
		}
		if bare != "" {
			return true, credentials.Mask(bare)
		}
	}
	return false, ""
}

func actions(d Detail) []string {
	var out []string
	if d.Enabled {
		out = append(out, "disable")
	} else {
		out = append(out, "enable")
	}
	out = append(out, "restart")
	if d.OAuth {
		out = append(out, "reconnect")
	}
	return out
}
