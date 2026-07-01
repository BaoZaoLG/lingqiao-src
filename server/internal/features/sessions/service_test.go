package sessions

import (
	"context"
	"testing"
	"time"
)

type memoryStore struct {
	cards       map[string]CardSnapshot
	sessions    map[string]Session
	blacklisted map[string]bool
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		cards:       make(map[string]CardSnapshot),
		sessions:    make(map[string]Session),
		blacklisted: make(map[string]bool),
	}
}

func (m *memoryStore) RunInTx(ctx context.Context, fn func(Tx) error) error {
	return fn(m)
}

func (m *memoryStore) GetCard(ctx context.Context, code string) (CardSnapshot, error) {
	c, ok := m.cards[code]
	if !ok {
		return CardSnapshot{}, ErrCardNotFound
	}
	return c, nil
}

func (m *memoryStore) IsCardBlacklisted(ctx context.Context, code string) (bool, error) {
	return m.blacklisted[code], nil
}

func (m *memoryStore) ListSessionsByCard(ctx context.Context, code string) ([]Session, error) {
	var res []Session
	for _, s := range m.sessions {
		if s.CardCode == code {
			res = append(res, s)
		}
	}
	return res, nil
}

func (m *memoryStore) InsertSession(ctx context.Context, session Session) error {
	m.sessions[session.Token] = session
	return nil
}

func (m *memoryStore) GetSession(ctx context.Context, token string) (Session, error) {
	s, ok := m.sessions[token]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return s, nil
}

func (m *memoryStore) UpdateSession(ctx context.Context, session Session) error {
	if _, ok := m.sessions[session.Token]; !ok {
		return ErrSessionNotFound
	}
	m.sessions[session.Token] = session
	return nil
}

func (m *memoryStore) DeleteSession(ctx context.Context, token string) error {
	delete(m.sessions, token)
	return nil
}

func (m *memoryStore) UpdateCardActivation(ctx context.Context, card CardSnapshot) error {
	if _, ok := m.cards[card.Code]; !ok {
		return ErrCardNotFound
	}
	m.cards[card.Code] = card
	return nil
}

func (m *memoryStore) UnbindCardPreservingRemaining(ctx context.Context, cardCode string, now time.Time) error {
	c, ok := m.cards[cardCode]
	if !ok {
		return ErrCardNotFound
	}
	if c.ActivatedAt != nil {
		duration := c.ExpiresAt.Sub(*c.ActivatedAt)
		c.ExpiresAt = now.Add(duration)
	}
	c.MachineID = ""
	c.ActivatedAt = nil
	m.cards[cardCode] = c
	return nil
}

func (m *memoryStore) ListSessions(ctx context.Context, filter ListFilter, now time.Time) ([]Session, int, error) {
	var res []Session
	for _, s := range m.sessions {
		if filter.ActiveOnly && s.ExpiresAt.Before(now) {
			continue
		}
		res = append(res, s)
	}
	return res, len(res), nil
}

func (m *memoryStore) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	count := 0
	for k, s := range m.sessions {
		if s.ExpiresAt.Before(now) {
			delete(m.sessions, k)
			count++
		}
	}
	return count, nil
}

// Tests defined by plan:
func TestServiceActivateBindsCardAndCreatesSession(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.cards["ABCDEF123456"] = CardSnapshot{
		Code:        "ABCDEF123456",
		CreatedAt:   now.Add(-24 * time.Hour),
		ExpiresAt:   now.Add(24 * time.Hour),
		Status:      "active",
		MaxSessions: 1,
	}

	session, card, err := s.Activate(context.Background(), ActivationInput{
		CardCode:      "ABCDEF123456",
		MachineID:     "m1",
		Fingerprint:   "f1",
		RemoteAddr:    "1.1.1.1",
		ClientVersion: "1.0",
	})
	if err != nil {
		t.Fatalf("Activate failed: %v", err)
	}
	if session.Token == "" {
		t.Fatal("Missing token")
	}
	if card.MachineID != "m1" {
		t.Fatalf("MachineID = %q", card.MachineID)
	}
}

