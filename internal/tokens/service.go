package tokens

import (
	"context"
	"errors"
	"strings"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type Backend interface {
	CreateAgentToken(context.Context, mcpproxy.CreateAgentTokenRequest) (mcpproxy.CreatedAgentToken, error)
	ListAgentTokens(context.Context) ([]mcpproxy.AgentToken, error)
	RegenerateAgentToken(context.Context, string) (mcpproxy.RegeneratedAgentToken, error)
	RevokeAgentToken(context.Context, string) error
	DeleteAgentToken(context.Context, string) error
}

type Service struct{ Backend Backend }

type CreateRequest struct {
	Name               string
	AllowedServers     []string
	Permissions        []string
	ExpiresIn          string
	ProfilePin         string
	ConfirmDestructive bool
}

func (s Service) Create(ctx context.Context, req CreateRequest) (mcpproxy.CreatedAgentToken, error) {
	if s.Backend == nil {
		return mcpproxy.CreatedAgentToken{}, errors.New("missing token backend")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return mcpproxy.CreatedAgentToken{}, errors.New("token name is required")
	}
	if len(req.AllowedServers) == 0 {
		return mcpproxy.CreatedAgentToken{}, errors.New("at least one allowed server is required")
	}
	hasRead, hasDestructive := false, false
	for _, p := range req.Permissions {
		switch p {
		case "read":
			hasRead = true
		case "write":
		case "destructive":
			hasDestructive = true
		default:
			return mcpproxy.CreatedAgentToken{}, errors.New("unsupported permission: " + p)
		}
	}
	if !hasRead {
		return mcpproxy.CreatedAgentToken{}, errors.New("read permission is required")
	}
	if hasDestructive && !req.ConfirmDestructive {
		return mcpproxy.CreatedAgentToken{}, errors.New("destructive permission requires explicit confirmation")
	}
	expires := strings.TrimSpace(req.ExpiresIn)
	if expires == "" {
		expires = "30d"
	}
	return s.Backend.CreateAgentToken(ctx, mcpproxy.CreateAgentTokenRequest{
		Name: name, AllowedServers: req.AllowedServers, Permissions: req.Permissions, ExpiresIn: expires, ProfilePin: strings.TrimSpace(req.ProfilePin),
	})
}

func (s Service) List(ctx context.Context) ([]mcpproxy.AgentToken, error) {
	return s.Backend.ListAgentTokens(ctx)
}
func (s Service) Regenerate(ctx context.Context, name string) (mcpproxy.RegeneratedAgentToken, error) {
	return s.Backend.RegenerateAgentToken(ctx, name)
}
func (s Service) Revoke(ctx context.Context, name string) error {
	return s.Backend.RevokeAgentToken(ctx, name)
}
func (s Service) Delete(ctx context.Context, name string) error {
	return s.Backend.DeleteAgentToken(ctx, name)
}
