package cards

import (
	"testing"
	"time"
)

func fixedSessionService(now time.Time) *SessionService {
	return NewSessionService(func() time.Time { return now }, 24*time.Hour)
}

func TestSessionServiceActivateRejectsMachineMismatch(t *testing.T) {
	service := fixedSessionService(time.Now())
	card := Card{Code: "ABC", MachineID: "machine-1", Status: StatusActive, MaxSessions: 1, ExpiresAt: time.Now().Add(time.Hour)}

	_, _, err := service.Activate(ActivationInput{
		Card:      card,
		MachineID: "machine-2",
	})
	if err == nil {
		t.Fatal("Activate should reject card bound to another machine")
	}
}

func TestSessionServiceActivateAllowsMultipleMachinesUpToMaxSessions(t *testing.T) {
	now := time.Date(2026, 6, 11, 22, 30, 0, 0, time.UTC)
	service := fixedSessionService(now)
	card := Card{Code: "ABC", MachineID: "machine-1", Status: StatusActive, MaxSessions: 6, ExpiresAt: now.Add(time.Hour)}

	_, session, err := service.Activate(ActivationInput{
		Card:      card,
		Token:     "token-2",
		MachineID: "machine-2",
		ActiveSessions: []Session{
			{CardCode: "ABC", MachineID: "machine-1", ExpiresAt: now.Add(time.Hour)},
		},
	})
	if err != nil {
		t.Fatalf("Activate should allow another machine below max sessions: %v", err)
	}
	if session.MachineID != "machine-2" {
		t.Fatalf("MachineID = %q, want machine-2", session.MachineID)
	}
}

func TestSessionServiceActivateRejectsMaxActiveSessions(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	service := fixedSessionService(now)
	card := Card{Code: "ABC", Status: StatusActive, MaxSessions: 1, ExpiresAt: now.Add(time.Hour)}

	_, _, err := service.Activate(ActivationInput{
		Card:      card,
		MachineID: "machine-1",
		ActiveSessions: []Session{
			{CardCode: "ABC", ExpiresAt: now.Add(time.Hour)},
		},
	})
	if err == nil {
		t.Fatal("Activate should reject max active sessions")
	}
}

func TestSessionServiceActivateStartsBillingOnFirstActivation(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	created := now.Add(-time.Hour)
	service := fixedSessionService(now)
	card := Card{
		Code:        "ABC",
		CreatedAt:   created,
		ExpiresAt:   created.Add(24 * time.Hour),
		Status:      StatusActive,
		MaxSessions: 1,
	}

	updatedCard, session, err := service.Activate(ActivationInput{
		Card:          card,
		Token:         "token-1",
		MachineID:     "machine-1",
		Fingerprint:   "fp",
		RemoteAddr:    "127.0.0.1",
		ClientVersion: "2.0.0",
	})
	if err != nil {
		t.Fatalf("Activate returned error: %v", err)
	}
	if updatedCard.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", updatedCard.MachineID)
	}
	if updatedCard.ActivatedAt == nil || !updatedCard.ActivatedAt.Equal(now) {
		t.Fatalf("ActivatedAt = %v, want %s", updatedCard.ActivatedAt, now)
	}
	if updatedCard.ExpiresAt != now.Add(24*time.Hour) {
		t.Fatalf("ExpiresAt = %s, want %s", updatedCard.ExpiresAt, now.Add(24*time.Hour))
	}
	if session.Token != "token-1" {
		t.Fatalf("Token = %q, want token-1", session.Token)
	}
	if session.ExpiresAt != now.Add(24*time.Hour) {
		t.Fatalf("Session ExpiresAt = %s, want %s", session.ExpiresAt, now.Add(24*time.Hour))
	}
}

func TestSessionServiceHeartbeatRejectsExpiredSession(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	service := fixedSessionService(now)

	_, err := service.Heartbeat(HeartbeatInput{
		Session:   Session{Token: "token", MachineID: "machine-1", ExpiresAt: now.Add(-time.Second)},
		Card:      Card{Code: "ABC", Status: StatusActive, ExpiresAt: now.Add(time.Hour)},
		MachineID: "machine-1",
	})
	if err == nil {
		t.Fatal("Heartbeat should reject expired session")
	}
}

func TestSessionServiceHeartbeatRejectsInvalidCard(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	service := fixedSessionService(now)

	_, err := service.Heartbeat(HeartbeatInput{
		Session:   Session{Token: "token", MachineID: "machine-1", ExpiresAt: now.Add(time.Hour)},
		Card:      Card{Code: "ABC", Status: StatusDisabled, ExpiresAt: now.Add(time.Hour)},
		MachineID: "machine-1",
	})
	if err == nil {
		t.Fatal("Heartbeat should reject disabled card")
	}
}

func TestSessionServiceHeartbeatRenewsSession(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	service := fixedSessionService(now)

	session, err := service.Heartbeat(HeartbeatInput{
		Session:       Session{Token: "token", MachineID: "machine-1", ExpiresAt: now.Add(time.Hour)},
		Card:          Card{Code: "ABC", Status: StatusActive, ExpiresAt: now.Add(time.Hour)},
		MachineID:     "machine-1",
		RemoteAddr:    "127.0.0.1",
		ClientVersion: "2.1.0",
	})
	if err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}
	if session.LastSeen != now {
		t.Fatalf("LastSeen = %s, want %s", session.LastSeen, now)
	}
	if session.ExpiresAt != now.Add(24*time.Hour) {
		t.Fatalf("ExpiresAt = %s, want %s", session.ExpiresAt, now.Add(24*time.Hour))
	}
	if session.ClientVersion != "2.1.0" {
		t.Fatalf("ClientVersion = %q, want 2.1.0", session.ClientVersion)
	}
}
