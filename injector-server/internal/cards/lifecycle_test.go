package cards

import (
	"testing"
	"time"
)

func TestLifecycleServiceCreateBuildsActiveCard(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	service := NewLifecycleService(
		func() (string, error) { return "ABCDEF-GHJKLM-NPQRST", nil },
		func() time.Time { return now },
	)

	card, err := service.Create(24*time.Hour, "note", 2, "agent-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if card.Code != "ABCDEF-GHJKLM-NPQRST" {
		t.Fatalf("Code = %q, want generated code", card.Code)
	}
	if card.Status != StatusActive {
		t.Fatalf("Status = %q, want %q", card.Status, StatusActive)
	}
	if card.CreatedAt != now {
		t.Fatalf("CreatedAt = %s, want %s", card.CreatedAt, now)
	}
	if card.ExpiresAt != now.Add(24*time.Hour) {
		t.Fatalf("ExpiresAt = %s, want %s", card.ExpiresAt, now.Add(24*time.Hour))
	}
	if card.AgentID != "agent-1" {
		t.Fatalf("AgentID = %q, want agent-1", card.AgentID)
	}
}

func TestLifecycleServiceUpdateStatus(t *testing.T) {
	service := NewLifecycleService(nil, time.Now)
	card := Card{Code: "ABC", Status: StatusActive}

	updated := service.UpdateStatus(card, StatusDisabled)
	if updated.Status != StatusDisabled {
		t.Fatalf("Status = %q, want %q", updated.Status, StatusDisabled)
	}
}

func TestLifecycleServiceExtendReactivatesExpiredCard(t *testing.T) {
	service := NewLifecycleService(nil, time.Now)
	expires := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	card := Card{Code: "ABC", Status: StatusExpired, ExpiresAt: expires}

	updated := service.Extend(card, 2*time.Hour)
	if updated.Status != StatusActive {
		t.Fatalf("Status = %q, want %q", updated.Status, StatusActive)
	}
	if updated.ExpiresAt != expires.Add(2*time.Hour) {
		t.Fatalf("ExpiresAt = %s, want %s", updated.ExpiresAt, expires.Add(2*time.Hour))
	}
}

func TestLifecycleServiceUpdateDetailsClampsMaxSessions(t *testing.T) {
	service := NewLifecycleService(nil, time.Now)
	card := Card{Code: "ABC", Note: "old", MaxSessions: 2}
	note := "new"
	maxSessions := 0

	updated := service.UpdateDetails(card, &note, &maxSessions)
	if updated.Note != "new" {
		t.Fatalf("Note = %q, want new", updated.Note)
	}
	if updated.MaxSessions != 1 {
		t.Fatalf("MaxSessions = %d, want 1", updated.MaxSessions)
	}
}
