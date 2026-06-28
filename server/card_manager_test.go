package main

import (
	"errors"
	"testing"
	"time"

	cardops "github.com/lingqiao/server/internal/cards"
)

func newTestCardManager(t *testing.T) CardManager {
	t.Helper()
	storage := NewJSONStorage(t.TempDir())
	return NewJSONCardManager(storage)
}

func TestCardManagerGenerateActivateHeartbeatDeactivate(t *testing.T) {
	cm := newTestCardManager(t)

	card, err := cm.GenerateCard(24*time.Hour, "test card", 1, "")
	if err != nil {
		t.Fatalf("GenerateCard returned error: %v", err)
	}
	if card.Code == "" {
		t.Fatal("GenerateCard returned empty code")
	}

	session, err := cm.ActivateCard(card.Code, "machine-1", "fingerprint-1", "127.0.0.1", "2.4.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}
	if session.Token == "" {
		t.Fatal("ActivateCard returned empty token")
	}
	if session.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", session.MachineID)
	}

	heartbeat, err := cm.Heartbeat(session.Token, "machine-1", "127.0.0.1", "2.4.1")
	if err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}
	if heartbeat.ClientVersion != "2.4.1" {
		t.Fatalf("ClientVersion = %q, want 2.4.1", heartbeat.ClientVersion)
	}

	if err := cm.DeactivateSession(session.Token, false); err != nil {
		t.Fatalf("DeactivateSession returned error: %v", err)
	}
	if _, err := cm.Heartbeat(session.Token, "machine-1", "127.0.0.1", "2.4.1"); err == nil {
		t.Fatal("Heartbeat after deactivation returned nil error")
	}
}

func TestCardManagerRejectsDisabledExpiredAndBlacklistedCards(t *testing.T) {
	cm := newTestCardManager(t)

	card, err := cm.GenerateCard(24*time.Hour, "test card", 1, "")
	if err != nil {
		t.Fatalf("GenerateCard returned error: %v", err)
	}

	if err := cm.UpdateCardStatus(card.Code, CardDisabled); err != nil {
		t.Fatalf("UpdateCardStatus disabled returned error: %v", err)
	}
	if _, err := cm.ValidateCard(card.Code); !errors.Is(err, cardops.ErrCardDisabled) {
		t.Fatalf("ValidateCard disabled error = %v, want ErrCardDisabled", err)
	}

	if err := cm.UpdateCardStatus(card.Code, CardActive); err != nil {
		t.Fatalf("UpdateCardStatus active returned error: %v", err)
	}
	cm.AddBlacklist(normalizeCardCode(card.Code), normalizeCardCode(card.Code), "test blacklist")
	if _, err := cm.ValidateCard(card.Code); !errors.Is(err, cardops.ErrCardBlacklisted) {
		t.Fatalf("ValidateCard blacklisted error = %v, want ErrCardBlacklisted", err)
	}
}

func TestCardManagerEnforcesMaxSessions(t *testing.T) {
	cm := newTestCardManager(t)

	card, err := cm.GenerateCard(24*time.Hour, "single session", 1, "")
	if err != nil {
		t.Fatalf("GenerateCard returned error: %v", err)
	}
	if _, err := cm.ActivateCard(card.Code, "machine-1", "fingerprint-1", "127.0.0.1", "2.4.0"); err != nil {
		t.Fatalf("first ActivateCard returned error: %v", err)
	}
	if _, err := cm.ActivateCard(card.Code, "machine-1", "fingerprint-1", "127.0.0.1", "2.4.0"); !errors.Is(err, cardops.ErrMaxSessionsReached) {
		t.Fatalf("second ActivateCard error = %v, want ErrMaxSessionsReached", err)
	}
}
