package main

import (
	"testing"
	"time"
)

func configureRuntimeForTest(t *testing.T, cfg RuntimeConfig) func() {
	t.Helper()
	previous := currentRuntimeConfig()
	ConfigureRuntime(cfg)
	return func() {
		ConfigureRuntime(previous)
	}
}

func TestConfiguredSessionTTLAffectsAdminAgentAndCardSessions(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{
		DataDir:    t.TempDir(),
		SessionTTL: 90 * time.Minute,
	})
	defer restore()

	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	admin := NewAdminHandler(cm)
	adminToken := admin.createSession()
	adminExpiry := admin.sessions[hashToken(adminToken)]
	if remaining := time.Until(adminExpiry); remaining < 89*time.Minute || remaining > 91*time.Minute {
		t.Fatalf("admin session ttl = %s, want about 90m", remaining)
	}

	agent := NewAgentHandler(cm)
	agentToken := agent.createSession()
	agentExpiry := agent.sessions[hashToken(agentToken)]
	if remaining := time.Until(agentExpiry); remaining < 89*time.Minute || remaining > 91*time.Minute {
		t.Fatalf("agent session ttl = %s, want about 90m", remaining)
	}

	card, err := cm.GenerateCard(24*time.Hour, "ttl", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	session, err := cm.ActivateCard(card.Code, "machine-ttl", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if remaining := time.Until(session.ExpiresAt); remaining < 89*time.Minute || remaining > 91*time.Minute {
		t.Fatalf("card session ttl = %s, want about 90m", remaining)
	}
}
