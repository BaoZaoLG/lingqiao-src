package agents

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type mockRepository struct {
	agents map[string]Account
}

func newMockRepository() *mockRepository {
	return &mockRepository{agents: map[string]Account{}}
}

func (m *mockRepository) CreateAgent(ctx context.Context, username, passwordHash, prefix string) (Account, error) {
	return m.CreateAgentTx(ctx, nil, username, passwordHash, prefix)
}

func (m *mockRepository) CreateAgentTx(ctx context.Context, tx *sql.Tx, username, passwordHash, prefix string) (Account, error) {
	for _, a := range m.agents {
		if a.Username == username {
			return Account{}, ErrUsernameExists
		}
	}
	id := "AGT-mock123"
	acc := Account{
		ID:        id,
		Username:  username,
		Password:  passwordHash,
		Prefix:    prefix,
		CreatedAt: time.Now(),
		Disabled:  false,
	}
	m.agents[id] = acc
	return acc, nil
}

func (m *mockRepository) GetAgent(ctx context.Context, id string) (*Account, error) {
	a, ok := m.agents[id]
	if !ok {
		return nil, nil
	}
	return &a, nil
}

func (m *mockRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	for _, a := range m.agents {
		if a.Username == username {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockRepository) UpdateAgentPassword(ctx context.Context, id, passwordHash string) error {
	a, ok := m.agents[id]
	if !ok {
		return ErrAgentNotFound
	}
	a.Password = passwordHash
	m.agents[id] = a
	return nil
}

func (m *mockRepository) AllAgents(ctx context.Context) ([]Account, error) {
	var res []Account
	for _, a := range m.agents {
		res = append(res, a)
	}
	return res, nil
}

func (m *mockRepository) UpdateAgentStatus(ctx context.Context, id string, disabled bool) error {
	a, ok := m.agents[id]
	if !ok {
		return ErrAgentNotFound
	}
	a.Disabled = disabled
	m.agents[id] = a
	return nil
}

func (m *mockRepository) DeleteAgent(ctx context.Context, id string) error {
	_, ok := m.agents[id]
	if !ok {
		return ErrAgentNotFound
	}
	delete(m.agents, id)
	return nil
}

func (m *mockRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return nil, nil
}

func TestService_CreateAgent_Validation(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, time.Now)

	// Test password too long
	longPassword := string(make([]byte, 73))
	_, err := svc.CreateAgent(context.Background(), "user1", longPassword, "")
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("expected ErrPasswordTooLong, got %v", err)
	}

	// Test successful creation
	acc, err := svc.CreateAgent(context.Background(), "user1", "securepass", "PRE")
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	if acc.Username != "user1" || acc.Prefix != "PRE" {
		t.Errorf("unexpected agent values: %+v", acc)
	}

	// Test username collision
	_, err = svc.CreateAgent(context.Background(), "user1", "anotherpass", "")
	if !errors.Is(err, ErrUsernameExists) {
		t.Errorf("expected ErrUsernameExists, got %v", err)
	}
}

