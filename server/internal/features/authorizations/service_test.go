package authorizations

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/sessions"
)

type memoryCardRepo struct {
	items map[string]cards.Card
}

func newMemoryCardRepo() *memoryCardRepo {
	return &memoryCardRepo{items: map[string]cards.Card{}}
}

func (r *memoryCardRepo) Get(ctx context.Context, code string) (cards.Card, error) {
	item, ok := r.items[code]
	if !ok {
		return cards.Card{}, cards.ErrNotFound
	}
	return item, nil
}

func (r *memoryCardRepo) Insert(ctx context.Context, card cards.Card) error {
	r.items[card.Code] = card
	return nil
}

func (r *memoryCardRepo) Save(ctx context.Context, card cards.Card) error {
	r.items[card.Code] = card
	return nil
}

func (r *memoryCardRepo) BatchSave(ctx context.Context, list []cards.Card) error {
	for _, item := range list {
		r.items[item.Code] = item
	}
	return nil
}

func (r *memoryCardRepo) List(ctx context.Context, filter cards.ListFilter) ([]cards.Card, error) {
	var out []cards.Card
	for _, item := range r.items {
		if filter.Query != "" && !strings.Contains(item.Code, filter.Query) && !strings.Contains(item.Note, filter.Query) {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.MachineID != "" && item.MachineID != filter.MachineID {
			continue
		}
		if filter.AgentID != "" && item.AgentID != filter.AgentID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *memoryCardRepo) UpdateStatusBulk(ctx context.Context, codes []string, status cards.Status) (int, error) {
	return 0, nil
}

func (r *memoryCardRepo) ExtendBulk(ctx context.Context, codes []string, extendHours int) (int, error) {
	return 0, nil
}

func (r *memoryCardRepo) Delete(ctx context.Context, code string) error {
	delete(r.items, code)
	return nil
}

func (r *memoryCardRepo) Count(ctx context.Context, filter cards.ListFilter) (int, error) {
	return len(r.items), nil
}

type memorySessionLister struct {
	items []sessions.Session
	err   error
}

func (l memorySessionLister) ListSessions(ctx context.Context, filter sessions.ListFilter, now time.Time) ([]sessions.Session, int, error) {
	if l.err != nil {
		return nil, 0, l.err
	}
	var out []sessions.Session
	for _, item := range l.items {
		if filter.CardCode != "" && item.CardCode != filter.CardCode {
			continue
		}
		if filter.MachineID != "" && item.MachineID != filter.MachineID {
			continue
		}
		if filter.ActiveOnly && !item.ExpiresAt.After(now) {
			continue
		}
		out = append(out, item)
	}
	return out, len(out), nil
}

func TestServiceListComputesLifecycleStatus(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	repo := newMemoryCardRepo()
	activatedAt := now.Add(-2 * time.Hour)
	repo.Save(context.Background(), cards.Card{Code: "ISSUED", Status: cards.StatusActive, CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour), MaxSessions: 1})
	repo.Save(context.Background(), cards.Card{Code: "BOUND", Status: cards.StatusActive, MachineID: "M1", ActivatedAt: &activatedAt, CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour), MaxSessions: 1})
	repo.Save(context.Background(), cards.Card{Code: "INUSE", Status: cards.StatusActive, MachineID: "M2", ActivatedAt: &activatedAt, CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour), MaxSessions: 1})
	repo.Save(context.Background(), cards.Card{Code: "EXPIRED", Status: cards.StatusActive, CreatedAt: now.Add(-48 * time.Hour), ExpiresAt: now.Add(-time.Minute), MaxSessions: 1})
	repo.Save(context.Background(), cards.Card{Code: "FROZEN", Status: cards.StatusDisabled, CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour), MaxSessions: 1})

	service := NewService(repo, memorySessionLister{items: []sessions.Session{
		{Token: "tok", CardCode: "INUSE", MachineID: "M2", LastSeen: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)},
	}}, func() time.Time { return now })

	result, err := service.List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	statusByCode := map[string]EffectiveStatus{}
	for _, item := range result {
		statusByCode[item.Code] = item.EffectiveStatus
	}
	if statusByCode["ISSUED"] != StatusIssued {
		t.Fatalf("ISSUED status = %q", statusByCode["ISSUED"])
	}
	if statusByCode["BOUND"] != StatusActivated {
		t.Fatalf("BOUND status = %q", statusByCode["BOUND"])
	}
	if statusByCode["INUSE"] != StatusInUse {
		t.Fatalf("INUSE status = %q", statusByCode["INUSE"])
	}
	if statusByCode["EXPIRED"] != StatusExpired {
		t.Fatalf("EXPIRED status = %q", statusByCode["EXPIRED"])
	}
	if statusByCode["FROZEN"] != StatusFrozen {
		t.Fatalf("FROZEN status = %q", statusByCode["FROZEN"])
	}
}

