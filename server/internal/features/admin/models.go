package admin

import (
	"time"
)

type Role string

const (
	RoleOwner          Role = "owner"
	RoleOperator       Role = "operator"
	RoleSupport        Role = "support"
	RoleReleaseManager Role = "release_manager"
	RoleAgentManager   Role = "agent_manager"
)

const DefaultOwnerUsername = "admin"

type AdminUser struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         Role       `json:"role"`
	Disabled     bool       `json:"disabled"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

type AdminSession struct {
	TokenHash  string     `json:"-"`
	AdminID    string     `json:"admin_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	RemoteAddr string     `json:"remote_addr"`
	UserAgent  string     `json:"user_agent"`
}

var RolePermissions = map[Role][]string{
	RoleOwner: {
		"cards.read", "cards.write", "cards.bulk",
		"sessions.read", "sessions.force_logout",
		"agents.read", "agents.write",
		"invites.write",
		"releases.read", "releases.write",
		"scripts.write", "payloads.write",
		"audit.read", "system.read", "admins.write",
	},
	RoleOperator: {
		"cards.read", "cards.write", "cards.bulk",
		"sessions.read", "sessions.force_logout",
		"audit.read",
	},
	RoleSupport: {
		"cards.read", "sessions.read", "sessions.force_logout",
	},
	RoleReleaseManager: {
		"releases.read", "releases.write",
		"scripts.write", "payloads.write",
	},
	RoleAgentManager: {
		"agents.read", "agents.write",
		"invites.write",
	},
}

func HasPermission(role Role, perm string) bool {
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}
