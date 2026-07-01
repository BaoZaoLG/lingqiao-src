package agents

import (
	"context"
	"fmt"
	"strings"
	"testing"
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
