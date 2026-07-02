package audit

import (
	"context"
	"time"
)

// Event ...
type Event struct {
	Time    time.Time `json:"time"`
	Action  string    `json:"action"`
	ActorID string    `json:"actor_id,omitempty"`
	Card    string    `json:"card,omitempty"`
	AgentID string    `json:"agent_id,omitempty"`
	Machine string    `json:"machine,omitempty"`
	IP      string    `json:"ip,omitempty"`
	Detail  string    `json:"detail,omitempty"`
}

// Filter ...
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

// Repository defines the storage interface for audit events.
type Repository interface {
	Insert(ctx context.Context, event Event) error
	List(ctx context.Context, limit int) ([]Event, error)
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
func (r *Recorder) Query(ctx context.Context, limit int) ([]Event, error) {
	if r.repo != nil {
		return r.repo.List(ctx, limit)
	}
	return nil, nil
}
