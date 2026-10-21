package agents

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestInviteService_Lifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewInviteService(db)

	// 创建邀请码 (max_uses > 1)
	inv, err := svc.Create(5, "admin")
	if err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}

	// 测试多次使用
	for i := 0; i < 5; i++ {
		err = svc.ValidateAndUse(inv.Code, "user")
		if err != nil {
			t.Errorf("expected success for use %d, got %v", i+1, err)
		}
	}
	err = svc.ValidateAndUse(inv.Code, "user")
	if err == nil || !strings.Contains(err.Error(), "邀请码已用完") {
		t.Errorf("expected '邀请码已用完' error, got %v", err)
	}
}

func TestInviteService_MaxUsesEdgeCases(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	inviteSvc := NewInviteService(db)

	// 1. max_uses = 1
	inv1, err := inviteSvc.Create(1, "admin")
	if err != nil {
		t.Fatalf("failed to create invite 1: %v", err)
	}
	err = inviteSvc.ValidateAndUse(inv1.Code, "user1")
	if err != nil {
		t.Errorf("expected success for first use, got %v", err)
	}
	err = inviteSvc.ValidateAndUse(inv1.Code, "user2")
	if err == nil || !strings.Contains(err.Error(), "邀请码已用完") {
		t.Errorf("expected '邀请码已用完' error, got %v", err)
	}

	// 2. max_uses = 0
	invUnlimited, err := inviteSvc.Create(0, "admin")
	if err != nil {
		t.Fatalf("failed to create unlimited invite: %v", err)
	}
	for i := 0; i < 10; i++ {
		err = inviteSvc.ValidateAndUse(invUnlimited.Code, fmt.Sprintf("user%d", i))
		if err != nil {
			t.Errorf("expected success on call %d for unlimited invite, got %v", i, err)
		}
	}

	// 3. 删除邀请码
	deleted, err := inviteSvc.Delete(invUnlimited.Code)
	if err != nil || !deleted {
		t.Errorf("failed to delete invite: %v (deleted=%v)", err, deleted)
	}
	err = inviteSvc.ValidateAndUse(invUnlimited.Code, "user")
	if err == nil || !strings.Contains(err.Error(), "邀请码无效") {
		t.Errorf("expected '邀请码无效' error for deleted invite, got %v", err)
	}
}

func TestInviteService_TxRollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewInviteService(db)

	inv, _ := svc.Create(2, "admin")

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}

	err = svc.ValidateAndUseTx(context.Background(), tx, inv.Code, "user1")
	if err != nil {
		t.Fatalf("failed to use in tx: %v", err)
	}

	// 事务回滚时 use_count 不增加
	tx.Rollback()

	invites, _ := svc.List()
	if invites[0].UseCount != 0 {
		t.Errorf("expected use count 0 after rollback, got %d", invites[0].UseCount)
	}
}

func TestInviteService_LegacyNullsAndTimeRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Production schema uses NOT NULL defaults for text/int columns; used_at remains nullable.
	// Simulate legacy-like empty attribution with zero counters and null used_at.
	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	if _, err := db.Exec(`
		INSERT INTO invites (code, created_at, created_by, used_by, used_at, max_uses, use_count)
		VALUES (?, ?, '', '', NULL, 0, 0)
	`, "INV-LEGACY-NULL", createdAt); err != nil {
		t.Fatalf("failed to insert legacy invite: %v", err)
	}

	svc := NewInviteService(db)
	invites, err := svc.List()
	if err != nil {
		t.Fatalf("failed to list legacy invite: %v", err)
	}
	if len(invites) != 1 {
		t.Fatalf("expected one invite, got %d", len(invites))
	}
	if !invites[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at changed during scan: got %v want %v", invites[0].CreatedAt, createdAt)
	}
	if invites[0].CreatedBy != "" || invites[0].UsedBy != "" || invites[0].MaxUses != 0 || invites[0].UseCount != 0 {
		t.Fatalf("legacy NULL values were not mapped to zero values: %+v", invites[0])
	}

	if err := svc.ValidateAndUse("INV-LEGACY-NULL", "legacy-user"); err != nil {
		t.Fatalf("legacy invite should be consumable: %v", err)
	}
	invites, err = svc.List()
	if err != nil {
		t.Fatalf("failed to list consumed legacy invite: %v", err)
	}
	if invites[0].UseCount != 1 || invites[0].UsedBy != "legacy-user" || invites[0].UsedAt.IsZero() {
		t.Fatalf("unexpected consumed legacy invite: %+v", invites[0])
	}
	if _, offset := invites[0].UsedAt.Zone(); offset != 0 {
		t.Fatalf("used_at must be stored in UTC, got %v", invites[0].UsedAt)
	}
}

func TestInviteService_CreateUsesUTC(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	invite, err := NewInviteService(db).Create(1, "admin")
	if err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}
	if _, offset := invite.CreatedAt.Zone(); offset != 0 {
		t.Fatalf("created_at must be UTC, got %v", invite.CreatedAt)
	}
}

func TestInviteService_ListUsesStableOrder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	for _, code := range []string{"INV-B", "INV-A"} {
		if _, err := db.Exec(`
			INSERT INTO invites (code, created_at, created_by, used_by, used_at, max_uses, use_count)
			VALUES (?, ?, '', '', NULL, 1, 0)
		`, code, createdAt); err != nil {
			t.Fatalf("failed to insert %s: %v", code, err)
		}
	}

	invites, err := NewInviteService(db).List()
	if err != nil {
		t.Fatalf("failed to list invites: %v", err)
	}
	if len(invites) != 2 || invites[0].Code != "INV-A" || invites[1].Code != "INV-B" {
		t.Fatalf("unexpected stable order: %+v", invites)
	}
}

func TestInviteService_ListHandlesNullableUsedAt(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO invites (code, created_at, created_by, used_by, used_at, max_uses, use_count)
		VALUES ('INV-NULL-TIME', ?, '', '', NULL, 1, 0)
	`, time.Now().UTC()); err != nil {
		t.Fatalf("failed to insert invite: %v", err)
	}

	invites, err := NewInviteService(db).List()
	if err != nil {
		t.Fatalf("failed to list invites: %v", err)
	}
	if len(invites) != 1 || !invites[0].UsedAt.IsZero() {
		t.Fatalf("nullable used_at should map to zero time: %+v", invites)
	}

	var usedAt sql.NullTime
	if err := db.QueryRow(`SELECT used_at FROM invites WHERE code = 'INV-NULL-TIME'`).Scan(&usedAt); err != nil {
		t.Fatalf("failed to scan raw used_at: %v", err)
	}
	if usedAt.Valid {
		t.Fatalf("expected SQL NULL used_at, got %v", usedAt.Time)
	}
}
