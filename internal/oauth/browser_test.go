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

func (fakeStarter) LogoutOAuth(context.Context, string) error { return nil }

func (fakeStarter) StartOAuth(context.Context, string) (mcpproxy.OAuthStart, error) {
	return mcpproxy.OAuthStart{AuthURL: "https://provider.example/authorize?x=1"}, nil
}

func TestBrowserOpenClearsStaleTabsBeforeCDPNewTab(t *testing.T) {
	var closed, opened, openedBeforeClose bool
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/list":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "old-tab", "type": "page", "url": "https://stale.example"},
				{"id": "worker-1", "type": "service_worker"},
			})
		case "/json/close/old-tab":
			if r.Method != http.MethodGet {
				t.Errorf("close method = %q", r.Method)
			}
			closed = true
			w.WriteHeader(http.StatusOK)
		case "/json/new":
			if r.Method != http.MethodPut {
				t.Errorf("new tab method = %q", r.Method)
			}
			openedBeforeClose = !closed
			opened = true
			rawQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "tab-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	b := Browser{CDPBaseURL: srv.URL, PublicSessionURL: "/oauth-browser/"}
	if err := b.Open(context.Background(), "https://provider.example/authorize?x=1"); err != nil {
		t.Fatal(err)
	}
	if !closed || !opened {
		t.Fatalf("closed=%v opened=%v", closed, opened)
	}
	if openedBeforeClose {
		t.Fatal("new OAuth tab opened before stale tab was closed")
	}
	if !strings.Contains(rawQuery, "https%3A%2F%2Fprovider.example%2Fauthorize") {
		t.Fatalf("query = %q", rawQuery)
	}
}

func TestServiceStartReturnsVisibleBrowserURL(t *testing.T) {
	var opened string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/list":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case "/json/new":
			opened = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "tab-1"})
		default:
			http.NotFound(w, r)
		}
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
