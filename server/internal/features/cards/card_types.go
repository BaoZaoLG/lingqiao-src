package cards

import "time"

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var crockfordDecode [256]int8

func init() {
	for i := range crockfordDecode {
		crockfordDecode[i] = -1
	}
	for i, c := range crockford {
		crockfordDecode[c] = int8(i)
		if c >= 'A' && c <= 'Z' {
			crockfordDecode[c+32] = int8(i)
		}
	}
}

// BlacklistEntry is a legacy JSON struct retained for the one-time
// JSON→SQLite migration path in db.go.
type BlacklistEntry struct {
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// MachineInfo holds aggregated info about a machine used by the admin UI.
type MachineInfo struct {
	MachineID     string     `json:"machine_id"`
	CardCount     int        `json:"card_count"`
	LastSeen      *time.Time `json:"last_seen,omitempty"`
	IsBlacklisted bool       `json:"is_blacklisted"`
}
