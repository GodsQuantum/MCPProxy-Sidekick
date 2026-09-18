package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeadersSetsSecurityBaseline(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	Headers(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://mcp.example.com/", nil))
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("missing CSP")
	}
}
