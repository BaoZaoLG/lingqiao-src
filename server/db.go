package main

import (
	"database/sql"
	"fmt"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/storage"
	"log"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

// openDB opens (or creates) the SQLite database at dbPath and applies all
// required schema migrations. It returns the open *sql.DB ready for use.
func openDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := applySchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

// applySchema runs all CREATE TABLE IF NOT EXISTS statements so that the
// database schema is always up-to-date on startup.
func applySchema(db *sql.DB) error {
	stmts1 := []string{
		`CREATE TABLE IF NOT EXISTS cards (
			code         TEXT PRIMARY KEY,
			machine_id   TEXT NOT NULL DEFAULT '',
			agent_id     TEXT NOT NULL DEFAULT '',
			created_at   DATETIME NOT NULL,
			activated_at DATETIME,
			expires_at   DATETIME NOT NULL,
			status       TEXT NOT NULL DEFAULT 'active',
			note         TEXT NOT NULL DEFAULT '',
			max_sessions INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token          TEXT PRIMARY KEY,
			card_code      TEXT NOT NULL,
			machine_id     TEXT NOT NULL,
			fingerprint    TEXT NOT NULL DEFAULT '',
			client_version TEXT NOT NULL DEFAULT '',
			created_at     DATETIME NOT NULL,
			last_seen      DATETIME NOT NULL,
			expires_at     DATETIME NOT NULL,
			remote_addr    TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS blacklist (
			value      TEXT PRIMARY KEY,
			type       TEXT NOT NULL DEFAULT '',
			reason     TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			time     DATETIME NOT NULL,
			action   TEXT NOT NULL,
			card     TEXT NOT NULL DEFAULT '',
			machine  TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			detail   TEXT NOT NULL DEFAULT '',
			addr     TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_time ON audit_log(time DESC)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id         TEXT PRIMARY KEY,
			username   TEXT NOT NULL,
			password   TEXT NOT NULL,
			prefix     TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			disabled   INTEGER NOT NULL DEFAULT 0
		)`,
	}
	for _, stmt := range stmts1 {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:min(40, len(stmt))], err)
		}
	}

	if err := deduplicateAgentUsernames(db); err != nil {
		return fmt.Errorf("deduplicate agent usernames: %w", err)
	}

	stmts2 := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_username ON agents(username)`,
		`CREATE TABLE IF NOT EXISTS invites (
			code       TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			created_by TEXT NOT NULL DEFAULT '',
			used_by    TEXT NOT NULL DEFAULT '',
			used_at    DATETIME,
			max_uses   INTEGER NOT NULL DEFAULT 0,
			use_count  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_invites_created_at_code ON invites(created_at DESC, code ASC)`,
	}
	for _, stmt := range stmts2 {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:min(40, len(stmt))], err)
		}
	}

	// Normalize legacy invites rows that predate NOT NULL defaults.
	if _, err := db.Exec(`
		UPDATE invites SET
			created_by = COALESCE(created_by, ''),
			used_by = COALESCE(used_by, ''),
			max_uses = COALESCE(max_uses, 0),
			use_count = COALESCE(use_count, 0)
	`); err != nil {
		return fmt.Errorf("normalize invites defaults: %w", err)
	}

	// Add risk control columns for agents
	agentRiskColumns := []string{
		"max_session_duration_minutes INTEGER NOT NULL DEFAULT 0",
		"max_daily_sessions INTEGER NOT NULL DEFAULT 0",
		"max_concurrent_sessions INTEGER NOT NULL DEFAULT 0",
	}
	for _, colDef := range agentRiskColumns {
		_, _ = db.Exec("ALTER TABLE agents ADD COLUMN " + colDef)
	}

	return nil
}

func deduplicateAgentUsernames(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, username FROM agents WHERE rowid NOT IN (SELECT MIN(rowid) FROM agents GROUP BY username)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type dup struct {
		id       string
		username string
	}
	var dups []dup
	for rows.Next() {
		var d dup
		if err := rows.Scan(&d.id, &d.username); err != nil {
			return err
		}
		dups = append(dups, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, d := range dups {
		base := d.username
		suffix := 1
		for {
			cand := fmt.Sprintf("%s_dup%d", base, suffix)
			var count int
			err := db.QueryRow(`SELECT COUNT(*) FROM agents WHERE username = ?`, cand).Scan(&count)
			if err != nil {
				return err
			}
			if count == 0 {
				if _, err = db.Exec(`UPDATE agents SET username = ? WHERE id = ?`, cand, d.id); err != nil {
					return err
				}
				break
			}
			suffix++
		}
	}
	return nil
}

// migrateJSONToSQLite copies cards, agents, invites, blacklist, and audit
// entries from oldCm (JSON-backed) into db (SQLite) when db is empty.
// This one-time migration is skipped on subsequent starts.
func migrateJSONToSQLite(db *sql.DB, storage storage.Store) error {
	// Count existing cards — if any rows exist, migration already ran.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&count); err != nil || count > 0 {
		return nil
	}

	log.Println("[DB] Migrating from JSON to SQLite...")

	// Load legacy JSON data
	type legacyCard struct {
		Code        string  `json:"code"`
		MachineID   string  `json:"machine_id"`
		AgentID     string  `json:"agent_id"`
		CreatedAt   string  `json:"created_at"`
		ActivatedAt *string `json:"activated_at"`
		ExpiresAt   string  `json:"expires_at"`
		Status      string  `json:"status"`
		Note        string  `json:"note"`
		MaxSessions int     `json:"max_sessions"`
	}
	type legacySession struct {
		Token         string `json:"token"`
		CardCode      string `json:"card_code"`
		MachineID     string `json:"machine_id"`
		Fingerprint   string `json:"fingerprint"`
		ClientVersion string `json:"client_version"`
		CreatedAt     string `json:"created_at"`
		LastSeen      string `json:"last_seen"`
		ExpiresAt     string `json:"expires_at"`
		RemoteAddr    string `json:"remote_addr"`
	}
	type legacyAgent struct {
		ID        string `json:"id"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		Prefix    string `json:"prefix"`
		CreatedAt string `json:"created_at"`
		Disabled  bool   `json:"disabled"`
	}
	type legacyAudit struct {
		Time    string `json:"time"`
		Action  string `json:"action"`
		Card    string `json:"card"`
		Machine string `json:"machine"`
		AgentID string `json:"agent_id"`
		Detail  string `json:"detail"`
		Addr    string `json:"addr"`
	}
	type legacyData struct {
		Cards     map[string]legacyCard    `json:"cards"`
		Sessions  map[string]legacySession `json:"sessions"`
		Blacklist []cards.BlacklistEntry   `json:"blacklist"`
		Agents    map[string]legacyAgent   `json:"agents"`
		AuditLog  []legacyAudit            `json:"audit"`
	}

	var data legacyData
	_ = storage.Load("cards", &data)
	if len(data.Cards) == 0 && len(data.Agents) == 0 {
		// Attempt loading from "data" key used by older installs
		_ = storage.Load("data", &data)
	}
	if len(data.Cards) == 0 && len(data.Agents) == 0 && len(data.Sessions) == 0 && len(data.Blacklist) == 0 {
		log.Println("[DB] No legacy JSON data to migrate.")
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migration: could not begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, c := range data.Cards {
		if c.Status == "" {
			c.Status = "active"
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO cards
			(code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.Code, c.MachineID, c.AgentID, c.CreatedAt, c.ActivatedAt, c.ExpiresAt,
			c.Status, c.Note, c.MaxSessions,
		); err != nil {
			return fmt.Errorf("migration: insert card %s: %w", c.Code, err)
		}
	}
	for _, s := range data.Sessions {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO sessions
			(token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.Token, s.CardCode, s.MachineID, s.Fingerprint, s.ClientVersion, s.CreatedAt, s.LastSeen, s.ExpiresAt, s.RemoteAddr,
		); err != nil {
			return fmt.Errorf("migration: insert session %s: %w", s.Token, err)
		}
	}
	for _, b := range data.Blacklist {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO blacklist (value, type, reason, created_at) VALUES (?, ?, ?, ?)`,
			b.Value, b.Type, b.Reason, b.CreatedAt,
		); err != nil {
			return fmt.Errorf("migration: insert blacklist %s: %w", b.Value, err)
		}
	}
	for id, a := range data.Agents {
		if a.ID == "" {
			a.ID = id
		}
		currentUsername := a.Username
		suffixLen := 4
		dupCounter := 0
		for {
			_, err := tx.Exec(`INSERT INTO agents (id, username, password, prefix, created_at, disabled) VALUES (?, ?, ?, ?, ?, ?)`,
				a.ID, currentUsername, a.Password, a.Prefix, a.CreatedAt, a.Disabled,
			)
			if err == nil {
				break
			}
			if strings.Contains(err.Error(), "UNIQUE constraint failed: agents.username") {
				if suffixLen <= len(a.ID) {
					currentUsername = fmt.Sprintf("%s-%s", a.Username, a.ID[:suffixLen])
					suffixLen++
				} else {
					dupCounter++
					currentUsername = fmt.Sprintf("%s-%s_dup%d", a.Username, a.ID, dupCounter)
				}
				continue
			}
			return fmt.Errorf("migration: insert agent %s: %w", a.ID, err)
		}
	}
	for _, e := range data.AuditLog {
		if _, err := tx.Exec(`INSERT INTO audit_log (time, action, card, machine, agent_id, detail, addr) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			e.Time, e.Action, e.Card, e.Machine, e.AgentID, e.Detail, e.Addr,
		); err != nil {
			return fmt.Errorf("migration: insert audit log %s: %w", e.Action, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration: commit tx: %w", err)
	}

	log.Println("[DB] Migration completed successfully.")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// dbPath returns the absolute path to the SQLite database file.
func dbFilePath(dataDir string) string {
	return dataDir + string(os.PathSeparator) + "server.db"
}
