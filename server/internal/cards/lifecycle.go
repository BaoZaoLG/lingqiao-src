package cards

import "time"

// Status ...
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusExpired  Status = "expired"
)

// Card ...
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

// Clock ...
type Clock func() time.Time
