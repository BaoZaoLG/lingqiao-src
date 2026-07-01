package sessions

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE cards (
			code TEXT PRIMARY KEY,
			machine_id TEXT,
			agent_id TEXT,
			created_at TEXT,
			activated_at TEXT,
			expires_at TEXT,
			status TEXT,
			note TEXT,
			max_sessions INTEGER
		);
		CREATE TABLE sessions (
			token TEXT PRIMARY KEY,
			card_code TEXT,
			machine_id TEXT,
			fingerprint TEXT,
			client_version TEXT,
			created_at TEXT,
			last_seen TEXT,
			expires_at TEXT,
			remote_addr TEXT
		);
		CREATE TABLE blacklist (value TEXT PRIMARY KEY, type TEXT, reason TEXT, created_at DATETIME);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}
	return db
}

func TestSQLiteRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	// Insert test card
	db.Exec("INSERT INTO cards (code, status, created_at, expires_at, max_sessions) VALUES ('C1', 'active', ?, ?, 1)", nowStr, nowStr)

	// Run in Tx
	err := repo.RunInTx(ctx, func(tx Tx) error {
		// GetCard
		c, err := tx.GetCard(ctx, "C1")
		if err != nil {
			t.Fatalf("GetCard failed: %v", err)
		}
		if c.Code != "C1" {
			t.Fatal("Wrong card")
		}

		// InsertSession
		s := Session{Token: "tok1", CardCode: "C1", MachineID: "m1", CreatedAt: now, LastSeen: now, ExpiresAt: now}
		if err := tx.InsertSession(ctx, s); err != nil {
			t.Fatalf("InsertSession failed: %v", err)
		}

		// UpdateCardActivation
		c.MachineID = "m1"
		activated := now
		c.ActivatedAt = &activated
		if err := tx.UpdateCardActivation(ctx, c); err != nil {
			t.Fatalf("UpdateCardActivation failed: %v", err)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify session was created
	s, err := repo.GetSession(ctx, "tok1")
	if err != nil {
		t.Fatal(err)
	}
	if s.MachineID != "m1" {
		t.Fatal("Wrong session saved")
	}

	// Test Unbind
	err = repo.RunInTx(ctx, func(tx Tx) error {
		return tx.UnbindCardPreservingRemaining(ctx, "C1", now.Add(time.Hour))
	})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.RunInTx(ctx, func(tx Tx) error {
		c, _ := tx.GetCard(ctx, "C1")
		if c.MachineID != "" {
			t.Fatal("Unbind didn't clear machine")
		}
		return nil
	})

	// Test ListSessions
	filter := ListFilter{ActiveOnly: true, Page: 1, PerPage: 10}
	sessions, total, err := repo.ListSessions(ctx, filter, now.Add(-time.Hour)) // it should be active compared to -1h
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(sessions) != 1 {
		t.Fatal("Expected 1 session")
	}
}
