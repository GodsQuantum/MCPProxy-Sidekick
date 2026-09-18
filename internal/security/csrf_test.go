package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/auth"
)

func TestMutationRejectsMissingCSRF(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/credential", nil)
	err := RequireMutation(req, auth.Session{CSRFToken: "abc"}, []string{"mcp.example.com"})
	if err == nil {
		t.Fatal("expected missing CSRF to fail")
	}
}

func TestMutationRejectsHostileOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/credential", nil)
	req.Header.Set("X-CSRF-Token", "abc")
	req.Header.Set("Origin", "https://evil.example")
	err := RequireMutation(req, auth.Session{CSRFToken: "abc"}, []string{"mcp.example.com"})
	if err == nil {
		t.Fatal("expected hostile origin to fail")
	}
}

func TestMutationAcceptsMatchingCSRFAndOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://mcp.example.com/api/credential", nil)
	req.Header.Set("X-CSRF-Token", "abc")
	req.Header.Set("Origin", "https://mcp.example.com")
	err := RequireMutation(req, auth.Session{CSRFToken: "abc"}, []string{"mcp.example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
