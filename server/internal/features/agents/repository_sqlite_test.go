package agents

import (
	"context"
	"errors"
	"testing"
)

func TestSQLiteRepository_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	// 创建 agent
	agent, err := repo.CreateAgent(ctx, "test_user", "hashed_pass", "TST")
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// 重复 username
	_, err = repo.CreateAgent(ctx, "test_user", "another_pass", "TS2")
	if !errors.Is(err, ErrUsernameExists) {
		t.Errorf("expected ErrUsernameExists, got %v", err)
	}

	// 查询不存在 agent
	notFoundAgent, err := repo.GetAgent(ctx, "non_existent")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if notFoundAgent != nil {
		t.Errorf("expected nil for non-existent agent, got %+v", notFoundAgent)
	}

	// 查询已存在 agent
	foundAgent, err := repo.GetAgent(ctx, agent.ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if foundAgent == nil || foundAgent.Username != "test_user" {
		t.Errorf("failed to get correct agent: %+v", foundAgent)
	}
}

func TestSQLiteRepository_UpdateAndDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	agent, _ := repo.CreateAgent(ctx, "upd_user", "pass1", "UP")

	// 修改密码
	err := repo.UpdateAgentPassword(ctx, agent.ID, "pass2")
	if err != nil {
		t.Errorf("failed to update password: %v", err)
	}
	updatedAgent, _ := repo.GetAgent(ctx, agent.ID)
	if updatedAgent.Password != "pass2" {
		t.Errorf("expected password pass2, got %s", updatedAgent.Password)
	}

	// 禁用/启用
	err = repo.UpdateAgentStatus(ctx, agent.ID, true)
	if err != nil {
		t.Errorf("failed to disable agent: %v", err)
	}
	disabledAgent, _ := repo.GetAgent(ctx, agent.ID)
	if !disabledAgent.Disabled {
		t.Errorf("expected agent to be disabled")
	}

	err = repo.UpdateAgentStatus(ctx, agent.ID, false)
	if err != nil {
		t.Errorf("failed to enable agent: %v", err)
	}
	enabledAgent, _ := repo.GetAgent(ctx, agent.ID)
	if enabledAgent.Disabled {
		t.Errorf("expected agent to be enabled")
	}

	// 删除 agent
	err = repo.DeleteAgent(ctx, agent.ID)
	if err != nil {
		t.Errorf("failed to delete agent: %v", err)
	}
	deletedAgent, _ := repo.GetAgent(ctx, agent.ID)
	if deletedAgent != nil {
		t.Errorf("expected agent to be deleted")
	}
}

func TestSQLiteRepository_ShortIDCollisionRetry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	origGen := generateShortID
	defer func() { generateShortID = origGen }()

	calls := 0
	generateShortID = func() string {
		calls++
		if calls <= 3 {
			return "colliding-short-id"
		}
		return "unique-short-id"
	}

	agent1, err := repo.CreateAgent(ctx, "agent1", "pass", "A1")
	if err != nil {
		t.Fatalf("failed to create agent 1: %v", err)
	}
	if agent1.ID != "AGT-colliding-short-id" {
		t.Errorf("unexpected agent 1 ID: %s", agent1.ID)
	}

	calls = 0
	agent2, err := repo.CreateAgent(ctx, "agent2", "pass", "A2")
	if err != nil {
		t.Fatalf("failed to create agent 2 with collision retry: %v", err)
	}
	if agent2.ID != "AGT-unique-short-id" {
		t.Errorf("unexpected agent 2 ID: %s", agent2.ID)
	}
	if calls != 4 {
		t.Errorf("expected 4 calls (3 collisions + 1 success), got %d", calls)
	}
}
