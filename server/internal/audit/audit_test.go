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

func (m *mockRepo) List(ctx context.Context, filter Filter, limit int) ([]Event, error) {
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

func TestRecorder_QueryDefaultsAndNilRepo(t *testing.T) {
	rec := NewRecorder(nil)
	if err := rec.Append(Event{Action: "x"}); err != nil {
		t.Fatalf("nil repo Append: %v", err)
	}
	events, err := rec.Query(context.Background(), Filter{}, 0)
	if err != nil || events != nil {
		t.Fatalf("nil repo Query: %v %#v", err, events)
	}

	mem := &recordingRepo{}
	rec2 := NewRecorder(mem)
	_ = rec2.Append(Event{Action: "a"})
	got, err := rec2.Query(context.Background(), Filter{Action: "a"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if mem.lastLimit != 100 {
		t.Fatalf("default limit = %d, want 100", mem.lastLimit)
	}
	got, err = rec2.Query(context.Background(), Filter{}, 50000)
	if err != nil {
		t.Fatal(err)
	}
	if mem.lastLimit != 10000 {
		t.Fatalf("cap limit = %d, want 10000", mem.lastLimit)
	}
	_ = got
}

type recordingRepo struct {
	lastLimit int
	events    []Event
}

func (r *recordingRepo) Insert(ctx context.Context, event Event) error {
	r.events = append(r.events, event)
	return nil
}

func (r *recordingRepo) List(ctx context.Context, filter Filter, limit int) ([]Event, error) {
	r.lastLimit = limit
	return r.events, nil
}

func TestSQLiteRepository_ListFilters(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		time DATETIME NOT NULL,
		action TEXT NOT NULL,
		card TEXT NOT NULL DEFAULT '',
		machine TEXT NOT NULL DEFAULT '',
		agent_id TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL DEFAULT '',
		addr TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewSQLiteRepository(db)
	now := time.Now().UTC().Truncate(time.Second)
	for i, action := range []string{"login", "logout", "login"} {
		if err := repo.Insert(context.Background(), Event{
			Time: now.Add(time.Duration(i) * time.Second), Action: action, Card: "C1", Detail: "d-" + action, IP: "10.0.0.1",
		}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := repo.List(context.Background(), Filter{Action: "login", Query: "d-login"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("filtered count = %d, want 2", len(events))
	}
}
