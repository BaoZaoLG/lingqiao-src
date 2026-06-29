package cards

import (
	"context"
	"errors"
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

func (r *memoryRepo) Save(ctx context.Context, card Card) error {
	r.cards[card.Code] = card
	return nil
}

func (r *memoryRepo) List(ctx context.Context, filter ListFilter) ([]Card, error) {
	result := make([]Card, 0, len(r.cards))
	for _, card := range r.cards {
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
