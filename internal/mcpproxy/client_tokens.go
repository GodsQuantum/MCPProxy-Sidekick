package mcpproxy

import (
	"context"
	"net/http"
	"net/url"
)

func (c *Client) ListAgentTokens(ctx context.Context) ([]AgentToken, error) {
	var out []AgentToken
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/tokens", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
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
