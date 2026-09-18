package postiz

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type Adapter struct {
	Editor     credentials.ServerEditor
	BaseURL    string
	ServerName string
}

func (a Adapter) Apply(ctx context.Context, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("empty Postiz API key")
	}
	base := strings.TrimRight(strings.TrimSpace(a.BaseURL), "/")
	if base == "" {
		return errors.New("empty Postiz MCP base URL")
	}
	escaped := url.PathEscape(value)
	patch := mcpproxy.ServerPatch{
		URL:     base + "/" + escaped,
		Headers: map[string]string{},
	}
	if err := a.Editor.PatchServer(ctx, a.ServerName, patch); err != nil {
		return err
	}
	return a.Editor.EnableServer(ctx, a.ServerName)
}
