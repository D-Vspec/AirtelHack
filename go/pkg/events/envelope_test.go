package events

import (
	"encoding/json"
	"regexp"
	"testing"
)

func TestNewIDIsUUIDv4(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewID()
		if !re.MatchString(id) {
			t.Fatalf("bad uuid %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	env, err := New(TopicSystemPing, "", "", PingPayload{Message: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if env.CorrelationID == "" || env.Version != 1 || env.Type != "system.ping" {
		t.Fatalf("unexpected envelope %+v", env)
	}
	b, _ := json.Marshal(env)
	var back Envelope
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	var p PingPayload
	_ = json.Unmarshal(back.Payload, &p)
	if p.Message != "hi" || back.ID != env.ID {
		t.Fatalf("round trip mismatch %+v %+v", back, p)
	}
}

func TestEveryFindingHasDefaultRoute(t *testing.T) {
	if len(DefaultRoute) == 0 {
		t.Fatal("no default routes generated")
	}
	for code, route := range DefaultRoute {
		if route == "" {
			t.Fatalf("finding %s has empty route", code)
		}
	}
}
