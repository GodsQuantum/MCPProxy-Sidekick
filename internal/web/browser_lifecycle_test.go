package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
)

func TestEnsureActiveBrowserLaunchLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "launches stopped browser", status: http.StatusOK},
		{name: "accepts already running", status: http.StatusConflict},
		{name: "rejects launcher failure", status: http.StatusInternalServerError, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			launcher := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost {
					t.Fatalf("method=%s", r.Method)
				}
				w.WriteHeader(tc.status)
			}))
			defer launcher.Close()

			instance := config.BrowserInstance{
				ID:         "cloak-primary",
				Label:      "Primary",
				CDPURL:     "http://manager:8080/api/profiles/a/cdp",
				BrowserURL: "https://browser.example.com/",
				LaunchURL:  launcher.URL + "/api/profiles/a/launch",
			}
			app := &Server{Cfg: config.Config{
				BrowserInstance:  instance.ID,
				BrowserInstances: []config.BrowserInstance{instance},
			}}
			got, err := app.ensureActiveBrowser(t.Context())
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if got.ID != instance.ID {
				t.Fatalf("instance=%q", got.ID)
			}
			if calls != 1 {
				t.Fatalf("launch calls=%d", calls)
			}
		})
	}
}

func TestEnsureActiveBrowserWithoutLaunchURLIsNoop(t *testing.T) {
	instance := config.BrowserInstance{ID: "legacy", CDPURL: "http://browser:9222", BrowserURL: "/browser/"}
	app := &Server{Cfg: config.Config{BrowserInstance: instance.ID, BrowserInstances: []config.BrowserInstance{instance}}}
	got, err := app.ensureActiveBrowser(t.Context())
	if err != nil || got.ID != instance.ID {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}
