package paperless

import (
	"context"
	"errors"
	"strings"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type Adapter struct {
	Editor   credentials.ServerEditor
	Endpoint string
}

func (a Adapter) Apply(ctx context.Context, alias, token string) error {
	alias = strings.TrimSpace(alias)
	token = strings.TrimSpace(token)
	endpoint := strings.TrimSpace(a.Endpoint)
	if alias == "" {
		return errors.New("empty Paperless alias")
	}
	if token == "" {
		return errors.New("empty Paperless token")
	}
	if endpoint == "" {
		return errors.New("empty Paperless MCP endpoint")
	}
	patch := mcpproxy.ServerPatch{
		URL:      endpoint,
		Protocol: "streamable-http",
		Headers:  map[string]string{"Authorization": "Bearer " + token},
	}
	if err := a.Editor.PatchServer(ctx, alias, patch); err != nil {
		return err
	}
	return a.Editor.EnableServer(ctx, alias)
}
