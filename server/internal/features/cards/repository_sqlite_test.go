package cards

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

	_, err = db.Exec(`
		CREATE TABLE cards (
			code TEXT PRIMARY KEY,
			agent_id TEXT,
			status TEXT,
			note TEXT,
			max_sessions INTEGER,
			created_at DATETIME,
			activated_at DATETIME,
			expires_at DATETIME,
			machine_id TEXT
		)
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

	// Test ErrNotFound
	_, err := repo.Get(ctx, "NONEXISTENT")
	if err == nil || err.Error() != "card not found" {
		t.Fatalf("Expected 'card not found' error, got %v", err)
	}

	// Test Save and Get
	now := time.Now().Truncate(time.Second) // SQLite drops sub-second precision easily depending on driver
	card := Card{
		Code:        "TEST-CARD-123",
		Status:      StatusActive,
		Note:        "Test note",
		MaxSessions: 2,
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
		MachineID:   "machine-1",
	}
	if err := repo.Save(ctx, card); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	retrieved, err := repo.Get(ctx, "TEST-CARD-123")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Code != card.Code || retrieved.MachineID != card.MachineID || retrieved.Status != card.Status {
		t.Fatalf("Retrieved card does not match: %+v", retrieved)
	}
	var rawExpires string
	if err := db.QueryRow("SELECT expires_at FROM cards WHERE code = ?", card.Code).Scan(&rawExpires); err != nil {
		t.Fatalf("scan raw expires_at: %v", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, rawExpires); err != nil {
		t.Fatalf("expires_at should be RFC3339-compatible, got %q: %v", rawExpires, err)
	}

	// Test List
	filter := ListFilter{Query: "TEST", Status: StatusActive}
	list, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].Code != "TEST-CARD-123" {
		t.Fatalf("List returned unexpected result: %+v", list)
	}

	// Test Count
	count, err := repo.Count(ctx, filter)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected count 1, got %d", count)
	}
}

func TestSQLiteRepositoryParsesLegacyGoTimeString(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSQLiteRepository(db)

	rawCreated := "2026-07-02 16:06:28.338015275 +0800 CST m=+3972.908943307"
	rawExpires := "2026-07-02 17:06:28.338015275 +0800 CST m=+3972.908943307"
	if _, err := db.Exec(
		"INSERT INTO cards (code, status, created_at, expires_at, max_sessions) VALUES (?, ?, ?, ?, ?)",
		"LEGACY-TIME", StatusActive, rawCreated, rawExpires, 1,
	); err != nil {
		t.Fatalf("insert legacy time card: %v", err)
	}

	card, err := repo.Get(context.Background(), "LEGACY-TIME")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if card.ExpiresAt.IsZero() || card.ExpiresAt.Year() != 2026 || card.ExpiresAt.Hour() != 17 {
		t.Fatalf("legacy expires_at was not parsed: %#v", card.ExpiresAt)
	}
}

func TestSQLiteRepository_Filters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	cards := []Card{
		{Code: "C1", Status: StatusActive, MachineID: "m1", AgentID: "a1", CreatedAt: now},
		{Code: "C2", Status: StatusDisabled, MachineID: "m2", AgentID: "a1", CreatedAt: now},
		{Code: "C3", Status: StatusActive, MachineID: "m1", AgentID: "a2", CreatedAt: now},
	}
	if err := repo.BatchSave(ctx, cards); err != nil {
		t.Fatalf("BatchSave failed: %v", err)
	}

	tests := []struct {
		name     string
		filter   ListFilter
		expected int
	}{
		{"filter by status", ListFilter{Status: StatusActive}, 2},
		{"filter by machine", ListFilter{MachineID: "m1"}, 2},
		{"filter by agent", ListFilter{AgentID: "a1"}, 2},
		{"filter by all", ListFilter{Status: StatusActive, MachineID: "m1", AgentID: "a1"}, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			list, err := repo.List(ctx, tc.filter)
			if err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if len(list) != tc.expected {
				t.Errorf("Expected %d cards, got %d", tc.expected, len(list))
			}
		})
	}
}

func TestSQLiteRepository_BatchSave(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	cards := []Card{
		{Code: "BATCH1", Status: StatusActive, MaxSessions: 1, CreatedAt: now},
		{Code: "BATCH2", Status: StatusActive, MaxSessions: 2, CreatedAt: now},
	}
	if err := repo.BatchSave(ctx, cards); err != nil {
		t.Fatalf("BatchSave failed: %v", err)
	}

	c1, _ := repo.Get(ctx, "BATCH1")
	if c1.MaxSessions != 1 {
		t.Errorf("Expected 1, got %d", c1.MaxSessions)
	}

	// Update existing via BatchSave
	cards[0].MaxSessions = 5
	if err := repo.BatchSave(ctx, cards); err != nil {
		t.Fatalf("BatchSave update failed: %v", err)
	}

	c1Updated, _ := repo.Get(ctx, "BATCH1")
	if c1Updated.MaxSessions != 5 {
		t.Errorf("Expected max sessions updated to 5, got %d", c1Updated.MaxSessions)
	}
}
