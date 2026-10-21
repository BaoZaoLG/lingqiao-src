package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateJSONToSQLite(t *testing.T) {
	// Create a temp directory for JSON Storage
	tmpDir := t.TempDir()

	storage := NewJSONStorage(tmpDir)

	// Write mock data.json (missing cards.json but having old legacy fields)
	dataJSON := []byte(`{
		"cards": {
			"TEST-123": {
				"code": "TEST-123",
				"machine_id": "mach-1",
				"created_at": "2026-01-01T00:00:00Z",
				"expires_at": "2026-02-01T00:00:00Z",
				"max_sessions": 1
			}
		},
		"agents": {
			"agent-1": {
				"id": "agent-1",
				"username": "admin",
				"password": "hashed",
				"created_at": "2026-01-01T00:00:00Z"
			}
		},
		"sessions": {
			"token-1": {
				"token": "token-1",
				"card_code": "TEST-123",
				"machine_id": "mach-1",
				"created_at": "2026-01-01T00:00:00Z",
				"last_seen": "2026-01-01T00:00:00Z",
				"expires_at": "2026-02-01T00:00:00Z"
			}
		},
		"blacklist": [
			{
				"type": "machine",
				"value": "mach-bad",
				"reason": "spam",
				"created_at": "2026-01-01T00:00:00Z"
			}
		],
		"audit": [
			{
				"time": "2026-01-01T00:00:00Z",
				"action": "test_action"
			}
		]
	}`)

	if err := os.WriteFile(filepath.Join(tmpDir, "data.json"), dataJSON, 0644); err != nil {
		t.Fatalf("failed to write data.json: %v", err)
	}

	// Create an in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	if err := applySchema(db); err != nil {
		t.Fatalf("failed to apply schema: %v", err)
	}

	// 1. Test running migration with data.json but no cards.json
	if err := migrateJSONToSQLite(db, storage); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify data was imported
	var cardCount, agentCount, sessionCount, blacklistCount, auditCount int
	db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&cardCount)
	db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&agentCount)
	db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessionCount)
	db.QueryRow(`SELECT COUNT(*) FROM blacklist`).Scan(&blacklistCount)
	db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&auditCount)

	if cardCount != 1 {
		t.Errorf("expected 1 card, got %d", cardCount)
	}
	if agentCount != 1 {
		t.Errorf("expected 1 agent, got %d", agentCount)
	}
	if sessionCount != 1 {
		t.Errorf("expected 1 session, got %d", sessionCount)
	}
	if blacklistCount != 1 {
		t.Errorf("expected 1 blacklist entry, got %d", blacklistCount)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 audit entry, got %d", auditCount)
	}

	// 2. Test already migrated (should skip)
	// Even if we put new data in JSON, it should skip because db is not empty
	if err := os.WriteFile(filepath.Join(tmpDir, "cards.json"), []byte(`{"cards":{"TEST-NEW":{"code":"TEST-NEW"}}}`), 0644); err != nil {
		t.Fatalf("failed to write cards.json: %v", err)
	}

	if err := migrateJSONToSQLite(db, storage); err != nil {
		t.Fatalf("migration failed on second run: %v", err)
	}

	var newCardCount int
	db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&newCardCount)
	if newCardCount != 1 {
		t.Errorf("expected 1 card (migration skipped), got %d", newCardCount)
	}
}

