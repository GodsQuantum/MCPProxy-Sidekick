package events

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

type fakeSource struct{ stream string }

func (f fakeSource) OpenEvents(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.stream)), nil
}

func TestRelayNormalizesKnownEventsAndNeverForwardsPayload(t *testing.T) {
	const secret = "SUPER_SECRET_SSE_VALUE"
	stream := "" +
		"event: servers.changed\n" +
		"data: {\"payload\":{\"servers\":[{\"headers\":{\"Authorization\":\"Bearer " + secret + "\"}}]}}\n\n" +
		"event: oauth.failed\n" +
		"data: {\"access_token\":\"" + secret + "\"}\n\n" +
		"event: config.reloaded\ndata: {\"api_key\":\"" + secret + "\"}\n\n" +
		"event: security.scan.completed\ndata: {\"cookie\":\"" + secret + "\"}\n\n" +
		"event: activity.completed\ndata: {\"arguments\":\"" + secret + "\"}\n\n" +
		"event: ping\ndata: {}\n\n" +
		"event: totally.unknown\ndata: {\"secret\":\"" + secret + "\"}\n\n"

	var got []Event
	relay := Relay{Source: fakeSource{stream: stream}, DuplicateWindow: time.Second}
	if err := relay.Run(context.Background(), func(e Event) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("events=%#v", got)
	}
	wantKinds := []string{"connections", "oauth", "state", "security", "activity"}
	for i, want := range wantKinds {
		if got[i].Kind != want {
			t.Fatalf("event %d kind=%q want=%q", i, got[i].Kind, want)
		}
		if strings.Contains(got[i].Event, secret) {
			t.Fatal("secret leaked through normalized event")
		}
	}
}

func TestRelayCoalescesImmediateDuplicateEvents(t *testing.T) {
	stream := "event: servers.changed\ndata: {}\n\nevent: servers.changed\ndata: {}\n\n"
	now := time.Unix(100, 0)
	relay := Relay{
		Source:          fakeSource{stream: stream},
		DuplicateWindow: time.Second,
		Now:             func() time.Time { return now },
	}
	var got []Event
	if err := relay.Run(context.Background(), func(e Event) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("duplicate events were not coalesced: %#v", got)
	}
}

func TestNormalizeDropsHeartbeatAndUnknownTypes(t *testing.T) {
	for _, name := range []string{"ping", "status", "something.untrusted", ""} {
		if _, ok := Normalize(name); ok {
			t.Fatalf("%q should be dropped", name)
		}
	}
}
