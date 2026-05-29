package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func setupTestCM(t *testing.T) (*CardManager, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "cardtest")
	if err != nil {
		t.Fatal(err)
	}
	s := NewJSONStorage(dir)
	cm := NewCardManager(s)
	return cm, dir
}

func teardownTestCM(dir string) {
	os.RemoveAll(dir)
}

func TestGenerateCard(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, err := cm.GenerateCard(24*time.Hour, "test", 1, "")
	if err != nil {
		t.Fatalf("GenerateCard failed: %v", err)
	}
	if card.Code == "" {
		t.Fatal("card code is empty")
	}
	if card.Status != CardActive {
		t.Fatalf("expected status %q, got %q", CardActive, card.Status)
	}
	if card.MaxSessions != 1 {
		t.Fatalf("expected max_sessions 1, got %d", card.MaxSessions)
	}
	if card.Note != "test" {
		t.Fatalf("expected note 'test', got %q", card.Note)
	}
}

func TestCardCodeFormat(t *testing.T) {
	code, err := generateCardCode()
	if err != nil {
		t.Fatalf("generateCardCode failed: %v", err)
	}
	if len(code) != 20 {
		t.Fatalf("expected code length 20, got %d: %q", len(code), code)
	}
	if code[6] != '-' || code[13] != '-' {
		t.Fatalf("expected dashes at positions 6 and 13, got %q", code)
	}
}

func TestActivateCard(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "test", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine1", "fp1", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard failed: %v", err)
	}
	if session.Token == "" {
		t.Fatal("session token is empty")
	}
	if session.MachineID != "machine1" {
		t.Fatalf("expected machine1, got %q", session.MachineID)
	}
}

func TestActivateCardDeactivateReactivate(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "test", 1, "")
	session, _ := cm.ActivateCard(card.Code, "machine1", "fp1", "127.0.0.1", "2.0.0")

	err := cm.DeactivateSession(session.Token, false)
	if err != nil {
		t.Fatalf("deactivate failed: %v", err)
	}

	session2, err := cm.ActivateCard(card.Code, "machine1", "fp1", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("re-activation after deactivation should succeed: %v", err)
	}
	if session2.Token == "" {
		t.Fatal("new session token is empty")
	}
}

func TestHeartbeat(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "test", 1, "")
	session, _ := cm.ActivateCard(card.Code, "machine1", "fp1", "127.0.0.1", "2.0.0")

	sess, err := cm.Heartbeat(session.Token, "machine1", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("heartbeat should succeed: %v", err)
	}
	if sess == nil {
		t.Fatal("heartbeat returned nil session")
	}

	_, err = cm.Heartbeat("invalid-token", "machine1", "127.0.0.1", "2.0.0")
	if err == nil {
		t.Fatal("heartbeat with invalid token should fail")
	}
}

func TestDeactivateSession(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "test", 1, "")
	session, _ := cm.ActivateCard(card.Code, "machine1", "fp1", "127.0.0.1", "2.0.0")

	err := cm.DeactivateSession(session.Token, false)
	if err != nil {
		t.Fatalf("deactivation should succeed: %v", err)
	}

	_, err = cm.Heartbeat(session.Token, "machine1", "127.0.0.1", "2.0.0")
	if err == nil {
		t.Fatal("heartbeat after deactivation should fail")
	}
}

func TestBlacklistCard(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "test", 1, "")
	// Blacklist using normalized code (no dashes, uppercase) since ValidateCard normalizes
	normalized := strings.ToUpper(strings.ReplaceAll(card.Code, "-", ""))
	cm.AddBlacklist("card", normalized, "test-blacklist")

	_, err := cm.ActivateCard(card.Code, "machine1", "fp1", "127.0.0.1", "2.0.0")
	if err == nil {
		t.Fatal("activation of blacklisted card should fail")
	}
}

func TestCardExpiry(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(1*time.Second, "test", 1, "")
	time.Sleep(2 * time.Second)

	_, err := cm.ActivateCard(card.Code, "machine1", "fp1", "127.0.0.1", "2.0.0")
	if err == nil {
		t.Fatal("activation of expired card should fail")
	}
}

func TestPersistence(t *testing.T) {
	dir, _ := os.MkdirTemp("", "cardtest")
	defer os.RemoveAll(dir)

	s := NewJSONStorage(dir)
	cm1 := NewCardManager(s)
	card, _ := cm1.GenerateCard(24*time.Hour, "persist-test", 1, "")
	code := card.Code

	cm2 := NewCardManager(s)
	cards := cm2.AllCards()
	found := false
	for _, c := range cards {
		if c.Code == code {
			found = true
			if c.Note != "persist-test" {
				t.Fatalf("note mismatch: got %q", c.Note)
			}
		}
	}
	if !found {
		t.Fatal("card not found after reload")
	}
}
