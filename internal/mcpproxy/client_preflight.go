package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Preflight checks local MCPProxy readiness without upstream calls.
// MCPProxy persists one local audit entry per check.
type PreflightTool struct {
	ID      string `json:"id"`
	PinHash string `json:"pin_hash,omitempty"`
}
type PreflightRequest struct {
	Tools   []PreflightTool  `json:"tools"`
	Profile string           `json:"profile,omitempty"`
	Policy  *PreflightPolicy `json:"policy,omitempty"`
}

type PreflightPolicy struct {
	ReadOnlyOnly bool `json:"read_only_only"`
}
type PreflightToolResult struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Hash        string `json:"hash,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Retryable   bool   `json:"retryable,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}
type PreflightResult struct {
	CheckedAt string                `json:"checked_at,omitempty"`
	Verdict   string                `json:"verdict"`
	Tools     []PreflightToolResult `json:"tools"`
}

var ErrPreflightUnavailable = errors.New("MCPProxy required-tools preflight is unavailable")

func (c *Client) Preflight(ctx context.Context, req PreflightRequest) (PreflightResult, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, "POST", "/api/v1/preflight", req, &raw); err != nil {
		if strings.Contains(err.Error(), "HTTP 404:") || strings.Contains(err.Error(), "HTTP 405:") {
			return PreflightResult{}, ErrPreflightUnavailable
		}
		return PreflightResult{}, fmt.Errorf("MCPProxy preflight request: %w", err)
	}
	var envelope struct {
		Success *bool           `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return PreflightResult{}, errors.New("invalid MCPProxy preflight response")
	}
	if envelope.Success != nil && !*envelope.Success {
		return PreflightResult{}, errors.New("MCPProxy preflight did not succeed")
	}
	if len(envelope.Data) > 0 {
		raw = envelope.Data
	}
	var result PreflightResult
	if err := json.Unmarshal(raw, &result); err != nil || strings.TrimSpace(result.Verdict) == "" || len(result.Tools) == 0 {
		return PreflightResult{}, errors.New("invalid MCPProxy preflight result")
	}
	return result, nil
}
