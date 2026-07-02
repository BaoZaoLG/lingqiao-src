package audit

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// mockRepo is used to test the Recorder's error propagation
type mockRepo struct {
	err error
}

func (m *mockRepo) Insert(ctx context.Context, event Event) error {
	return m.err
}

func (m *mockRepo) List(ctx context.Context, limit int) ([]Event, error) {
	return nil, nil
}

func TestRecorder_Append(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &mockRepo{err: nil}
		recorder := NewRecorder(repo)

		err := recorder.Append(Event{Action: "test"})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("failure propagates error", func(t *testing.T) {
		expectedErr := errors.New("db write failed")
		repo := &mockRepo{err: expectedErr}
		recorder := NewRecorder(repo)

		err := recorder.Append(Event{Action: "test"})
		if err == nil {
			t.Errorf("expected error, got nil")
		} else if err != expectedErr {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})
}

func TestSQLiteRepository_Insert(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	// Setup schema
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		time     DATETIME NOT NULL,
		action   TEXT NOT NULL,
		card     TEXT NOT NULL DEFAULT '',
		machine  TEXT NOT NULL DEFAULT '',
		agent_id TEXT NOT NULL DEFAULT '',
		detail   TEXT NOT NULL DEFAULT '',
		addr     TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	repo := NewSQLiteRepository(db)

	now := time.Now()
	event := Event{
		Time:    now,
		Action:  "login",
		Card:    "CARD-1",
		Machine: "MACH-1",
		AgentID: "AGENT-1",
		Detail:  "success",
		IP:      "127.0.0.1",
	}

	if err := repo.Insert(context.Background(), event); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Verify insertion
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&count); err != nil {
		t.Fatalf("QueryRow failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}

	// Verify data
	var action, card, ip string
	if err := db.QueryRow(`SELECT action, card, addr FROM audit_log`).Scan(&action, &card, &ip); err != nil {
		t.Fatalf("QueryRow failed: %v", err)
	}
	if action != "login" || card != "CARD-1" || ip != "127.0.0.1" {
		t.Errorf("data mismatch, got action=%s card=%s ip=%s", action, card, ip)
	}

	// Test failure on closed DB
	db.Close()
	err = repo.Insert(context.Background(), event)
	if err == nil {
		t.Errorf("expected error on closed db, got nil")
	}
}
