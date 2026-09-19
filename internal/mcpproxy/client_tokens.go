package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) ListAgentTokens(ctx context.Context) ([]AgentToken, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/tokens", nil, &raw); err != nil {
		return nil, err
	}
	return decodeAgentTokens(raw)
}

func decodeAgentTokens(raw json.RawMessage) ([]AgentToken, error) {
	var direct []AgentToken
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decode agent token inventory: %w", err)
	}
	for _, key := range []string{"tokens", "data"} {
		if child, ok := obj[key]; ok {
			if tokens, err := decodeAgentTokens(child); err == nil {
				return tokens, nil
			}
		}
	}
	return nil, errors.New("agent token response did not contain a token array")
}

func (c *Client) CreateAgentToken(ctx context.Context, req CreateAgentTokenRequest) (CreatedAgentToken, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/tokens", req, &raw); err != nil {
		return CreatedAgentToken{}, err
	}
	return decodeCreatedAgentToken(raw)
}

func (c *Client) RegenerateAgentToken(ctx context.Context, name string) (RegeneratedAgentToken, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/tokens/"+url.PathEscape(name)+"/regenerate", map[string]any{}, &raw); err != nil {
		return RegeneratedAgentToken{}, err
	}
	return decodeRegeneratedAgentToken(raw)
}

func (c *Client) RevokeAgentToken(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/tokens/"+url.PathEscape(name), nil, nil)
}

func (c *Client) DeleteAgentToken(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/tokens/"+url.PathEscape(name)+"/permanent", nil, nil)
}

func decodeCreatedAgentToken(raw json.RawMessage) (CreatedAgentToken, error) {
	var direct CreatedAgentToken
	if err := json.Unmarshal(raw, &direct); err == nil && direct.Token != "" {
		return direct, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return CreatedAgentToken{}, fmt.Errorf("decode created Agent Token: %w", err)
	}
	if child, ok := obj["data"]; ok {
		return decodeCreatedAgentToken(child)
	}
	return CreatedAgentToken{}, errors.New("created Agent Token response did not contain a one-time token")
}

func decodeRegeneratedAgentToken(raw json.RawMessage) (RegeneratedAgentToken, error) {
	var direct RegeneratedAgentToken
	if err := json.Unmarshal(raw, &direct); err == nil && direct.Token != "" {
		return direct, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return RegeneratedAgentToken{}, fmt.Errorf("decode regenerated Agent Token: %w", err)
	}
	if child, ok := obj["data"]; ok {
		return decodeRegeneratedAgentToken(child)
	}
	return RegeneratedAgentToken{}, errors.New("regenerated Agent Token response did not contain a one-time token")
}
