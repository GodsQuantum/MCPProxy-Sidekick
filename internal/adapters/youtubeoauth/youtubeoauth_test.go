package youtubeoauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdapterStatusAndStart(t *testing.T) {
	var starts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"configured":          true,
				"complete_configured": true,
				"profile":             "full",
				"handle":              "@examplecreator",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/start":
			starts++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"auth_url":            "https://accounts.google.com/o/oauth2/v2/auth?client_id=test",
				"configured":          true,
				"complete_configured": true,
				"profile":             "full",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := Adapter{BaseURL: srv.URL}
	st, err := a.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Configured || !st.CompleteConfigured || st.Handle != "@examplecreator" {
		t.Fatalf("unexpected status: %#v", st)
	}
	start, err := a.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if starts != 1 || start.AuthURL == "" {
		t.Fatalf("starts=%d authURL=%q", starts, start.AuthURL)
	}
}

func TestConnectedForServer(t *testing.T) {
	st := Status{Configured: true, CompleteConfigured: false}
	if !ConnectedForServer("youtube-creator-primary", st) {
		t.Fatal("creator should use creator token status")
	}
	if ConnectedForServer("youtube-primary", st) {
		t.Fatal("complete should use complete bundle status")
	}
	st.CompleteConfigured = true
	if !ConnectedForServer("youtube-primary", st) {
		t.Fatal("complete should be connected once bundle exists")
	}
	if ConnectedForServer("video-primary", st) {
		t.Fatal("unexpected match")
	}
	if MatchesServer("youtube-") || MatchesServer("youtube-creator-") {
		t.Fatal("empty identity suffix should not match")
	}
}
