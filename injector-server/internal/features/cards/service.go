package cards

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidDuration = errors.New("duration must be positive")
	ErrInvalidSessions = errors.New("max sessions must be at least 1")
)

type CodeGenerator func() (string, error)
type Clock func() time.Time

type Service struct {
	repo Repository
	generateCode CodeGenerator
	now Clock
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
