package mcpproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, adminKeyFile string) (*Client, error) {
	raw, err := os.ReadFile(adminKeyFile)
	if err != nil {
		return nil, err
	}
	return NewClientWithKey(baseURL, string(raw))
}

func NewClientWithKey(baseURL, rawKey string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("empty MCPProxy base URL")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid MCPProxy base URL: %w", err)
	}
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return nil, errors.New("empty MCPProxy admin key")
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  key,
		http:    &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (c *Client) ListServers(ctx context.Context) ([]Server, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/servers", nil, &raw); err != nil {
		return nil, err
	}
	return decodeServers(raw)
}

func (c *Client) PatchServer(ctx context.Context, name string, patch ServerPatch) error {
	return c.doJSON(ctx, http.MethodPatch, "/api/v1/servers/"+url.PathEscape(name), patch, nil)
}

func (c *Client) EnableServer(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/v1/servers/"+url.PathEscape(name)+"/enable", map[string]any{}, nil)
}

func (c *Client) DisableServer(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/v1/servers/"+url.PathEscape(name)+"/disable", map[string]any{}, nil)
}

func (c *Client) LogoutOAuth(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/v1/servers/"+url.PathEscape(name)+"/logout", map[string]any{}, nil)
}

func (c *Client) StartOAuth(ctx context.Context, name string) (OAuthStart, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/servers/"+url.PathEscape(name)+"/login", map[string]any{}, &raw); err != nil {
		return OAuthStart{}, err
	}
	start := findOAuthStart(raw)
	if start.AuthURL == "" {
		return OAuthStart{}, errors.New("MCPProxy login response did not include auth_url")
	}
	return start, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mcpproxy %s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func decodeServers(raw json.RawMessage) ([]Server, error) {
	var direct []Server
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decode server inventory: %w", err)
	}
	for _, key := range []string{"servers", "data"} {
		if child, ok := obj[key]; ok {
			if servers, err := decodeServers(child); err == nil {
				return servers, nil
			}
			var nested map[string]json.RawMessage
			if json.Unmarshal(child, &nested) == nil {
				if grandchild, ok := nested["servers"]; ok {
					return decodeServers(grandchild)
				}
			}
		}
	}
	return nil, errors.New("server inventory did not contain a server array")
}

func findOAuthStart(raw json.RawMessage) OAuthStart {
	var direct OAuthStart
	_ = json.Unmarshal(raw, &direct)
	if direct.AuthURL != "" {
		return direct
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return OAuthStart{}
	}
	for _, child := range obj {
		if start := findOAuthStart(child); start.AuthURL != "" {
			return start
		}
	}
	return OAuthStart{}
}
