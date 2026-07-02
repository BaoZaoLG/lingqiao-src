package authorizations

import (
	"time"

	"github.com/lingqiao/server/internal/features/sessions"
)

type EffectiveStatus string

const (
	StatusIssued    EffectiveStatus = "issued"
	StatusActivated EffectiveStatus = "activated"
	StatusInUse     EffectiveStatus = "in_use"
	StatusExpired   EffectiveStatus = "expired"
	StatusFrozen    EffectiveStatus = "frozen"
	StatusArchived  EffectiveStatus = "archived"
)

type ListFilter struct {
	Query           string
	EffectiveStatus EffectiveStatus
	AgentID         string
	MachineID       string
	Bound           string
	ExpiresAfter    time.Time
	ExpiresBefore   time.Time
	MaxSessions     int
	IncludeArchived bool
}

type Action string

const (
	ActionExtend        Action = "extend"
	ActionUnbindMachine Action = "unbind_machine"
	ActionFreeze        Action = "freeze"
	ActionRestore       Action = "restore"
	ActionArchiveDelete Action = "archive_delete"
)

type ImpactPreview struct {
	Action         Action   `json:"action"`
	TargetCount    int      `json:"target_count"`
	ActiveSessions int      `json:"active_sessions"`
	Machines       []string `json:"machines"`
	Reversible     bool     `json:"reversible"`
	Warnings       []string `json:"warnings"`
}

type ActionResult struct {
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Failed  int      `json:"failed"`
	Codes   []string `json:"codes"`
}

type AuthorizationSummary struct {
	Code            string          `json:"code"`
	AgentID         string          `json:"agent_id,omitempty"`
	MachineID       string          `json:"machine_id,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	ActivatedAt     *time.Time      `json:"activated_at,omitempty"`
	ExpiresAt       time.Time       `json:"expires_at"`
	EffectiveStatus EffectiveStatus `json:"effective_status"`
	StoredStatus    string          `json:"stored_status"`
	Note            string          `json:"note,omitempty"`
	MaxSessions     int             `json:"max_sessions"`
	ActiveSessions  int             `json:"active_sessions"`
	LastSeen        *time.Time      `json:"last_seen,omitempty"`
}

type AuthorizationDetail struct {
	AuthorizationSummary
	Sessions []sessions.Session `json:"sessions"`
}
