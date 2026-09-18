package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

func (c *Client) ListAgentTokens(ctx context.Context) ([]AgentToken, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/tokens", nil, &raw); err != nil {
		return nil, err
	}
	var direct []AgentToken
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}
	var wrapped struct {
		Tokens []AgentToken `json:"tokens"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Tokens != nil {
		return wrapped.Tokens, nil
	}
	return nil, errors.New("agent token response did not contain a token array")
}

func (c *Client) CreateAgentToken(ctx context.Context, req CreateAgentTokenRequest) (CreatedAgentToken, error) {
	var out CreatedAgentToken
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/tokens", req, &out)
	return out, err
}

func (c *Client) RegenerateAgentToken(ctx context.Context, name string) (RegeneratedAgentToken, error) {
	var out RegeneratedAgentToken
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/tokens/"+url.PathEscape(name)+"/regenerate", map[string]any{}, &out)
	return out, err
}

func (c *Client) RevokeAgentToken(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/tokens/"+url.PathEscape(name), nil, nil)
}

func (c *Client) DeleteAgentToken(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/tokens/"+url.PathEscape(name)+"/permanent", nil, nil)
}
