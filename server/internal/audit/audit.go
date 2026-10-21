package audit

import (
	"context"
	"time"
)

// Event ...
type Event struct {
	Time         time.Time `json:"time"`
	Action       string    `json:"action"`
	ActorType    string    `json:"actor_type,omitempty"`
	ActorID      string    `json:"actor_id,omitempty"`
	ResourceType string    `json:"resource_type,omitempty"`
	ResourceID   string    `json:"resource_id,omitempty"`
	Before       string    `json:"before,omitempty"`
	After        string    `json:"after,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	RequestID    string    `json:"request_id,omitempty"`
	Card         string    `json:"card,omitempty"`
	AgentID      string    `json:"agent_id,omitempty"`
	Machine      string    `json:"machine,omitempty"`
	IP           string    `json:"ip,omitempty"`
	Detail       string    `json:"detail,omitempty"`
}

// Filter ...
type Filter struct {
	Action       string
	ActorType    string
	ActorID      string
	ResourceType string
	ResourceID   string
	Card         string
	AgentID      string
	Machine      string
	IP           string
	From         time.Time
	To           time.Time
	Query        string // Used for detail matching
}

// Repository defines the storage interface for audit events.
type Repository interface {
	Insert(ctx context.Context, event Event) error
	List(ctx context.Context, filter Filter, limit int) ([]Event, error)
}

// Recorder ...
type Recorder struct {
	repo Repository
}

// NewRecorder ...
func NewRecorder(repo Repository) *Recorder {
	return &Recorder{repo: repo}
}

// Append writes the event to the repository synchronously and returns any error.
func (r *Recorder) Append(event Event) error {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	if r.repo != nil {
		if err := r.repo.Insert(context.Background(), event); err != nil {
			return err
		}
	}
	return nil
}

// Query ...
func (r *Recorder) Query(ctx context.Context, filter Filter, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 10000 {
		limit = 10000
	}
	if r.repo != nil {
		return r.repo.List(ctx, filter, limit)
	}
	return nil, nil
}
