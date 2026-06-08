package audit

import (
	"sync"
	"time"
)

type Event struct {
	Time    time.Time
	Action  string
	ActorID string
	Card    string
	AgentID string
	Machine string
	IP      string
	Detail  string
}

type Filter struct {
	Action  string
	ActorID string
	Card    string
	AgentID string
	Machine string
	IP      string
	From    time.Time
	To      time.Time
}

type Recorder struct {
	mu     sync.RWMutex
	events []Event
}

func NewRecorder() *Recorder {
	return &Recorder{events: make([]Event, 0)}
}

func (r *Recorder) Append(event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}

	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *Recorder) Query(filter Filter) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Event, 0)
	for _, event := range r.events {
		if !matches(event, filter) {
			continue
		}
		out = append(out, event)
	}
	return out
}

func matches(event Event, filter Filter) bool {
	if filter.Action != "" && event.Action != filter.Action {
		return false
	}
	if filter.ActorID != "" && event.ActorID != filter.ActorID {
		return false
	}
	if filter.Card != "" && event.Card != filter.Card {
		return false
	}
	if filter.AgentID != "" && event.AgentID != filter.AgentID {
		return false
	}
	if filter.Machine != "" && event.Machine != filter.Machine {
		return false
	}
	if filter.IP != "" && event.IP != filter.IP {
		return false
	}
	if !filter.From.IsZero() && event.Time.Before(filter.From) {
		return false
	}
	if !filter.To.IsZero() && event.Time.After(filter.To) {
		return false
	}
	return true
}
