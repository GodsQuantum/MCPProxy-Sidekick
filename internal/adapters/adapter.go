package adapters

import (
	"context"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type View struct {
	Kind       string `json:"kind"`
	Configured bool   `json:"configured"`
	Preview    string `json:"preview,omitempty"`
}

type ApplyRequest struct {
	Server     string
	Value      string
	Mode       string
	HeaderName string
}

type Adapter interface {
	Match(mcpproxy.Server) bool
	CredentialView(context.Context, mcpproxy.Server) (View, error)
	Apply(context.Context, ApplyRequest) error
}
