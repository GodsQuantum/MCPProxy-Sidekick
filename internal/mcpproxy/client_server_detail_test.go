package mcpproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetServerFindsNamedServer(t *testing.T) {
	client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/servers" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]Server{
			{Name: "alpha"},
			{Name: "github", Status: "ready", ToolCount: 94},
		})
	})
	got, err := client.GetServer(context.Background(), "github")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "github" || got.ToolCount != 94 {
		t.Fatalf("server=%#v", got)
	}
}

func TestGetServerRejectsUnknownServer(t *testing.T) {
	client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Server{{Name: "alpha"}})
	})
	if _, err := client.GetServer(context.Background(), "missing"); err == nil {
		t.Fatal("expected unknown server error")
	}
}

func TestRestartServerUsesCanonicalEndpoint(t *testing.T) {
	var path string
	client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := client.RestartServer(context.Background(), "github"); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v1/servers/github/restart" {
		t.Fatalf("path=%q", path)
	}
}
