package cards

import (
	"context"
	"time"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusExpired  Status = "expired"
)

type Card struct {
	Code        string
	MachineID   string
	AgentID     string
	CreatedAt   time.Time
	ActivatedAt *time.Time
	ExpiresAt   time.Time
	Status      Status
	Note        string
	MaxSessions int
}

type Repository interface {
	Get(ctx context.Context, code string) (Card, error)
	Save(ctx context.Context, card Card) error
	List(ctx context.Context, filter ListFilter) ([]Card, error)
}

type ListFilter struct {
	Query     string
	Status    Status
	MachineID string
	AgentID   string
}
