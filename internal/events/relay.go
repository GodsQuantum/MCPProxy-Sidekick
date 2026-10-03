package events

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

type Source interface {
	OpenEvents(context.Context) (io.ReadCloser, error)
}

type Event struct {
	Kind  string `json:"kind"`
	Event string `json:"event"`
}

type Relay struct {
	Source          Source
	DuplicateWindow time.Duration
	Now             func() time.Time
}

func Normalize(eventType string) (Event, bool) {
	eventType = strings.TrimSpace(eventType)
	switch {
	case eventType == "servers.changed":
		return Event{Kind: "connections", Event: eventType}, true
	case eventType == "config.reloaded" || eventType == "config.saved" || eventType == "secrets.changed":
		return Event{Kind: "state", Event: eventType}, true
	case strings.HasPrefix(eventType, "profile.") || strings.HasPrefix(eventType, "profiles.") ||
		strings.HasPrefix(eventType, "tool.") || strings.HasPrefix(eventType, "tools."):
		return Event{Kind: "state", Event: eventType}, true
	case strings.HasPrefix(eventType, "oauth."):
		return Event{Kind: "oauth", Event: eventType}, true
	case eventType == "activity" || strings.HasPrefix(eventType, "activity."):
		return Event{Kind: "activity", Event: eventType}, true
	case strings.HasPrefix(eventType, "security.") || strings.HasPrefix(eventType, "sensitive_data."):
		return Event{Kind: "security", Event: eventType}, true
	default:
		return Event{}, false
	}
}

func (r Relay) Run(ctx context.Context, emit func(Event) error) error {
	if r.Source == nil {
		return errors.New("event source is not configured")
	}
	if emit == nil {
		return errors.New("event emitter is not configured")
	}
	body, err := r.Source.OpenEvents(ctx)
	if err != nil {
		return err
	}
	defer body.Close()

	now := r.Now
	if now == nil {
		now = time.Now
	}
	window := r.DuplicateWindow
	if window <= 0 {
		window = 150 * time.Millisecond
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var eventType string
	sawData := false
	lastKey := ""
	var lastAt time.Time

	flush := func() error {
		if eventType == "" || !sawData {
			eventType = ""
			sawData = false
			return nil
		}
		ev, ok := Normalize(eventType)
		eventType = ""
		sawData = false
		if !ok {
			return nil
		}
		at := now()
		key := ev.Kind + "\x00" + ev.Event
		if key == lastKey && !lastAt.IsZero() && at.Sub(lastAt) < window {
			return nil
		}
		lastKey, lastAt = key, at
		return emit(ev)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			// Deliberately discard the entire upstream data payload. Sidekick
			// relays only a normalized invalidation type so credentials, tool
			// arguments, OAuth data and server headers cannot cross this boundary.
			sawData = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}
