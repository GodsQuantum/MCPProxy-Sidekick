package mcpproxy

import "time"

type CreateAgentTokenRequest struct {
	Name           string   `json:"name"`
	AllowedServers []string `json:"allowed_servers"`
	Permissions    []string `json:"permissions"`
	ExpiresIn      string   `json:"expires_in,omitempty"`
	ProfilePin     string   `json:"profile_pin,omitempty"`
}

type CreatedAgentToken struct {
	Name           string    `json:"name"`
	Token          string    `json:"token"`
	AllowedServers []string  `json:"allowed_servers"`
	Permissions    []string  `json:"permissions"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	ProfilePin     string    `json:"profile_pin,omitempty"`
}

type AgentToken struct {
	Name           string     `json:"name"`
	TokenPrefix    string     `json:"token_prefix"`
	AllowedServers []string   `json:"allowed_servers"`
	Permissions    []string   `json:"permissions"`
	ExpiresAt      time.Time  `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	Revoked        bool       `json:"revoked"`
	ProfilePin     string     `json:"profile_pin,omitempty"`
}

type RegeneratedAgentToken struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}
