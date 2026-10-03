package mcpproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) GetServer(ctx context.Context, name string) (Server, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Server{}, fmt.Errorf("server name is required")
	}
	servers, err := c.ListServers(ctx)
	if err != nil {
		return Server{}, err
	}
	for _, server := range servers {
		if server.Name == name {
			return server, nil
		}
	}
	return Server{}, fmt.Errorf("server %q does not exist", name)
}

func (c *Client) RestartServer(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("server name is required")
	}
	return c.doJSON(ctx, http.MethodPost, "/api/v1/servers/"+url.PathEscape(name)+"/restart", map[string]any{}, nil)
}
