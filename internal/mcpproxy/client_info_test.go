package mcpproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func infoClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyFile, []byte("admin-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := NewClient(srv.URL, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestInfoDecodesDirectAndWrappedResponses(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "direct", body: `{"version":"v0.69.0"}`},
		{name: "wrapped", body: `{"success":true,"data":{"version":"v0.69.0"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("X-API-Key"); got != "admin-key" {
					t.Fatalf("api key=%q", got)
				}
				_, _ = w.Write([]byte(tc.body))
			})
			info, err := client.Info(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if info.Version != "v0.69.0" {
				t.Fatalf("version=%q", info.Version)
			}
		})
	}
}

func TestProbeEndpointDistinguishesAbsentPresentAndFailure(t *testing.T) {
	status := http.StatusNotFound
	client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})

	ok, err := client.ProbeEndpoint(context.Background(), "/api/v1/attention")
	if err != nil || ok {
		t.Fatalf("404 ok=%v err=%v", ok, err)
	}

	status = http.StatusMethodNotAllowed
	ok, err = client.ProbeEndpoint(context.Background(), "/api/v1/attention")
	if err != nil || !ok {
		t.Fatalf("405 ok=%v err=%v", ok, err)
	}

	status = http.StatusServiceUnavailable
	ok, err = client.ProbeEndpoint(context.Background(), "/api/v1/attention")
	if err == nil || ok {
		t.Fatalf("503 ok=%v err=%v", ok, err)
	}
}