func TestDeduplicateAgentUsernames(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	// Apply schema manually to skip deduplication during setup
	stmts := []string{
		`CREATE TABLE agents (
			id         TEXT PRIMARY KEY,
			username   TEXT NOT NULL,
			password   TEXT NOT NULL,
			prefix     TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			disabled   INTEGER NOT NULL DEFAULT 0
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("failed to setup schema: %v", err)
		}
	}

	// Insert duplicates
	// 1. "admin" (first row) -> keeps name
	// 2. "admin" (second row) -> will become "admin_dup1"
	// 3. "admin" (third row) -> will become "admin_dup2"
	db.Exec(`INSERT INTO agents (id, username, password, created_at) VALUES ('id-1', 'admin', 'pw', '2026-01-01')`)
	db.Exec(`INSERT INTO agents (id, username, password, created_at) VALUES ('id-2', 'admin', 'pw', '2026-01-01')`)
	db.Exec(`INSERT INTO agents (id, username, password, created_at) VALUES ('id-3', 'admin', 'pw', '2026-01-01')`)
	// 4. "admin_dup1" (already exists, will force the second admin to use a different name or force dup collision handling)
	db.Exec(`INSERT INTO agents (id, username, password, created_at) VALUES ('id-4', 'admin_dup1', 'pw', '2026-01-01')`)

	if err := deduplicateAgentUsernames(db); err != nil {
		t.Fatalf("deduplicate failed: %v", err)
	}

	// Check results
	rows, err := db.Query(`SELECT id, username FROM agents ORDER BY id`)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	results := make(map[string]string)
	for rows.Next() {
		var id, uname string
		if err := rows.Scan(&id, &uname); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		results[id] = uname
	}

	if results["id-1"] != "admin" {
		t.Errorf("id-1 should be 'admin', got %s", results["id-1"])
	}
	// id-2 and id-3 should be assigned dup suffixes that don't collide with 'admin_dup1'
	if results["id-2"] == "admin" || results["id-3"] == "admin" {
		t.Errorf("duplicates were not renamed")
	}
	if results["id-4"] != "admin_dup1" {
		t.Errorf("id-4 should retain its original name, got %s", results["id-4"])
	}

	// Create unique index to ensure no collisions
	if _, err := db.Exec(`CREATE UNIQUE INDEX idx_agents_username ON agents(username)`); err != nil {
		t.Fatalf("failed to create unique index, duplicates still exist: %v", err)
	}
}

func TestApplySchemaInvitesLegacyCompatibility(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-invites.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE invites (
			code TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			created_by TEXT,
			used_by TEXT,
			used_at DATETIME,
			max_uses INTEGER,
			use_count INTEGER
		);
		INSERT INTO invites (code, created_at, created_by, used_by, used_at, max_uses, use_count)
		VALUES ('INV-LEGACY', '2026-01-02T03:04:05Z', NULL, NULL, NULL, NULL, NULL);
	`); err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}

	if err := applySchema(db); err != nil {
		t.Fatalf("first schema application failed: %v", err)
	}
	if err := applySchema(db); err != nil {
		t.Fatalf("second schema application failed: %v", err)
	}

	var createdBy, usedBy string
	var maxUses, useCount int
	if err := db.QueryRow(`
		SELECT created_by, used_by, max_uses, use_count
		FROM invites
		WHERE code = 'INV-LEGACY'
	`).Scan(&createdBy, &usedBy, &maxUses, &useCount); err != nil {
		t.Fatalf("legacy invite defaults were not normalized: %v", err)
	}
	if createdBy != "" || usedBy != "" || maxUses != 0 || useCount != 0 {
		t.Fatalf("unexpected normalized values: created_by=%q used_by=%q max_uses=%d use_count=%d",
			createdBy, usedBy, maxUses, useCount)
	}

	var indexSQL string
	if err := db.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_invites_created_at_code'
	`).Scan(&indexSQL); err != nil {
		t.Fatalf("invite list index missing: %v", err)
	}
	if !strings.Contains(indexSQL, "created_at DESC, code ASC") {
		t.Fatalf("unexpected invite list index definition: %s", indexSQL)
	}
}

func TestMigrateJSONInvitesToSQLiteAtomicAndIdempotent(t *testing.T) {
	t.Run("imports legacy rows once", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewJSONStorage(tmpDir)
		if err := os.WriteFile(filepath.Join(tmpDir, "invites.json"), []byte(`[
			{
				"code": "INV-OLD-1",
				"created_at": "2026-01-02T03:04:05Z",
				"created_by": "legacy-admin",
				"max_uses": 2,
				"use_count": 1
			}
		]`), 0600); err != nil {
			t.Fatalf("failed to write invites.json: %v", err)
		}

		db, err := sql.Open("sqlite", filepath.Join(tmpDir, "migration.db"))
		if err != nil {
			t.Fatalf("failed to open sqlite: %v", err)
		}
		defer db.Close()
		if err := applySchema(db); err != nil {
			t.Fatalf("failed to apply schema: %v", err)
		}

		if err := migrateJSONInvitesToSQLite(db, storage); err != nil {
			t.Fatalf("invite migration failed: %v", err)
		}
		if err := migrateJSONInvitesToSQLite(db, storage); err != nil {
			t.Fatalf("repeated invite migration failed: %v", err)
		}

		var count, maxUses, useCount int
		var createdBy, usedBy string
		var usedAt sql.NullTime
		if err := db.QueryRow(`
			SELECT COUNT(*), created_by, used_by, used_at, max_uses, use_count
			FROM invites
			WHERE code = 'INV-OLD-1'
		`).Scan(&count, &createdBy, &usedBy, &usedAt, &maxUses, &useCount); err != nil {
			t.Fatalf("failed to read migrated invite: %v", err)
		}
		if count != 1 || createdBy != "legacy-admin" || usedBy != "" || usedAt.Valid || maxUses != 2 || useCount != 1 {
			t.Fatalf("unexpected migrated invite: count=%d created_by=%q used_by=%q used_at=%v max_uses=%d use_count=%d",
				count, createdBy, usedBy, usedAt, maxUses, useCount)
		}
	})

	t.Run("rolls back the whole import on insert failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage := NewJSONStorage(tmpDir)
		if err := os.WriteFile(filepath.Join(tmpDir, "invites.json"), []byte(`[
			{"code":"INV-GOOD","created_at":"2026-01-02T03:04:05Z","max_uses":1},
			{"code":"INV-BAD","created_at":"2026-01-02T03:04:05Z","max_uses":1}
		]`), 0600); err != nil {
			t.Fatalf("failed to write invites.json: %v", err)
		}

		db, err := sql.Open("sqlite", filepath.Join(tmpDir, "atomic.db"))
		if err != nil {
			t.Fatalf("failed to open sqlite: %v", err)
		}
		defer db.Close()
		if _, err := db.Exec(`
			CREATE TABLE invites (
				code TEXT PRIMARY KEY CHECK (code <> 'INV-BAD'),
				created_at DATETIME NOT NULL,
				created_by TEXT NOT NULL DEFAULT '',
				used_by TEXT NOT NULL DEFAULT '',
				used_at DATETIME,
				max_uses INTEGER NOT NULL DEFAULT 0,
				use_count INTEGER NOT NULL DEFAULT 0
			)
		`); err != nil {
			t.Fatalf("failed to create constrained schema: %v", err)
		}

		if err := migrateJSONInvitesToSQLite(db, storage); err == nil {
			t.Fatal("expected invite migration to fail")
		}
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM invites`).Scan(&count); err != nil {
			t.Fatalf("failed to count invites: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected atomic rollback, found %d rows", count)
		}
	})
}
