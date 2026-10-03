package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Info struct {
	Version string `json:"version"`
}

func (c *Client) Info(ctx context.Context) (Info, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/info", nil, &raw); err != nil {
		return Info{}, err
	}
	info := findInfo(raw)
	if strings.TrimSpace(info.Version) == "" {
		return Info{}, errors.New("MCPProxy info response did not include version")
	}
	return info, nil
}

func (c *Client) ProbeEndpoint(ctx context.Context, path string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return false, fmt.Errorf("mcpproxy GET %s: HTTP %d", path, resp.StatusCode)
	case resp.StatusCode >= 500:
		return false, fmt.Errorf("mcpproxy GET %s: HTTP %d", path, resp.StatusCode)
	default:
		return true, nil
	}
}

func findInfo(raw json.RawMessage) Info {
	var direct Info
	_ = json.Unmarshal(raw, &direct)
	if strings.TrimSpace(direct.Version) != "" {
		return direct
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return Info{}
	}
	for _, child := range obj {
		if info := findInfo(child); strings.TrimSpace(info.Version) != "" {
			return info
		}
	}
	return Info{}
}
