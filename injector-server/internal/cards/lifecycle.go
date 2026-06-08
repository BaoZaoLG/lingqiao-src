package cards

import "time"

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusExpired  Status = "expired"
)

type Card struct {
	Code        string
	MachineID   string
	AgentID     string
	CreatedAt   time.Time
	ActivatedAt *time.Time
	ExpiresAt   time.Time
	Status      Status
	Note        string
	MaxSessions int
}

type CodeGenerator func() (string, error)
type Clock func() time.Time

type LifecycleService struct {
	generateCode CodeGenerator
	now          Clock
}

func NewLifecycleService(generateCode CodeGenerator, now Clock) *LifecycleService {
	if now == nil {
		now = time.Now
	}
	return &LifecycleService{generateCode: generateCode, now: now}
}

func (s *LifecycleService) Create(duration time.Duration, note string, maxSessions int, agentID string) (Card, error) {
	code, err := s.generateCode()
	if err != nil {
		return Card{}, err
	}
	now := s.now()
	return Card{
		Code:        code,
		AgentID:     agentID,
		CreatedAt:   now,
		ExpiresAt:   now.Add(duration),
		Status:      StatusActive,
		Note:        note,
		MaxSessions: maxSessions,
	}, nil
}

func (s *LifecycleService) UpdateStatus(card Card, status Status) Card {
	card.Status = status
	return card
}

func (s *LifecycleService) Extend(card Card, duration time.Duration) Card {
	card.ExpiresAt = card.ExpiresAt.Add(duration)
	if card.Status == StatusExpired {
		card.Status = StatusActive
	}
	return card
}

func (s *LifecycleService) UpdateDetails(card Card, note *string, maxSessions *int) Card {
	if note != nil {
		card.Note = *note
	}
	if maxSessions != nil {
		if *maxSessions < 1 {
			*maxSessions = 1
		}
		card.MaxSessions = *maxSessions
	}
	return card
}
