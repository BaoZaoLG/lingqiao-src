package cards

import (
	"context"
	"errors"
	"time"

	cardops "github.com/lingqiao/server/internal/cards"
)

var (
	ErrInvalidDuration = errors.New("duration must be positive")
	ErrInvalidSessions = errors.New("max sessions must be at least 1")
)

type CodeGenerator func() (string, error)
type Clock func() time.Time

type Service struct {
	repo         Repository
	generateCode CodeGenerator
	now          Clock
}

func NewService(repo Repository, generateCode CodeGenerator, now Clock) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, generateCode: generateCode, now: now}
}

func (s *Service) Create(ctx context.Context, duration time.Duration, note string, maxSessions int, agentID string) (Card, error) {
	if duration <= 0 {
		return Card{}, ErrInvalidDuration
	}
	if maxSessions < 1 {
		return Card{}, ErrInvalidSessions
	}
	code, err := s.generateCode()
	if err != nil {
		return Card{}, err
	}
	now := s.now()
	card := Card{
		Code:        code,
		AgentID:     agentID,
		CreatedAt:   now,
		ExpiresAt:   now.Add(duration),
		Status:      StatusActive,
		Note:        note,
		MaxSessions: maxSessions,
	}
	if err := s.repo.Save(ctx, card); err != nil {
		return Card{}, err
	}
	return card, nil
}

func (s *Service) CreateWithCode(ctx context.Context, code string, duration time.Duration, note string, maxSessions int, agentID string) (Card, error) {
	if duration <= 0 {
		return Card{}, ErrInvalidDuration
	}
	if maxSessions < 1 {
		return Card{}, ErrInvalidSessions
	}
	now := s.now()
	card := Card{
		Code:        code,
		AgentID:     agentID,
		CreatedAt:   now,
		ExpiresAt:   now.Add(duration),
		Status:      StatusActive,
		Note:        note,
		MaxSessions: maxSessions,
	}
	if err := s.repo.Insert(ctx, card); err != nil {
		return Card{}, err
	}
	return card, nil
}

