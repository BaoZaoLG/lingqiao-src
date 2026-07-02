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
	StatusArchived Status = "archived"
	StatusUnused   Status = "unused"
)

type Card struct {
	Code        string     `json:"code"`
	MachineID   string     `json:"machine_id"`
	AgentID     string     `json:"agent_id"`
	CreatedAt   time.Time  `json:"created_at"`
	ActivatedAt *time.Time `json:"activated_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Status      Status     `json:"status"`
	Note        string     `json:"note"`
	MaxSessions int        `json:"max_sessions"`
}

type Repository interface {
	Get(ctx context.Context, code string) (Card, error)
	Insert(ctx context.Context, card Card) error
	Save(ctx context.Context, card Card) error
	BatchSave(ctx context.Context, cards []Card) error
	List(ctx context.Context, filter ListFilter) ([]Card, error)
	UpdateStatusBulk(ctx context.Context, codes []string, status Status) (int, error)
	ExtendBulk(ctx context.Context, codes []string, extendHours int) (int, error)
	Delete(ctx context.Context, code string) error
	Count(ctx context.Context, filter ListFilter) (int, error)
}

type ListFilter struct {
	Query     string
	Status    Status
	MachineID string
	AgentID   string
}
