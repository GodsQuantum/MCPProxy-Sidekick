package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreflightUsesNativeEndpointAndOnlyRequestedTools(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "wrapped"}[wrapped], func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/preflight" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("X-API-Key") != "operator-test" {
					t.Errorf("missing admin authentication")
				}
				var request PreflightRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if request.Profile != "writer" || request.Policy == nil || !request.Policy.ReadOnlyOnly || len(request.Tools) != 2 || request.Tools[0].ID != "catalog:stats" || request.Tools[1].PinHash != "sha256/v5:abc" {
					t.Errorf("request=%+v", request)
				}
				result := PreflightResult{Verdict: "degraded_retryable", Tools: []PreflightToolResult{{ID: "catalog:stats", Status: "ready"}, {ID: "render:export", Status: "unavailable", Reason: "server_unhealthy", Retryable: true, Remediation: "Start render node"}}}
				if wrapped {
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": result})
				} else {
					_ = json.NewEncoder(w).Encode(result)
				}
			}))
			defer api.Close()
			client, err := NewClientWithKey(api.URL, "operator-test")
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.Preflight(context.Background(), PreflightRequest{Profile: "writer", Policy: &PreflightPolicy{ReadOnlyOnly: true}, Tools: []PreflightTool{{ID: "catalog:stats"}, {ID: "render:export", PinHash: "sha256/v5:abc"}}})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || got.Verdict != "degraded_retryable" || len(got.Tools) != 2 || !got.Tools[1].Retryable || got.Tools[1].Reason != "server_unhealthy" {
				t.Errorf("calls=%d result=%+v", calls, got)
			}
		})
	}
}
func TestPreflightDoesNotMisrepresentOlderProxyOrMalformedResponse(t *testing.T) {
	for _, tc := range []struct {
		status          int
		body            string
		wantUnsupported bool
	}{
		{404, `not found`, true},
		{405, `not allowed`, true},
		{200, `{"success":true,"data":{"tools":[]}}`, false},
		{200, `{"success":false,"data":{"verdict":"ready","tools":[{"id":"a:b","status":"ready"}]}}`, false},
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		client, _ := NewClientWithKey(s.URL, "test")
		_, err := client.Preflight(context.Background(), PreflightRequest{Tools: []PreflightTool{{ID: "a:b"}}})
		s.Close()
		if err == nil {
			t.Fatalf("expected error for status=%d body=%s", tc.status, tc.body)
		}
		if errors.Is(err, ErrPreflightUnavailable) != tc.wantUnsupported {
			t.Errorf("wrong class %v", err)
		}
	}
}
func TestPreflightResultOnlySurfacesSelectedDataFields(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"verdict":"ready","tools":[{"id":"catalog:stats","status":"ready","hash":"sha256/v5:abc","secret":"never-expose"}],"secret":"never-expose"}}`))
	}))
	defer s.Close()
	c, _ := NewClientWithKey(s.URL, "test")
	v, err := c.Preflight(context.Background(), PreflightRequest{Tools: []PreflightTool{{ID: "catalog:stats"}}})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "never-expose") {
		t.Fatal("upstream extra fields leaked")
	}
}
