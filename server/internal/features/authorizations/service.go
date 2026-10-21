package authorizations

import (
	"context"
	"sort"
	"time"

	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/sessions"
)

type Clock func() time.Time

type SessionLister interface {
	ListSessions(ctx context.Context, filter sessions.ListFilter, now time.Time) ([]sessions.Session, int, error)
}

type Service struct {
	cards    cards.Repository
	sessions SessionLister
	now      Clock
}

func NewService(cardsRepo cards.Repository, sessionLister SessionLister, now Clock) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{cards: cardsRepo, sessions: sessionLister, now: now}
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]AuthorizationSummary, error) {
	cardFilter := cards.ListFilter{
		Query:     filter.Query,
		AgentID:   filter.AgentID,
		MachineID: filter.MachineID,
	}
	list, err := s.cards.List(ctx, cardFilter)
	if err != nil {
		return nil, err
	}

	now := s.now()
	out := make([]AuthorizationSummary, 0, len(list))
	for _, card := range list {
		sessionsList, _, err := s.sessions.ListSessions(ctx, sessions.ListFilter{CardCode: card.Code, ActiveOnly: true}, now)
		if err != nil {
			return nil, err
		}
		summary := summarize(card, sessionsList, now)
		if summary.EffectiveStatus == StatusArchived && !filter.IncludeArchived {
			continue
		}
		if !matchesFilter(card, summary, filter) {
			continue
		}
		if filter.EffectiveStatus != "" && summary.EffectiveStatus != filter.EffectiveStatus {
			continue
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *Service) Detail(ctx context.Context, code string) (AuthorizationDetail, error) {
	card, err := s.cards.Get(ctx, code)
	if err != nil {
		return AuthorizationDetail{}, err
	}
	now := s.now()
	sessionsList, _, err := s.sessions.ListSessions(ctx, sessions.ListFilter{CardCode: card.Code, ActiveOnly: true}, now)
	if err != nil {
		return AuthorizationDetail{}, err
	}
	return AuthorizationDetail{
		AuthorizationSummary: summarize(card, sessionsList, now),
		Sessions:             sessionsList,
	}, nil
}

func (s *Service) PreviewAction(ctx context.Context, codes []string, action Action, extendHours int) (ImpactPreview, error) {
	preview := ImpactPreview{
		Action:     action,
		Reversible: action != ActionArchiveDelete,
	}
	if action == ActionArchiveDelete {
		preview.Reversible = true
		preview.Warnings = append(preview.Warnings, "授权将从默认列表隐藏，客户端不可继续使用，但历史记录会保留")
	}
	now := s.now()
	machineSet := map[string]struct{}{}
	for _, code := range codes {
		card, err := s.cards.Get(ctx, code)
		if err != nil {
			return preview, err
		}
		preview.TargetCount++
		if card.MachineID != "" {
			machineSet[card.MachineID] = struct{}{}
		}
		sessionsList, _, err := s.sessions.ListSessions(ctx, sessions.ListFilter{CardCode: card.Code, ActiveOnly: true}, now)
		if err != nil {
			return preview, err
		}
		preview.ActiveSessions += len(sessionsList)
	}
	for machine := range machineSet {
		preview.Machines = append(preview.Machines, machine)
	}
	sort.Strings(preview.Machines)
	return preview, nil
}

func (s *Service) ExecuteAction(ctx context.Context, codes []string, action Action, extendHours int) (ActionResult, error) {
	var result ActionResult
	for _, code := range codes {
		card, err := s.cards.Get(ctx, code)
		if err != nil {
			result.Failed++
			continue
		}
		changed := false
		switch action {
		case ActionExtend:
			if extendHours <= 0 {
				extendHours = 720
			}
			card.ExpiresAt = card.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)
			if card.Status == cards.StatusExpired {
				card.Status = cards.StatusActive
			}
			changed = true
		case ActionUnbindMachine:
			if card.MachineID != "" || card.ActivatedAt != nil {
				if card.ActivatedAt != nil {
					originalDuration := card.ExpiresAt.Sub(*card.ActivatedAt)
					card.ExpiresAt = card.CreatedAt.Add(originalDuration)
				}
				card.MachineID = ""
				card.ActivatedAt = nil
				changed = true
			}
		case ActionFreeze:
			if card.Status != cards.StatusDisabled {
				card.Status = cards.StatusDisabled
				changed = true
			}
		case ActionRestore:
			if card.Status == cards.StatusDisabled || card.Status == cards.StatusArchived {
				card.Status = cards.StatusActive
				changed = true
			}
		case ActionArchiveDelete:
			if card.Status != cards.StatusArchived {
				card.Status = cards.StatusArchived
				card.MachineID = ""
				card.ActivatedAt = nil
				changed = true
			}
		default:
			result.Failed++
			continue
		}
		if !changed {
			result.Skipped++
			continue
		}
		if err := s.cards.Save(ctx, card); err != nil {
			result.Failed++
			continue
		}
		result.Updated++
		result.Codes = append(result.Codes, code)
	}
	return result, nil
}

func summarize(card cards.Card, activeSessions []sessions.Session, now time.Time) AuthorizationSummary {
	var lastSeen *time.Time
	for _, sess := range activeSessions {
		if lastSeen == nil || sess.LastSeen.After(*lastSeen) {
			t := sess.LastSeen
			lastSeen = &t
		}
	}
	return AuthorizationSummary{
		Code:            card.Code,
		AgentID:         card.AgentID,
		MachineID:       card.MachineID,
		CreatedAt:       card.CreatedAt,
		ActivatedAt:     card.ActivatedAt,
		ExpiresAt:       card.ExpiresAt,
		EffectiveStatus: effectiveStatus(card, activeSessions, now),
		StoredStatus:    string(card.Status),
		Note:            card.Note,
		MaxSessions:     card.MaxSessions,
		ActiveSessions:  len(activeSessions),
		LastSeen:        lastSeen,
	}
}

func effectiveStatus(card cards.Card, activeSessions []sessions.Session, now time.Time) EffectiveStatus {
	if card.Status == cards.StatusArchived {
		return StatusArchived
	}
	if card.Status == cards.StatusDisabled {
		return StatusFrozen
	}
	if card.Status == cards.StatusExpired || (!card.ExpiresAt.IsZero() && !now.Before(card.ExpiresAt)) {
		return StatusExpired
	}
	if len(activeSessions) > 0 {
		return StatusInUse
	}
	if card.MachineID != "" || card.ActivatedAt != nil {
		return StatusActivated
	}
	return StatusIssued
}

func matchesFilter(card cards.Card, summary AuthorizationSummary, filter ListFilter) bool {
	switch filter.Bound {
	case "true", "1":
		if card.MachineID == "" {
			return false
		}
	case "false", "0":
		if card.MachineID != "" {
			return false
		}
	}
	if !filter.ExpiresAfter.IsZero() && card.ExpiresAt.Before(filter.ExpiresAfter) {
		return false
	}
	if !filter.ExpiresBefore.IsZero() && card.ExpiresAt.After(filter.ExpiresBefore) {
		return false
	}
	if filter.MaxSessions > 0 && card.MaxSessions != filter.MaxSessions {
		return false
	}
	return summary.Code != ""
}
