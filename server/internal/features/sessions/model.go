package sessions

import "time"

type Session struct {
	Token         string    `json:"token"`
	CardCode      string    `json:"card_code"`
	MachineID     string    `json:"machine_id"`
	Fingerprint   string    `json:"fingerprint"`
	ClientVersion string    `json:"client_version"`
	CreatedAt     time.Time `json:"created_at"`
	LastSeen      time.Time `json:"last_seen"`
	ExpiresAt     time.Time `json:"expires_at"`
	RemoteAddr    string    `json:"remote_addr"`
}

type CardSnapshot struct {
	Code        string
	Status      string
	Note        string
	MachineID   string
	AgentID     string
	CreatedAt   time.Time
	ActivatedAt *time.Time
	ExpiresAt   time.Time
	MaxSessions int
}

type ActivationInput struct {
	CardCode      string
	MachineID     string
	Fingerprint   string
	RemoteAddr    string
	ClientVersion string
}

type HeartbeatInput struct {
	SessionToken  string
	MachineID     string
	RemoteAddr    string
	ClientVersion string
}

type DeactivateInput struct {
	SessionToken string
	MachineID    string
	CardCode     string
	Unbind       bool
	IsAdminForce bool
}

type ListFilter struct {
	CardCode   string
	MachineID  string
	ActiveOnly bool
	Page       int
	PerPage    int
}

type ListResult struct {
	Items   []Session
	Total   int
	Page    int
	PerPage int
}
