package cards

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type memoryRepo struct {
	cards map[string]Card
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{cards: map[string]Card{}}
}

func (r *memoryRepo) Get(ctx context.Context, code string) (Card, error) {
	card, ok := r.cards[code]
	if !ok {
		return Card{}, errors.New("not found")
	}
	return card, nil
}

func (r *memoryRepo) Insert(ctx context.Context, card Card) error {
	if _, ok := r.cards[card.Code]; ok {
		return errors.New("already exists") // Or whatever DB driver error like UNIQUE constraint failed
	}
	r.cards[card.Code] = card
	return nil
}

func (r *memoryRepo) Save(ctx context.Context, card Card) error {
	r.cards[card.Code] = card
	return nil
}

func (r *memoryRepo) List(ctx context.Context, filter ListFilter) ([]Card, error) {
	result := make([]Card, 0, len(r.cards))
	for _, card := range r.cards {
		if filter.AgentID != "" && card.AgentID != filter.AgentID {
			continue
		}
		if filter.MachineID != "" && card.MachineID != filter.MachineID {
			continue
		}
		if filter.Status != "" && card.Status != filter.Status {
			continue
		}
		if filter.Query != "" {
			if !strings.Contains(card.Code, filter.Query) && !strings.Contains(card.Note, filter.Query) {
				continue
			}
		}
		result = append(result, card)
	}
	return result, nil
}

func (r *memoryRepo) BatchSave(ctx context.Context, cards []Card) error {
	for _, c := range cards {
		r.cards[c.Code] = c
	}
	return nil
}

func (r *memoryRepo) UpdateStatusBulk(ctx context.Context, codes []string, status Status) (int, error) {
	affected := 0
	for _, code := range codes {
		if c, ok := r.cards[code]; ok {
			c.Status = status
			r.cards[code] = c
			affected++
		}
	}
	return affected, nil
}

func (r *memoryRepo) ExtendBulk(ctx context.Context, codes []string, extendHours int) (int, error) {
	affected := 0
	for _, code := range codes {
		if c, ok := r.cards[code]; ok {
			c.ExpiresAt = c.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)
			if c.Status == StatusExpired {
				c.Status = StatusActive
			}
			r.cards[code] = c
			affected++
		}
	}
	return affected, nil
}

func (r *memoryRepo) Delete(ctx context.Context, code string) error {
	delete(r.cards, code)
	return nil
}

func (r *memoryRepo) Count(ctx context.Context, filter ListFilter) (int, error) {
	return len(r.cards), nil
}

func TestServiceCreateCard(t *testing.T) {
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepo()
	service := NewService(repo, func() (string, error) {
		return "ABCD-EFGH-JKLM", nil
	}, func() time.Time {
		return now
	})

	card, err := service.Create(context.Background(), 24*time.Hour, "note", 2, "agent-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if card.Code != "ABCD-EFGH-JKLM" {
		t.Fatalf("Code = %q", card.Code)
	}
	if card.AgentID != "agent-1" {
		t.Fatalf("AgentID = %q", card.AgentID)
	}
	if card.Status != StatusActive {
		t.Fatalf("Status = %q", card.Status)
	}
	if !card.ExpiresAt.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("ExpiresAt = %s", card.ExpiresAt)
	}
	if _, ok := repo.cards[card.Code]; !ok {
		t.Fatal("card was not saved")
	}
}

func TestServiceCreateRejectsInvalidInputs(t *testing.T) {
	service := NewService(newMemoryRepo(), func() (string, error) {
		return "ABCD-EFGH-JKLM", nil
	}, time.Now)

	if _, err := service.Create(context.Background(), 0, "", 1, ""); !errors.Is(err, ErrInvalidDuration) {
		t.Fatalf("duration error = %v", err)
	}
	if _, err := service.Create(context.Background(), time.Hour, "", 0, ""); !errors.Is(err, ErrInvalidSessions) {
		t.Fatalf("session error = %v", err)
	}
}

