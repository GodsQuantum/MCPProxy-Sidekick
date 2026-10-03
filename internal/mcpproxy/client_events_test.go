package mcpproxy

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestOpenEventsUsesHeaderAuthAndSSEAccept(t *testing.T) {
	client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "admin-key" {
			t.Fatalf("api key=%q", got)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("accept=%q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: ping\ndata: {}\n\n")
	})
	body, err := client.OpenEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "event: ping\ndata: {}\n\n" {
		t.Fatalf("stream=%q", raw)
	}
}

func TestOpenEventsRejectsNonSSEOrErrorStatus(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "secret-shaped-upstream-error", http.StatusServiceUnavailable)
		})
		if _, err := client.OpenEvents(context.Background()); err == nil {
			t.Fatal("expected status error")
		}
	})
	t.Run("content type", func(t *testing.T) {
		client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "{}")
		})
		if _, err := client.OpenEvents(context.Background()); err == nil {
			t.Fatal("expected content type error")
		}
	})
}
