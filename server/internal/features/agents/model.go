package agents

import (
	"time"
)

// Account represents an agent account.
type Account struct {
	ID                        string    `json:"id"`
	Username                  string    `json:"username"`
	Password                  string    `json:"password"`
	Prefix                    string    `json:"prefix"`
	CreatedAt                 time.Time `json:"created_at"`
	Disabled                  bool      `json:"disabled"`
	MaxSessionDurationMinutes int       `json:"max_session_duration_minutes,omitempty"`
	MaxDailySessions          int       `json:"max_daily_sessions,omitempty"`
	MaxConcurrentSessions     int       `json:"max_concurrent_sessions,omitempty"`
}

type AgentLimits struct {
	MaxSessionDurationMinutes int `json:"max_session_duration_minutes,omitempty"`
	MaxDailySessions          int `json:"max_daily_sessions,omitempty"`
	MaxConcurrentSessions     int `json:"max_concurrent_sessions,omitempty"`
}