func TestServiceUnbind(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo, nil, time.Now)

	now := time.Now()
	activated := now.Add(-2 * time.Hour) // Activated 2 hours ago
	expires := now.Add(22 * time.Hour)   // 24 total hours

	card := Card{
		Code:        "BIND-CARD",
		MachineID:   "machine-1",
		CreatedAt:   now.Add(-4 * time.Hour), // created 4 hours ago
		ActivatedAt: &activated,
		ExpiresAt:   expires,
	}
	repo.Save(context.Background(), card)

	err := service.Unbind(context.Background(), "BIND-CARD")
	if err != nil {
		t.Fatalf("Unbind failed: %v", err)
	}

	updated, _ := repo.Get(context.Background(), "BIND-CARD")
	if updated.MachineID != "" {
		t.Errorf("MachineID not cleared: %s", updated.MachineID)
	}
	if updated.ActivatedAt != nil {
		t.Error("ActivatedAt not cleared")
	}
	expectedExpires := card.CreatedAt.Add(expires.Sub(activated))
	if !updated.ExpiresAt.Equal(expectedExpires) {
		t.Errorf("Expected expires %s, got %s", expectedExpires, updated.ExpiresAt)
	}
}

func TestServiceBulkUpdateDetailedDryRun(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo, nil, time.Now)

	now := time.Now()
	repo.Save(context.Background(), Card{Code: "C1", Status: StatusActive, ExpiresAt: now.Add(24 * time.Hour)})
	repo.Save(context.Background(), Card{Code: "C2", Status: StatusExpired, ExpiresAt: now.Add(-time.Hour)})

	// Test dry_run = true
	res, err := service.BulkUpdateCardsDetailed(context.Background(), []string{"C1", "C2", "C3"}, "expire", 0, true)
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}
	if res.Updated != 1 {
		t.Errorf("Expected 1 updated, got %d", res.Updated)
	}
	if res.Skipped != 2 {
		t.Errorf("Expected 2 skipped, got %d", res.Skipped)
	}

	c1, _ := repo.Get(context.Background(), "C1")
	if c1.Status != StatusActive {
		t.Errorf("Dry run altered DB: C1 status is %s", c1.Status)
	}
}

func TestServiceQueryAndStats(t *testing.T) {
	repo := newMemoryRepo()
	now := time.Now()
	service := NewService(repo, nil, func() time.Time { return now })

	// Add cards for agent-1
	repo.Save(context.Background(), Card{Code: "C1", AgentID: "agent-1", Status: StatusActive, ExpiresAt: now.Add(time.Hour)})
	repo.Save(context.Background(), Card{Code: "C2", AgentID: "agent-1", Status: StatusActive, ExpiresAt: now.Add(-time.Hour)}) // expired by time
	repo.Save(context.Background(), Card{Code: "C3", AgentID: "agent-1", Status: StatusExpired, ExpiresAt: now.Add(time.Hour)}) // expired by status

	// Add cards for agent-2
	repo.Save(context.Background(), Card{Code: "C4", AgentID: "agent-2", Status: StatusActive, ExpiresAt: now.Add(time.Hour)})

	// 1. Verify AllCards
	all, err := service.AllCards(context.Background())
	if err != nil || len(all) != 4 {
		t.Errorf("expected 4 cards total, got %d (err: %v)", len(all), err)
	}

	// 2. Verify AgentCards
	a1Cards, err := service.AgentCards(context.Background(), "agent-1")
	if err != nil || len(a1Cards) != 3 {
		t.Errorf("expected 3 cards for agent-1, got %d (err: %v)", len(a1Cards), err)
	}

	// 3. Verify AgentStats
	total, active, expired, err := service.AgentStats(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("AgentStats failed: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if active != 1 { // Only C1 is active & not expired
		t.Errorf("expected active 1, got %d", active)
	}
	if expired != 2 { // C2 (expired by time) & C3 (expired by status)
		t.Errorf("expected expired 2, got %d", expired)
	}

	// 4. Verify SearchCards
	searchRes, _ := service.SearchCards(context.Background(), "C1", "", "")
	if len(searchRes) != 1 || searchRes[0].Code != "C1" {
		t.Errorf("SearchCards by Code failed")
	}

	searchRes2, _ := service.SearchCards(context.Background(), "", "expired", "")
	if len(searchRes2) != 1 || searchRes2[0].Code != "C3" {
		t.Errorf("SearchCards by Status failed")
	}

	// Add machine binding for testing
	repo.Save(context.Background(), Card{Code: "C5", MachineID: "mac123", Status: StatusActive, ExpiresAt: now.Add(time.Hour)})

	// 5. Verify GetCardsByMachine
	macRes, _ := service.GetCardsByMachine(context.Background(), "mac123")
	if len(macRes) != 1 || macRes[0].Code != "C5" {
		t.Errorf("GetCardsByMachine failed")
	}
}
