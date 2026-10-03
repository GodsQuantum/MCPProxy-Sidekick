package capabilities

import (
	"context"
	"time"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type Probe interface {
	Info(context.Context) (mcpproxy.Info, error)
	ProbeEndpoint(context.Context, string) (bool, error)
}

type State struct {
	Version       string `json:"version"`
	ProfileV3     bool   `json:"profile_v3"`
	Clients       bool   `json:"clients"`
	Attention     bool   `json:"attention"`
	AccessExplain bool   `json:"access_explain"`
}

func Detect(ctx context.Context, probe Probe) State {
	info, err := probe.Info(ctx)
	if err != nil {
		return State{}
	}
	state := State{Version: info.Version}
	state.ProfileV3 = probePath(ctx, probe, "/api/v1/profiles/try")
	state.Clients = probePath(ctx, probe, "/api/v1/clients")
	state.Attention = probePath(ctx, probe, "/api/v1/attention")
	state.AccessExplain = probePath(ctx, probe, "/api/v1/access/explain")
	return state
}

func probePath(ctx context.Context, probe Probe, path string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ok, err := probe.ProbeEndpoint(probeCtx, path)
	return err == nil && ok
}