func (s *Service) BatchCreate(ctx context.Context, count int, duration time.Duration, note string, maxSessions int, agentID string) ([]Card, error) {
	if count <= 0 {
		return nil, errors.New("count must be positive")
	}
	if duration <= 0 {
		return nil, ErrInvalidDuration
	}
	if maxSessions < 1 {
		return nil, ErrInvalidSessions
	}

	var cards []Card
	now := s.now()
	for i := 0; i < count; i++ {
		code, err := s.generateCode()
		if err != nil {
			return nil, err
		}
		cards = append(cards, Card{
			Code:        code,
			AgentID:     agentID,
			CreatedAt:   now,
			ExpiresAt:   now.Add(duration),
			Status:      StatusActive,
			Note:        note,
			MaxSessions: maxSessions,
		})
	}

	if err := s.repo.BatchSave(ctx, cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func (s *Service) UpdateStatus(ctx context.Context, code string, status Status) error {
	card, err := s.repo.Get(ctx, code)
	if err != nil {
		return err
	}
	card.Status = status
	return s.repo.Save(ctx, card)
}

func (s *Service) Extend(ctx context.Context, code string, extendHours int) error {
	if extendHours <= 0 {
		return errors.New("extendHours must be positive")
	}
	card, err := s.repo.Get(ctx, code)
	if err != nil {
		return err
	}
	card.ExpiresAt = card.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)
	if card.Status == StatusExpired {
		card.Status = StatusActive
	}
	return s.repo.Save(ctx, card)
}

func (s *Service) Unbind(ctx context.Context, code string) error {
	card, err := s.repo.Get(ctx, code)
	if err != nil {
		return err
	}
	if card.ActivatedAt != nil {
		originalDuration := card.ExpiresAt.Sub(*card.ActivatedAt)
		card.ExpiresAt = card.CreatedAt.Add(originalDuration)
	}
	card.MachineID = ""
	card.ActivatedAt = nil
	return s.repo.Save(ctx, card)
}

func (s *Service) UpdateStatusBulk(ctx context.Context, codes []string, status Status) (int, error) {
	return s.repo.UpdateStatusBulk(ctx, codes, status)
}

func (s *Service) ExtendBulk(ctx context.Context, codes []string, extendHours int) (int, error) {
	if extendHours <= 0 {
		return 0, errors.New("extendHours must be positive")
	}
	return s.repo.ExtendBulk(ctx, codes, extendHours)
}

func (s *Service) UpdateDetails(ctx context.Context, code string, note *string, maxSessions *int) error {
	card, err := s.repo.Get(ctx, code)
	if err != nil {
		return err
	}

	changed := false
	if note != nil {
		card.Note = *note
		changed = true
	}
	if maxSessions != nil {
		if *maxSessions < 1 {
			return ErrInvalidSessions
		}
		card.MaxSessions = *maxSessions
		changed = true
	}

	if !changed {
		return nil
	}

	return s.repo.Save(ctx, card)
}

func (s *Service) BulkUpdateCardsDetailed(ctx context.Context, codes []string, action string, extendHours int, dryRun bool) (cardops.BulkResult, error) {
	var result cardops.BulkResult
	bulkAction := cardops.BulkAction(action)
	if err := cardops.ValidateBulkAction(bulkAction); err != nil {
		for _, code := range codes {
			result.AddItem(cardops.BulkItemResult{
				Code:    code,
				Status:  cardops.BulkItemFailed,
				Message: err.Error(),
			})
		}
		return result, err
	}

	for _, code := range codes {
		card, err := s.repo.Get(ctx, code)
		if err != nil {
			result.AddItem(cardops.BulkItemResult{
				Code:    code,
				Status:  cardops.BulkItemSkipped,
				Message: "card not found",
			})
			continue
		}

		changed := false
		switch bulkAction {
		case cardops.BulkDisable, cardops.BulkEnable, cardops.BulkExpire:
			status := Status(action)
			if action == "expire" {
				status = StatusExpired
			}
			if card.Status != status {
				card.Status = status
				changed = true
			}
		case cardops.BulkExtend:
			if extendHours <= 0 {
				extendHours = 720
			}
			card.ExpiresAt = card.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)
			if card.Status == StatusExpired {
				card.Status = StatusActive
			}
			changed = true
		case cardops.BulkUnbind:
			if card.MachineID != "" {
				if card.ActivatedAt != nil {
					originalDuration := card.ExpiresAt.Sub(*card.ActivatedAt)
					card.ExpiresAt = card.CreatedAt.Add(originalDuration)
				}
				card.MachineID = ""
				card.ActivatedAt = nil
				changed = true
			}
		}

		if !changed {
			result.AddItem(cardops.BulkItemResult{
				Code:   code,
				Status: cardops.BulkItemSkipped,
			})
			continue
		}

		if !dryRun {
			if err := s.repo.Save(ctx, card); err != nil {
				result.AddItem(cardops.BulkItemResult{
					Code:    code,
					Status:  cardops.BulkItemFailed,
					Message: err.Error(),
				})
				continue
			}
		}

		result.AddItem(cardops.BulkItemResult{
			Code:   code,
			Status: cardops.BulkItemUpdated,
		})
	}

	return result, nil
}

func (s *Service) AllCards(ctx context.Context) ([]Card, error) {
	return s.repo.List(ctx, ListFilter{})
}

func (s *Service) SearchCards(ctx context.Context, query, status, machineID string) ([]Card, error) {
	return s.repo.List(ctx, ListFilter{
		Query:     query,
		Status:    Status(status),
		MachineID: machineID,
	})
}

func (s *Service) GetCard(ctx context.Context, code string) (Card, error) {
	return s.repo.Get(ctx, code)
}

func (s *Service) GetCardsByMachine(ctx context.Context, machineID string) ([]Card, error) {
	return s.repo.List(ctx, ListFilter{
		MachineID: machineID,
	})
}

func (s *Service) AgentCards(ctx context.Context, agentID string) ([]Card, error) {
	return s.repo.List(ctx, ListFilter{AgentID: agentID})
}

func (s *Service) AgentStats(ctx context.Context, agentID string) (total, active, expired int, err error) {
	cardsList, err := s.repo.List(ctx, ListFilter{AgentID: agentID})
	if err != nil {
		return 0, 0, 0, err
	}
	now := s.now()
	for _, c := range cardsList {
		total++
		if c.Status == StatusActive && now.Before(c.ExpiresAt) {
			active++
		}
		if now.After(c.ExpiresAt) || c.Status == StatusExpired {
			expired++
		}
	}
	return total, active, expired, nil
}