func TestService_ChangePassword_Validation(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo, time.Now)

	acc, _ := svc.CreateAgent(context.Background(), "user1", "oldpass", "")

	// Wrong old password
	err := svc.ChangePassword(context.Background(), acc.ID, "wrongold", "newpass")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}

	// Password too long
	longPassword := string(make([]byte, 73))
	err = svc.ChangePassword(context.Background(), acc.ID, "oldpass", longPassword)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("expected ErrPasswordTooLong, got %v", err)
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	schema := `
	CREATE TABLE agents (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		password TEXT NOT NULL,
		prefix TEXT,
		created_at DATETIME NOT NULL,
		disabled INTEGER NOT NULL DEFAULT 0
	);
	CREATE UNIQUE INDEX idx_agents_username ON agents(username);

	CREATE TABLE invites (
		code TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		created_by TEXT,
		used_by TEXT,
		used_at DATETIME,
		max_uses INTEGER,
		use_count INTEGER
	);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	return db
}

func TestService_Integration_RegisterAgent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo, time.Now)
	inviteSvc := NewInviteService(db)

	ctx := context.Background()

	// 1. Create a mock invite code
	invite, err := inviteSvc.Create(2, "admin")
	if err != nil {
		t.Fatalf("failed to create invite code: %v", err)
	}

	// 2. Register agent 1 using invite (success)
	agent1, err := svc.RegisterAgent(ctx, "agent1", "password123", "AG1", invite.Code, inviteSvc)
	if err != nil {
		t.Fatalf("failed to register agent 1: %v", err)
	}
	if agent1.Username != "agent1" || agent1.Prefix != "AG1" {
		t.Errorf("unexpected registered agent values: %+v", agent1)
	}

	// Verify invite use count
	invites, _ := inviteSvc.List()
	if len(invites) != 1 || invites[0].UseCount != 1 {
		t.Errorf("unexpected invite state: %+v", invites)
	}

	// 3. Register agent 2 with the same username (should fail and rollback)
	_, err = svc.RegisterAgent(ctx, "agent1", "password456", "AG2", invite.Code, inviteSvc)
	if !errors.Is(err, ErrUsernameExists) {
		t.Errorf("expected ErrUsernameExists, got %v", err)
	}

	// Verify invite use count was NOT incremented on rollback (still 1)
	invites, _ = inviteSvc.List()
	if invites[0].UseCount != 1 {
		t.Errorf("expected invite use count to be 1 after rollback, got %d", invites[0].UseCount)
	}

	// 4. Register agent 2 with different username (success, use count reaches 2)
	agent2, err := svc.RegisterAgent(ctx, "agent2", "password456", "AG2", invite.Code, inviteSvc)
	if err != nil {
		t.Fatalf("failed to register agent 2: %v", err)
	}
	if agent2.Username != "agent2" {
		t.Errorf("expected agent2, got %s", agent2.Username)
	}

	invites, _ = inviteSvc.List()
	if invites[0].UseCount != 2 {
		t.Errorf("expected invite use count to be 2, got %d", invites[0].UseCount)
	}

	// 5. Try registering agent 3 with the same code (should fail due to exhausted invite)
	_, err = svc.RegisterAgent(ctx, "agent3", "password789", "AG3", invite.Code, inviteSvc)
	if err == nil || err.Error() != "邀请码已用完" {
		t.Errorf("expected '邀请码已用完' error, got %v", err)
	}
}

func TestService_Integration_AdminOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo, time.Now)
	inviteSvc := NewInviteService(db)

	ctx := context.Background()
	invite, _ := inviteSvc.Create(1, "admin")

	// Create agent
	agent, err := svc.RegisterAgent(ctx, "agent_test", "initial_pass", "TST", invite.Code, inviteSvc)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}

	// 1. Verify can login with initial password
	_, _, err = svc.Login(ctx, "agent_test", "initial_pass")
	if err != nil {
		t.Fatalf("failed to login initially: %v", err)
	}

	// 2. Force Reset Password
	err = svc.ForceResetPassword(ctx, agent.ID, "new_admin_pass")
	if err != nil {
		t.Fatalf("failed to force reset password: %v", err)
	}
	// Verify can login with new password
	_, _, err = svc.Login(ctx, "agent_test", "new_admin_pass")
	if err != nil {
		t.Fatalf("failed to login with admin reset password: %v", err)
	}

	// 3. Disable agent
	err = svc.SetAgentStatus(ctx, agent.ID, true)
	if err != nil {
		t.Fatalf("failed to disable agent: %v", err)
	}
	dbAgent, _ := repo.GetAgent(ctx, agent.ID)
	if !dbAgent.Disabled {
		t.Error("expected agent to be disabled")
	}

	// Verify cannot login when disabled
	_, _, err = svc.Login(ctx, "agent_test", "new_admin_pass")
	if !errors.Is(err, ErrAgentDisabled) {
		t.Errorf("expected ErrAgentDisabled, got %v", err)
	}

	// Re-enable agent
	err = svc.SetAgentStatus(ctx, agent.ID, false)
	if err != nil {
		t.Fatalf("failed to re-enable agent: %v", err)
	}

	// 4. Delete agent
	err = svc.DeleteAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to delete agent: %v", err)
	}
	dbAgent, _ = repo.GetAgent(ctx, agent.ID)
	if dbAgent != nil {
		t.Error("expected agent to be deleted")
	}

	// Try operation on non-existent agent (should return ErrAgentNotFound)
	err = svc.SetAgentStatus(ctx, "non_existent", true)
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("expected ErrAgentNotFound for status update on non-existent, got %v", err)
	}

	err = svc.DeleteAgent(ctx, "non_existent")
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("expected ErrAgentNotFound for deletion on non-existent, got %v", err)
	}
}
