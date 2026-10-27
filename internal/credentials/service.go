package credentials

import (
	"context"
	"errors"
	"strings"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type ServerEditor interface {
	PatchServer(context.Context, string, mcpproxy.ServerPatch) error
	EnableServer(context.Context, string) error
}

type Service struct {
	Editor ServerEditor
}

func (s Service) SetGeneric(ctx context.Context, server, mode, headerName, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("empty credential")
	}
	var headers map[string]string
	switch mode {
	case "bearer":
		headers = map[string]string{"Authorization": "Bearer " + value}
	case "x-api-key":
		headers = map[string]string{"X-API-Key": value}
	case "custom-header":
		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			return errors.New("custom header name is required")
		}
		headers = map[string]string{headerName: value}
	default:
		return errors.New("unsupported credential mode")
	}
	if err := s.Editor.PatchServer(ctx, server, mcpproxy.ServerPatch{Headers: headers}); err != nil {
		return err
	}
	return s.Editor.EnableServer(ctx, server)
}

// SetEnvironment patches only requested variables using MCPProxy native merge semantics.
func (s Service) SetEnvironment(ctx context.Context, server string, values map[string]string) error {
	if len(values) == 0 {
		return errors.New("no environment credentials")
	}
	for key, value := range values {
		if !isSecretEnv(key) || strings.TrimSpace(value) == "" {
			return errors.New("invalid or empty credential field")
		}
	}
	if err := s.Editor.PatchServer(ctx, server, mcpproxy.ServerPatch{Env: values}); err != nil {
		return err
	}
	return s.Editor.EnableServer(ctx, server)
}
