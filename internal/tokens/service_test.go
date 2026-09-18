package tokens

import (
	"context"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type fakeBackend struct {
	created mcpproxy.CreateAgentTokenRequest
}

func (f *fakeBackend) CreateAgentToken(_ context.Context, req mcpproxy.CreateAgentTokenRequest) (mcpproxy.CreatedAgentToken, error) {
	f.created = req
	return mcpproxy.CreatedAgentToken{Name: req.Name, Token: "mcp_agt_once"}, nil
}
func (f *fakeBackend) ListAgentTokens(context.Context) ([]mcpproxy.AgentToken, error) {
	return nil, nil
}
func (f *fakeBackend) RegenerateAgentToken(context.Context, string) (mcpproxy.RegeneratedAgentToken, error) {
	return mcpproxy.RegeneratedAgentToken{}, nil
}
func (f *fakeBackend) RevokeAgentToken(context.Context, string) error { return nil }
func (f *fakeBackend) DeleteAgentToken(context.Context, string) error { return nil }

func TestCreateRequiresReadPermission(t *testing.T) {
	s := Service{Backend: &fakeBackend{}}
	_, err := s.Create(context.Background(), CreateRequest{
		Name:           "family",
		AllowedServers: []string{"paperless-profile-b"},
		Permissions:    []string{"write"},
		ExpiresIn:      "30d",
	})
	if err == nil {
		t.Fatal("expected read permission requirement")
	}
}

func TestCreateRequiresExplicitDestructiveConfirmation(t *testing.T) {
	s := Service{Backend: &fakeBackend{}}
	_, err := s.Create(context.Background(), CreateRequest{
		Name:               "family",
		AllowedServers:     []string{"paperless-profile-b"},
		Permissions:        []string{"read", "destructive"},
		ExpiresIn:          "30d",
		ConfirmDestructive: false,
	})
	if err == nil {
		t.Fatal("expected destructive confirmation requirement")
	}
}

func TestCreatePassesProfilePin(t *testing.T) {
	f := &fakeBackend{}
	s := Service{Backend: f}
	got, err := s.Create(context.Background(), CreateRequest{
		Name:           "family",
		AllowedServers: []string{"paperless-profile-b", "immich-profile-b"},
		Permissions:    []string{"read", "write"},
		ExpiresIn:      "30d",
		ProfilePin:     "family-member",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "mcp_agt_once" {
		t.Fatalf("token = %q", got.Token)
	}
	if f.created.ProfilePin != "family-member" {
		t.Fatalf("profile pin = %q", f.created.ProfilePin)
	}
}