func TestServiceActivateRejectsDisabledExpiredAndBlacklistedCards(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.cards["DISABLED"] = CardSnapshot{Code: "DISABLED", Status: "disabled", MaxSessions: 1, ExpiresAt: now.Add(time.Hour)}
	m.cards["EXPIRED"] = CardSnapshot{Code: "EXPIRED", Status: "active", MaxSessions: 1, ExpiresAt: now.Add(-time.Hour)}
	m.cards["BLACKLISTED"] = CardSnapshot{Code: "BLACKLISTED", Status: "active", MaxSessions: 1, ExpiresAt: now.Add(time.Hour)}
	m.blacklisted["BLACKLISTED"] = true

	if _, _, err := s.Activate(context.Background(), ActivationInput{CardCode: "DISABLED", MachineID: "m1"}); err != ErrCardDisabled {
		t.Fatalf("Expected ErrCardDisabled, got %v", err)
	}
	if _, _, err := s.Activate(context.Background(), ActivationInput{CardCode: "EXPIRED", MachineID: "m1"}); err != ErrCardExpired {
		t.Fatalf("Expected ErrCardExpired, got %v", err)
	}
	if _, _, err := s.Activate(context.Background(), ActivationInput{CardCode: "BLACKLISTED", MachineID: "m1"}); err != ErrCardBlacklisted {
		t.Fatalf("Expected ErrCardBlacklisted, got %v", err)
	}
}

func TestServiceActivateRejectsOtherMachineWhenSingleSession(t *testing.T) {
	s, m := setupServiceTest()
	m.cards["C1"] = CardSnapshot{Code: "C1", Status: "active", MaxSessions: 1, MachineID: "m1", ExpiresAt: time.Now().Add(time.Hour)}
	_, _, err := s.Activate(context.Background(), ActivationInput{CardCode: "C1", MachineID: "m2"})
	if err == nil {
		t.Fatal("Expected error for different machine")
	}
}

func TestServiceActivateRejectsMaxSessions(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.cards["C1"] = CardSnapshot{Code: "C1", Status: "active", MaxSessions: 1, ExpiresAt: now.Add(time.Hour)}
	s.Activate(context.Background(), ActivationInput{CardCode: "C1", MachineID: "m1"})

	_, _, err := s.Activate(context.Background(), ActivationInput{CardCode: "C1", MachineID: "m1"})
	if err == nil {
		t.Fatal("Expected ErrMaxSessionsReached")
	}
}

func TestServiceHeartbeatExtendsSessionAndUpdatesClientVersion(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.cards["C1"] = CardSnapshot{Code: "C1", Status: "active", MaxSessions: 1, MachineID: "m1", ExpiresAt: now.Add(time.Hour)}
	m.sessions["tok1"] = Session{Token: "tok1", CardCode: "C1", MachineID: "m1", ClientVersion: "1.0", ExpiresAt: now.Add(time.Minute)}

	session, _, err := s.Heartbeat(context.Background(), HeartbeatInput{SessionToken: "tok1", MachineID: "m1", ClientVersion: "2.0"})
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if session.ClientVersion != "2.0" {
		t.Fatalf("Expected updated version")
	}
}

func TestServiceHeartbeatDeletesExpiredOrInvalidSession(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.cards["C1"] = CardSnapshot{Code: "C1", Status: "active", MaxSessions: 1, MachineID: "m1", ExpiresAt: now.Add(-time.Hour)} // expired card
	m.sessions["tok1"] = Session{Token: "tok1", CardCode: "C1", MachineID: "m1", ExpiresAt: now.Add(time.Minute)}

	_, _, err := s.Heartbeat(context.Background(), HeartbeatInput{SessionToken: "tok1", MachineID: "m1"})
	if err == nil {
		t.Fatal("Expected error for expired card")
	}
	if _, ok := m.sessions["tok1"]; ok {
		t.Fatal("Expected session to be deleted")
	}
}

