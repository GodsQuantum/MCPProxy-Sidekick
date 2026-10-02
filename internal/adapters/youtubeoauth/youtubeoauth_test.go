package youtubeoauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusAndStart(t *testing.T) {
	var startCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/status":
			_ = json.NewEncoder(w).Encode(Status{
				Configured: true, CompleteConfigured: true, Profile: "full",
				Handle: "@arezkisugar", ChannelID: "UC123",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/start":
			startCalls++
			_ = json.NewEncoder(w).Encode(StartResult{
				AuthURL: "https://accounts.google.com/o/oauth2/v2/auth?x=1",
				Status: Status{Configured: true, CompleteConfigured: true, Profile: "full"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := Adapter{BaseURL: srv.URL}
	got, err := a.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Configured || !got.CompleteConfigured || got.Handle != "@arezkisugar" {
		t.Fatalf("status=%#v", got)
	}
	start, err := a.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if start.AuthURL == "" || startCalls != 1 {
		t.Fatalf("start=%#v calls=%d", start, startCalls)
	}
}

func TestMatchesServer(t *testing.T) {
	for _, name := range []string{"youtube-arezki", "youtube-creator-arezki"} {
		if !MatchesServer(name) {
			t.Fatalf("%q should match", name)
		}
	}
	if MatchesServer("youtube-other") {
		t.Fatal("unexpected match")
	}
}