func TestServiceDetailIncludesSessions(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	repo := newMemoryCardRepo()
	repo.Save(context.Background(), cards.Card{Code: "DETAIL", Status: cards.StatusActive, MachineID: "M1", CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour), MaxSessions: 2})
	service := NewService(repo, memorySessionLister{items: []sessions.Session{
		{Token: "tok-1", CardCode: "DETAIL", MachineID: "M1", LastSeen: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)},
		{Token: "tok-2", CardCode: "OTHER", MachineID: "M2", LastSeen: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)},
	}}, func() time.Time { return now })

	detail, err := service.Detail(context.Background(), "DETAIL")
	if err != nil {
		t.Fatalf("Detail returned error: %v", err)
	}
	if detail.Code != "DETAIL" {
		t.Fatalf("Code = %q", detail.Code)
	}
	if detail.ActiveSessions != 1 {
		t.Fatalf("ActiveSessions = %d", detail.ActiveSessions)
	}
	if len(detail.Sessions) != 1 || detail.Sessions[0].Token != "tok-1" {
		t.Fatalf("Sessions = %#v", detail.Sessions)
	}
}

func TestServiceDetailReturnsNotFound(t *testing.T) {
	service := NewService(newMemoryCardRepo(), memorySessionLister{}, time.Now)
	_, err := service.Detail(context.Background(), "missing")
	if !errors.Is(err, cards.ErrNotFound) {
		t.Fatalf("expected cards.ErrNotFound, got %v", err)
	}
}

func TestServiceListAppliesAuthorizationFilters(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	repo := newMemoryCardRepo()
	repo.Save(context.Background(), cards.Card{Code: "MATCH", Status: cards.StatusActive, MachineID: "M1", CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(2 * time.Hour), MaxSessions: 2})
	repo.Save(context.Background(), cards.Card{Code: "UNBOUND", Status: cards.StatusActive, CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(2 * time.Hour), MaxSessions: 2})
	repo.Save(context.Background(), cards.Card{Code: "WRONG-SESSIONS", Status: cards.StatusActive, MachineID: "M2", CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(2 * time.Hour), MaxSessions: 1})
	repo.Save(context.Background(), cards.Card{Code: "TOO-LATE", Status: cards.StatusActive, MachineID: "M3", CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(48 * time.Hour), MaxSessions: 2})

	service := NewService(repo, memorySessionLister{}, func() time.Time { return now })
	result, err := service.List(context.Background(), ListFilter{
		Bound:         "true",
		ExpiresBefore: now.Add(24 * time.Hour),
		MaxSessions:   2,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 1 || result[0].Code != "MATCH" {
		t.Fatalf("expected only MATCH, got %#v", result)
	}
}

func TestServicePreviewAndArchiveAuthorization(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	repo := newMemoryCardRepo()
	repo.Save(context.Background(), cards.Card{Code: "ARCHIVE-ME", Status: cards.StatusActive, MachineID: "M1", CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(24 * time.Hour), MaxSessions: 1})
	service := NewService(repo, memorySessionLister{items: []sessions.Session{
		{Token: "tok-1", CardCode: "ARCHIVE-ME", MachineID: "M1", LastSeen: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)},
	}}, func() time.Time { return now })

	preview, err := service.PreviewAction(context.Background(), []string{"ARCHIVE-ME"}, ActionArchiveDelete, 0)
	if err != nil {
		t.Fatalf("PreviewAction returned error: %v", err)
	}
	if preview.TargetCount != 1 || preview.ActiveSessions != 1 || preview.Reversible != true {
		t.Fatalf("unexpected preview: %#v", preview)
	}

	result, err := service.ExecuteAction(context.Background(), []string{"ARCHIVE-ME"}, ActionArchiveDelete, 0)
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if result.Updated != 1 {
		t.Fatalf("Updated = %d", result.Updated)
	}
	archived, err := repo.Get(context.Background(), "ARCHIVE-ME")
	if err != nil {
		t.Fatalf("Get archived returned error: %v", err)
	}
	if archived.Status != cards.StatusArchived {
		t.Fatalf("status = %q", archived.Status)
	}
	list, err := service.List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("archived authorization should be hidden by default, got %#v", list)
	}
	list, err = service.List(context.Background(), ListFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("List include archived returned error: %v", err)
	}
	if len(list) != 1 || list[0].EffectiveStatus != StatusArchived {
		t.Fatalf("archived authorization not visible with include flag: %#v", list)
	}
}