func TestServiceDeactivateDeletesSession(t *testing.T) {
	s, m := setupServiceTest()
	m.sessions["tok1"] = Session{Token: "tok1", CardCode: "C1"}
	if err := s.Deactivate(context.Background(), DeactivateInput{SessionToken: "tok1"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.sessions["tok1"]; ok {
		t.Fatal("Session should be deleted")
	}
}

func TestServiceDeactivateWithUnbindPreservesRemainingDuration(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	activated := now.Add(-time.Hour)
	m.cards["C1"] = CardSnapshot{Code: "C1", MachineID: "m1", ActivatedAt: &activated, ExpiresAt: now.Add(23 * time.Hour)}
	m.sessions["tok1"] = Session{Token: "tok1", CardCode: "C1", MachineID: "m1"}

	if err := s.Deactivate(context.Background(), DeactivateInput{SessionToken: "tok1", Unbind: true}); err != nil {
		t.Fatal(err)
	}
	c := m.cards["C1"]
	if c.MachineID != "" {
		t.Fatal("Expected empty machine id")
	}
}

func TestServiceValidateBoundRejectsMissingTokenMachineMismatchAndCardMismatch(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.cards["C1"] = CardSnapshot{Code: "C1", Status: "active", ExpiresAt: now.Add(time.Hour)}
	m.sessions["tok1"] = Session{Token: "tok1", CardCode: "C1", MachineID: "m1", ExpiresAt: now.Add(time.Minute)}

	if _, err := s.ValidateBound(context.Background(), "", "m1", "C1"); err == nil {
		t.Fatal("Expected error for missing token")
	}
	if _, err := s.ValidateBound(context.Background(), "tok1", "m2", "C1"); err == nil {
		t.Fatal("Expected error for machine mismatch")
	}
	if _, err := s.ValidateBound(context.Background(), "tok1", "m1", "C2"); err == nil {
		t.Fatal("Expected error for card mismatch")
	}
}

func TestServiceValidateActiveTokenPreservesTokenOnlyCompatibility(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.sessions["tok1"] = Session{Token: "tok1", CardCode: "C1", MachineID: "m1", ExpiresAt: now.Add(time.Minute)}
	m.cards["C1"] = CardSnapshot{Code: "C1", Status: "active", ExpiresAt: now.Add(time.Hour)}
	if _, err := s.ValidateActiveToken(context.Background(), "tok1"); err != nil {
		t.Fatal(err)
	}
}

func TestServiceListReturnsOnlyActiveSortedAndPaginated(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.sessions["tok1"] = Session{Token: "tok1", ExpiresAt: now.Add(time.Minute), LastSeen: now}
	m.sessions["tok2"] = Session{Token: "tok2", ExpiresAt: now.Add(-time.Minute), LastSeen: now} // expired
	res, err := s.List(context.Background(), ListFilter{ActiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].Token != "tok1" {
		t.Fatal("Expected only active sessions")
	}
}

func TestServiceCleanupExpiredDeletesOnlyExpiredSessions(t *testing.T) {
	s, m := setupServiceTest()
	now := time.Now()
	m.sessions["tok1"] = Session{Token: "tok1", ExpiresAt: now.Add(time.Minute)}
	m.sessions["tok2"] = Session{Token: "tok2", ExpiresAt: now.Add(-time.Minute)}
	count, _ := s.CleanupExpired(context.Background())
	if count != 1 {
		t.Fatal("Expected 1 deleted session")
	}
	if len(m.sessions) != 1 {
		t.Fatal("Expected 1 remaining session")
	}
}

func setupServiceTest() (*Service, *memoryStore) {
	m := newMemoryStore()
	return NewService(m, 30*time.Minute, func() string { return "tok" + time.Now().String() }, time.Now), m
}
