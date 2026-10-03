package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	sidekickevents "github.com/GodsQuantum/mcpproxy-sidekick/internal/events"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	emit := func(ev sidekickevents.Event) error {
		payload, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	if err := emit(sidekickevents.Event{Kind: "ready", Event: "sidekick.ready"}); err != nil {
		return
	}
	relay := sidekickevents.Relay{Source: s.Proxy}
	err := relay.Run(r.Context(), emit)
	if r.Context().Err() != nil {
		return
	}
	if err != nil {
		_ = emit(sidekickevents.Event{Kind: "fallback", Event: "mcpproxy.events.unavailable"})
		return
	}
	// A clean EOF from an upstream event stream is still unexpected while the
	// authenticated browser session is alive. Tell the UI to enter polling
	// fallback and reconnect instead of silently leaving it with stale state.
	_ = emit(sidekickevents.Event{Kind: "fallback", Event: "mcpproxy.events.closed"})
}
