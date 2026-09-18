package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type fakeStarter struct{}

func (fakeStarter) StartOAuth(context.Context, string) (mcpproxy.OAuthStart, error) {
	return mcpproxy.OAuthStart{AuthURL: "https://provider.example/authorize?x=1"}, nil
}

func TestBrowserOpenUsesCDPNewTab(t *testing.T) {
	var method, rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		rawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "tab-1"})
	}))
	defer srv.Close()

	b := Browser{CDPBaseURL: srv.URL, PublicSessionURL: "/oauth-browser/"}
	if err := b.Open(context.Background(), "https://provider.example/authorize?x=1"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut {
		t.Fatalf("method = %q", method)
	}
	if !strings.Contains(rawQuery, "https%3A%2F%2Fprovider.example%2Fauthorize") {
		t.Fatalf("query = %q", rawQuery)
	}
}

func TestServiceStartReturnsVisibleBrowserURL(t *testing.T) {
	var opened string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		opened = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "tab-1"})
	}))
	defer srv.Close()

	s := Service{
		Starter: fakeStarter{},
		Browser: Browser{CDPBaseURL: srv.URL, PublicSessionURL: "/oauth-browser/"},
	}
	result, err := s.Start(context.Background(), "canva")
	if err != nil {
		t.Fatal(err)
	}
	if result.BrowserURL != "/oauth-browser/" {
		t.Fatalf("BrowserURL = %q", result.BrowserURL)
	}
	if opened == "" {
		t.Fatal("browser was not navigated")
	}
}
