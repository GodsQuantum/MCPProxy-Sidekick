package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreflightRequiresOperatorSessionAndCSRF(t *testing.T) {
	forwarded := 0
	h := newConnectionHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/preflight" {
			forwarded++
			if r.Header.Get("X-API-Key") != "admin-key" {
				t.Error("admin credential missing")
			}
			var req struct {
				Profile string `json:"profile"`
				Tools   []struct {
					ID string `json:"id"`
				} `json:"tools"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
			}
			if req.Profile != "family" || len(req.Tools) != 2 || req.Tools[0].ID != "media:stats" {
				t.Errorf("request=%+v", req)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"verdict": "degraded_retryable", "tools": []map[string]any{{"id": "media:stats", "status": "ready"}, {"id": "render:export", "status": "unavailable", "reason": "server_unhealthy", "retryable": true, "remediation": "Start render node"}}}})
			return
		}
		http.NotFound(w, r)
	})
	body := []byte(`{"profile":"family","policy":{"read_only_only":true},"tools":[{"id":" media:stats "},{"id":"render:export"}]}`)
	makeRequest := func(withCookie, withCSRF bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/preflight", bytes.NewReader(body))
		req.Header.Set("Origin", "https://mcp.example.com")
		req.Header.Set("Content-Type", "application/json")
		if withCookie {
			req.AddCookie(h.cookie)
		}
		if withCSRF {
			req.Header.Set("X-CSRF-Token", h.csrf)
		}
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		return rec
	}
	for _, tt := range []struct {
		session, csrf bool
		want          int
	}{
		{false, false, 401}, {true, false, 403}, {true, true, 200},
	} {
		res := makeRequest(tt.session, tt.csrf)
		if res.Code != tt.want {
			t.Errorf("status=%d want=%d body=%s", res.Code, tt.want, res.Body.String())
		}
	}
	if forwarded != 1 {
		t.Fatalf("forwarded=%d want 1", forwarded)
	}
	if !strings.Contains(makeRequest(true, true).Body.String(), "degraded_retryable") {
		t.Fatal("native result missing")
	}
}

func TestPreflightRejectsInvalidIDsBeforeUpstream(t *testing.T) {
	calls := 0
	h := newConnectionHarness(t, func(w http.ResponseWriter, r *http.Request) { calls++; http.NotFound(w, r) })
	inputs := []string{
		`{"tools":[]}`,
		`{"tools":[{"id":"notqualified"}]}`,
		`{"tools":[{"id":"a:b"},{"id":"a:b"}]}`,
		`{"tools":[{"id":"a:b"}],"profile":"invalid profile"}`,
		`{"tools":[{"id":"a:b"}],"evil":"pass-through"}`,
	}
	for _, body := range inputs {
		req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/preflight", strings.NewReader(body))
		req.Header.Set("Origin", "https://mcp.example.com")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.AddCookie(h.cookie)
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Errorf("unexpected %d body=%s", rec.Code, body)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid calls forwarded=%d", calls)
	}
}

func TestPreflightFailsClosedWithGenericErrorOrGracefulUnsupported(t *testing.T) {
	for _, status := range []int{404, 502} {
		h := newConnectionHarness(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("private-provider-credential"))
		})
		req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/preflight", strings.NewReader(`{"tools":[{"id":"a:b"}]}`))
		req.Header.Set("Origin", "https://mcp.example.com")
		req.Header.Set("X-CSRF-Token", h.csrf)
		req.AddCookie(h.cookie)
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		want := 502
		if status == 404 {
			want = 501
		}
		if rec.Code != want || strings.Contains(rec.Body.String(), "private-provider-credential") {
			t.Errorf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
}
