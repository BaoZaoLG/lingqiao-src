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
