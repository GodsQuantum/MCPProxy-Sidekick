package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type fakeStarter struct{}

func (fakeStarter) LogoutOAuth(context.Context, string) error { return nil }

func (fakeStarter) StartOAuth(context.Context, string) (mcpproxy.OAuthStart, error) {
	return mcpproxy.OAuthStart{AuthURL: "https://provider.example/authorize?x=1"}, nil
}

func TestBrowserOpenClearsStaleTabsBeforeCDPNewTab(t *testing.T) {
	var closed, opened, openedBeforeClose bool
	var listClose, closeClose, newClose bool
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/list":
			listClose = r.Close
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "old-tab", "type": "page", "url": "https://stale.example"},
				{"id": "worker-1", "type": "service_worker"},
			})
		case "/json/close/old-tab":
			if r.Method != http.MethodGet {
				t.Errorf("close method = %q", r.Method)
			}
			closeClose = r.Close
			closed = true
			w.WriteHeader(http.StatusOK)
		case "/json/new":
			if r.Method != http.MethodPut {
				t.Errorf("new tab method = %q", r.Method)
			}
			newClose = r.Close
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
	if !listClose || !closeClose || !newClose {
		t.Fatalf("CDP requests must disable keep-alive: list=%v close=%v new=%v", listClose, closeClose, newClose)
	}
	if !strings.Contains(rawQuery, "https%3A%2F%2Fprovider.example%2Fauthorize") {
		t.Fatalf("query = %q", rawQuery)
	}
}

func TestBrowserOpenIgnoresStaleTabCloseRace(t *testing.T) {
	var opened bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/list":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "vanished-tab", "type": "page", "url": "https://stale.example"},
			})
		case "/json/close/vanished-tab":
			http.Error(w, "No such target", http.StatusInternalServerError)
		case "/json/new":
			opened = true
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
	if !opened {
		t.Fatal("new OAuth tab was not opened after stale close race")
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

type startOnlyStarter struct{}

func (startOnlyStarter) StartOAuth(context.Context, string) (mcpproxy.OAuthStart, error) {
	return mcpproxy.OAuthStart{AuthURL: "https://provider.example/authorize?x=1"}, nil
}

func TestServiceStartDoesNotRequirePreemptiveLogout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/list":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case "/json/new":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "tab-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s := Service{
		Starter: startOnlyStarter{},
		Browser: Browser{CDPBaseURL: srv.URL, PublicSessionURL: "/oauth-browser/"},
	}
	if _, err := s.Start(context.Background(), "google"); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserOpenFallsBackToCDPWebSocket(t *testing.T) {
	var srv *httptest.Server
	var created bool
	upgrader := websocket.Upgrader{}

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/list":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case "/json/new":
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		case "/json/version":
			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
			_ = json.NewEncoder(w).Encode(map[string]any{"webSocketDebuggerUrl": wsURL})
		case "/ws":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Errorf("upgrade: %v", err)
				return
			}
			defer conn.Close()
			var req map[string]any
			if err := conn.ReadJSON(&req); err != nil {
				t.Errorf("read CDP request: %v", err)
				return
			}
			if req["method"] != "Target.createTarget" {
				t.Errorf("method = %v", req["method"])
			}
			params, _ := req["params"].(map[string]any)
			if params["url"] != "https://provider.example/authorize?x=1" {
				t.Errorf("url = %v", params["url"])
			}
			created = true
			_ = conn.WriteJSON(map[string]any{
				"id":     1,
				"result": map[string]any{"targetId": "oauth-tab"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	b := Browser{CDPBaseURL: srv.URL, PublicSessionURL: "/oauth-browser/"}
	if err := b.Open(context.Background(), "https://provider.example/authorize?x=1"); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("Target.createTarget fallback was not used")
	}
}
