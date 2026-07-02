package main

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
	_ "modernc.org/sqlite"
)

func setupTestDBAndServices(t *testing.T) (*sql.DB, *cards.Service, *agents.Service) {
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
		disabled INTEGER NOT NULL DEFAULT 0,
		max_session_duration_minutes INTEGER NOT NULL DEFAULT 0,
		max_daily_sessions INTEGER NOT NULL DEFAULT 0,
		max_concurrent_sessions INTEGER NOT NULL DEFAULT 0
	);
	CREATE UNIQUE INDEX idx_agents_username ON agents(username);

	CREATE TABLE cards (
		code TEXT PRIMARY KEY,
		machine_id TEXT NOT NULL DEFAULT '',
		agent_id TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL,
		activated_at DATETIME,
		expires_at DATETIME NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		note TEXT NOT NULL DEFAULT '',
		max_sessions INTEGER NOT NULL DEFAULT 1
	);

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

	cardsRepo := cards.NewSQLiteRepository(db)
	cardsSvc := cards.NewService(cardsRepo, func() (string, error) { return "TEST-CARD-CODE", nil }, time.Now)

	agentsRepo := agents.NewSQLiteRepository(db)
	agentsSvc := agents.NewService(agentsRepo, time.Now)

	return db, cardsSvc, agentsSvc
}

func TestAgentCardManagerAdapter_StatusChecks(t *testing.T) {
	db, cardsSvc, agentsSvc := setupTestDBAndServices(t)
	defer db.Close()

	inviteSvc := agents.NewInviteService(db)
	invite, err := inviteSvc.Create(1, "admin")
	if err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}

	// Register agent
	agent, err := agentsSvc.RegisterAgent(context.Background(), "test_agent", "password123", "TST", invite.Code, inviteSvc)
	if err != nil {
		t.Fatalf("failed to register agent: %v", err)
	}

	adapter := &agentCardManagerAdapter{
		cardsSvc:  cardsSvc,
		agentsSvc: agentsSvc,
	}

	// 1. Success check
	snapshot, err := adapter.GenerateCard(24*time.Hour, "note", 1, agent.ID)
	if err != nil {
		t.Fatalf("expected successful card generation, got: %v", err)
	}
	if snapshot.Code != "TEST-CARD-CODE" {
		t.Errorf("unexpected card code: %s", snapshot.Code)
	}

	// 2. Disable agent
	err = agentsSvc.SetAgentStatus(context.Background(), agent.ID, true)
	if err != nil {
		t.Fatalf("failed to disable agent: %v", err)
	}

	_, err = adapter.GenerateCard(24*time.Hour, "note", 1, agent.ID)
	if err == nil || !strings.Contains(err.Error(), "agent is disabled") {
		t.Errorf("expected 'agent is disabled' error, got: %v", err)
	}

	_, err = adapter.BatchGenerateCards(3, 24*time.Hour, "note", 1, agent.ID)
	if err == nil || !strings.Contains(err.Error(), "agent is disabled") {
		t.Errorf("expected 'agent is disabled' error, got: %v", err)
	}

	// 3. Deleted/Non-existent agent ID
	_, err = adapter.GenerateCard(24*time.Hour, "note", 1, "AGT-nonexistent")
	if err == nil || !strings.Contains(err.Error(), "agent not found") {
		t.Errorf("expected 'agent not found' error, got: %v", err)
	}

	_, err = adapter.BatchGenerateCards(3, 24*time.Hour, "note", 1, "AGT-nonexistent")
	if err == nil || !strings.Contains(err.Error(), "agent not found") {
		t.Errorf("expected 'agent not found' error, got: %v", err)
	}
}
