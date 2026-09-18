package mcpproxy

type Health struct {
	Summary string `json:"summary"`
}

type Server struct {
	Name          string            `json:"name"`
	Enabled       bool              `json:"enabled"`
	Status        string            `json:"status"`
	Protocol      string            `json:"protocol,omitempty"`
	URL           string            `json:"url,omitempty"`
	ToolCount     int               `json:"tool_count,omitempty"`
	Authenticated bool              `json:"authenticated,omitempty"`
	Quarantined   bool              `json:"quarantined,omitempty"`
	OAuth         map[string]any    `json:"oauth,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Health        Health            `json:"health,omitempty"`
	LastError     string            `json:"last_error,omitempty"`
}

type ServerPatch struct {
	URL      string            `json:"url,omitempty"`
	Protocol string            `json:"protocol,omitempty"`
	Enabled  *bool             `json:"enabled,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	OAuth    map[string]any    `json:"oauth,omitempty"`
}

type OAuthStart struct {
	AuthURL       string `json:"auth_url"`
	CorrelationID string `json:"correlation_id,omitempty"`
	BrowserOpened bool   `json:"browser_opened,omitempty"`
}
