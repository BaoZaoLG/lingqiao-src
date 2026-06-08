package audit

import (
	"testing"
	"time"
)

func TestRecorderAppendsAndQueriesEvents(t *testing.T) {
	rec := NewRecorder()
	now := time.Now()

	rec.Append(Event{Time: now, Action: "card_generated", ActorID: "admin", Card: "ABC"})
	rec.Append(Event{Time: now.Add(time.Second), Action: "agent_login", ActorID: "agent-1"})

	events := rec.Query(Filter{Action: "card_generated"})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Card != "ABC" {
		t.Fatalf("Card = %q, want ABC", events[0].Card)
	}
}

func TestRecorderQueryByTimeRange(t *testing.T) {
	rec := NewRecorder()
	base := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)

	rec.Append(Event{Time: base.Add(-time.Hour), Action: "old"})
	rec.Append(Event{Time: base, Action: "inside"})
	rec.Append(Event{Time: base.Add(time.Hour), Action: "new"})

	events := rec.Query(Filter{From: base.Add(-time.Minute), To: base.Add(time.Minute)})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Action != "inside" {
		t.Fatalf("Action = %q, want inside", events[0].Action)
	}
}
