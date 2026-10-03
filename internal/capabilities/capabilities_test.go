package capabilities

import (
	"context"
	"errors"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type fakeProbe struct {
	info      mcpproxy.Info
	infoErr   error
	endpoints map[string]bool
	errs      map[string]error
}

func (f fakeProbe) Info(context.Context) (mcpproxy.Info, error) {
	return f.info, f.infoErr
}

func (f fakeProbe) ProbeEndpoint(_ context.Context, path string) (bool, error) {
	if err := f.errs[path]; err != nil {
		return false, err
	}
	return f.endpoints[path], nil
}

func TestDetectBaselineV069WithoutOptionalEndpoints(t *testing.T) {
	state := Detect(context.Background(), fakeProbe{
		info: mcpproxy.Info{Version: "v0.69.0"},
	})
	if state.Version != "v0.69.0" {
		t.Fatalf("version=%q", state.Version)
	}
	if state.ProfileV3 || state.Clients || state.Attention || state.AccessExplain {
		t.Fatalf("unexpected optional capabilities: %#v", state)
	}
}

func TestDetectMarksOptionalCapabilitiesWhenEndpointsExist(t *testing.T) {
	state := Detect(context.Background(), fakeProbe{
		info: mcpproxy.Info{Version: "v0.70.0"},
		endpoints: map[string]bool{
			"/api/v1/profiles/try":   true,
			"/api/v1/clients":        true,
			"/api/v1/attention":      true,
			"/api/v1/access/explain": true,
		},
	})
	if !state.ProfileV3 || !state.Clients || !state.Attention || !state.AccessExplain {
		t.Fatalf("missing capabilities: %#v", state)
	}
}

func TestDetectTransportFailureReturnsBaselineState(t *testing.T) {
	state := Detect(context.Background(), fakeProbe{
		infoErr: errors.New("offline"),
		endpoints: map[string]bool{
			"/api/v1/clients": true,
		},
	})
	if state.Version != "" || state.ProfileV3 || state.Clients || state.Attention || state.AccessExplain {
		t.Fatalf("state=%#v", state)
	}
}
