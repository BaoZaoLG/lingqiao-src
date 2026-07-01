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

func TestCardManagerGenerateCard(t *testing.T) {
	cm := newTestCardManager(t)

	card, err := cm.GenerateCard(24*time.Hour, "test card", 1, "")
	if err != nil {
		t.Fatalf("GenerateCard returned error: %v", err)
	}
	if card.Code == "" {
		t.Fatal("GenerateCard returned empty code")
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
