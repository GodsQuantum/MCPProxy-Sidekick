package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func (c *Client) GetConfig(ctx context.Context) (map[string]any, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/config", nil, &raw); err != nil {
		return nil, err
	}
	return decodeConfigDocument(raw)
}

func (c *Client) ValidateConfig(ctx context.Context, doc map[string]any) error {
	var response struct {
		Success bool     `json:"success"`
		Valid   bool     `json:"valid"`
		Errors  []string `json:"errors"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/config/validate", doc, &response); err != nil {
		return err
	}
	if !response.Valid {
		if len(response.Errors) == 0 {
			return errors.New("MCPProxy rejected configuration")
		}
		return fmt.Errorf("MCPProxy rejected configuration: %v", response.Errors)
	}
	return nil
}

func (c *Client) ApplyConfig(ctx context.Context, doc map[string]any) error {
	var response struct {
		Success          bool     `json:"success"`
		Errors           []string `json:"errors"`
		ValidationErrors []any    `json:"validation_errors"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/config/apply", doc, &response); err != nil {
		return err
	}
	if !response.Success && (len(response.Errors) > 0 || len(response.ValidationErrors) > 0) {
		return fmt.Errorf("MCPProxy failed to apply configuration")
	}
	return nil
}

func decodeConfigDocument(raw json.RawMessage) (map[string]any, error) {
	var direct map[string]any
	if err := json.Unmarshal(raw, &direct); err != nil {
		return nil, fmt.Errorf("decode MCPProxy config: %w", err)
	}
	for _, key := range []string{"config", "data"} {
		if child, ok := direct[key]; ok {
			if doc, ok := child.(map[string]any); ok {
				return doc, nil
			}
		}
	}
	if _, ok := direct["mcpServers"]; ok {
		return direct, nil
	}
	return nil, errors.New("MCPProxy config response did not contain config document")
}
