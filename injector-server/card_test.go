package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	auditsvc "github.com/lingqiao/server/internal/audit"
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

func TestGenerateCardWithCodeNormalizesAndRejectsDuplicates(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, err := cm.GenerateCardWithCode("abcdef-ghjkmn-pqrstv", 24*time.Hour, "imported", 2, "agent-1")
	if err != nil {
		t.Fatalf("GenerateCardWithCode returned error: %v", err)
	}
	if card.Code != "ABCDEF-GHJKMN-PQRSTV" {
		t.Fatalf("Code = %q, want normalized display code", card.Code)
	}
	if card.Note != "imported" {
		t.Fatalf("Note = %q, want imported", card.Note)
	}
	if card.MaxSessions != 2 {
		t.Fatalf("MaxSessions = %d, want 2", card.MaxSessions)
	}

	if _, err := cm.GenerateCardWithCode("ABCDEFGHJKMNPQRSTV", 24*time.Hour, "dup", 1, ""); err == nil {
		t.Fatal("GenerateCardWithCode should reject duplicate normalized card code")
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

func TestActivateCardAllowsMultipleMachinesUpToMaxSessions(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "multi-machine", 6, "")
	for i := 1; i <= 6; i++ {
		machineID := fmt.Sprintf("machine-%d", i)
		session, err := cm.ActivateCard(card.Code, machineID, "fp", "127.0.0.1", "2.0.0")
		if err != nil {
			t.Fatalf("activation %d for %s returned error: %v", i, machineID, err)
		}
		if session.MachineID != machineID {
			t.Fatalf("session MachineID = %q, want %q", session.MachineID, machineID)
		}
	}

	if _, err := cm.ActivateCard(card.Code, "machine-7", "fp", "127.0.0.1", "2.0.0"); err == nil {
		t.Fatal("seventh machine should exceed max sessions")
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

func TestBulkUpdateCardsDetailedReportsPerItemResults(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "bulk-test", 1, "")

	result, err := cm.BulkUpdateCardsDetailed([]string{card.Code, "MISSING-CARD-CODE"}, "disable", 0)
	if err != nil {
		t.Fatalf("BulkUpdateCardsDetailed returned error: %v", err)
	}
	if result.Updated != 1 {
		t.Fatalf("Updated = %d, want 1", result.Updated)
	}
	if result.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", result.Skipped)
	}
	if len(result.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(result.Items))
	}

	updated := cm.GetCard(card.Code)
	if updated.Status != CardDisabled {
		t.Fatalf("Status = %q, want %q", updated.Status, CardDisabled)
	}
}

func TestBulkUpdateCardsDetailedRejectsInvalidAction(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "bulk-test", 1, "")

	result, err := cm.BulkUpdateCardsDetailed([]string{card.Code}, "unknown", 0)
	if err == nil {
		t.Fatal("BulkUpdateCardsDetailed should reject invalid action")
	}
	if result.Failed != 1 {
		t.Fatalf("Failed = %d, want 1", result.Failed)
	}
}

func TestBulkUpdateCardsDetailedMirrorsAuditToRecorder(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	recorder := auditsvc.NewRecorder()
	cm.SetAuditRecorder(recorder)

	card, _ := cm.GenerateCard(24*time.Hour, "bulk-audit", 1, "")
	if _, err := cm.BulkUpdateCardsDetailed([]string{card.Code}, "disable", 0); err != nil {
		t.Fatalf("BulkUpdateCardsDetailed returned error: %v", err)
	}

	events := recorder.Query(auditsvc.Filter{Action: "bulk_disable"})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Detail == "" {
		t.Fatal("mirrored audit event should include detail")
	}
}

func TestGenerateCardMirrorsAuditToRecorder(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	recorder := auditsvc.NewRecorder()
	cm.SetAuditRecorder(recorder)

	card, err := cm.GenerateCard(24*time.Hour, "audit-card", 1, "agent-1")
	if err != nil {
		t.Fatalf("GenerateCard returned error: %v", err)
	}

	events := recorder.Query(auditsvc.Filter{Action: "card_generated", Card: card.Code})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].AgentID != "agent-1" {
		t.Fatalf("AgentID = %q, want agent-1", events[0].AgentID)
	}
}

func TestActivateCardMirrorsAuditToRecorder(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	recorder := auditsvc.NewRecorder()
	cm.SetAuditRecorder(recorder)

	card, _ := cm.GenerateCard(24*time.Hour, "audit-activate", 1, "")
	if _, err := cm.ActivateCard(card.Code, "machine-audit", "fp", "127.0.0.1", "2.0.0"); err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	events := recorder.Query(auditsvc.Filter{Action: "card_activated", Machine: "machine-audit"})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].IP != "127.0.0.1" {
		t.Fatalf("IP = %q, want 127.0.0.1", events[0].IP)
	}
}

func TestCreateAgentMirrorsAuditToRecorder(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	recorder := auditsvc.NewRecorder()
	cm.SetAuditRecorder(recorder)

	agent, err := cm.CreateAgent("agent_audit", "hash", "")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	events := recorder.Query(auditsvc.Filter{Action: "agent_created"})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if !strings.Contains(events[0].Detail, agent.ID) {
		t.Fatalf("Detail = %q, want agent id %q", events[0].Detail, agent.ID)
	}
}

func TestCreateAgentRejectsEmptyUsername(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	_, err := cm.CreateAgent("", "hash", "")
	if err == nil {
		t.Fatal("CreateAgent should reject empty username")
	}
}

func TestAddBlacklistMirrorsAuditToRecorder(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	recorder := auditsvc.NewRecorder()
	cm.SetAuditRecorder(recorder)

	cm.AddBlacklist("machine", "machine-audit", "reason")

	events := recorder.Query(auditsvc.Filter{Action: "blacklist_added"})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if !strings.Contains(events[0].Detail, "machine-audit") {
		t.Fatalf("Detail = %q, want machine id", events[0].Detail)
	}
}

func TestUnbindCardClearsMachineAndMirrorsAudit(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	recorder := auditsvc.NewRecorder()
	cm.SetAuditRecorder(recorder)

	card, _ := cm.GenerateCard(24*time.Hour, "unbind", 1, "")
	if _, err := cm.ActivateCard(card.Code, "machine-unbind", "fp", "127.0.0.1", "2.0.0"); err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	if err := cm.UnbindCard(card.Code); err != nil {
		t.Fatalf("UnbindCard returned error: %v", err)
	}

	updated := cm.GetCard(card.Code)
	if updated.MachineID != "" {
		t.Fatalf("MachineID = %q, want empty", updated.MachineID)
	}
	if updated.ActivatedAt != nil {
		t.Fatalf("ActivatedAt = %v, want nil", updated.ActivatedAt)
	}
	events := recorder.Query(auditsvc.Filter{Action: "card_unbound"})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
}
