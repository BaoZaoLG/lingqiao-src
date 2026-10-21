package blacklist

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setup(t *testing.T) *Service {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE blacklist (
		value TEXT PRIMARY KEY,
		type TEXT NOT NULL DEFAULT '',
		reason TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(NewSQLiteRepository(db))
}

func TestBlacklistServiceCRUD(t *testing.T) {
	svc := setup(t)
	ctx := context.Background()
	if err := svc.Add(ctx, "machine", "m-1", "abuse"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(ctx, "machine", "  ", "x"); err == nil {
		t.Fatal("expected empty value error")
	}
	ok, err := svc.IsBlacklisted(ctx, "m-1")
	if err != nil || !ok {
		t.Fatalf("expected blacklisted: %v %v", ok, err)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %d", err, len(list))
	}
	if err := svc.Remove(ctx, "m-1"); err != nil {
		t.Fatal(err)
	}
	ok, err = svc.IsBlacklisted(ctx, "m-1")
	if err != nil || ok {
		t.Fatalf("expected removed: %v %v", ok, err)
	}
}

func TestBlacklistALL(t *testing.T) {
	svc := setup(t)
	ctx := context.Background()
	if err := svc.Add(ctx, "global", "ALL", "lockdown"); err != nil {
		t.Fatal(err)
	}
	ok, err := svc.IsBlacklisted(ctx, "anything")
	if err != nil || !ok {
		t.Fatalf("ALL should match: %v %v", ok, err)
	}
}
