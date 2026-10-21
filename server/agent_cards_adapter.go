// agentCardManagerAdapter is a root compatibility adapter mapping the agents feature to the cards service.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
)

type agentCardManagerAdapter struct {
	cardsSvc  *cards.Service
	agentsSvc *agents.Service
	now       func() time.Time
}

func (a *agentCardManagerAdapter) AgentCards(agentID string) []agents.CardSnapshot {
	cardsList, err := a.cardsSvc.AgentCards(context.Background(), agentID)
	if err != nil {
		return nil
	}
	result := make([]agents.CardSnapshot, 0, len(cardsList))
	for _, c := range cardsList {
		result = append(result, toAgentCardSnapshot(c))
	}
	return result
}

func (a *agentCardManagerAdapter) currentTime() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

func (a *agentCardManagerAdapter) checkLimits(agent *agents.Account, count int, duration time.Duration, maxSessions int) error {
	if agent.MaxSessionDurationMinutes > 0 &&
		duration > time.Duration(agent.MaxSessionDurationMinutes)*time.Minute {
		return fmt.Errorf("duration exceeds maximum allowed (%d minutes)", agent.MaxSessionDurationMinutes)
	}
	if agent.MaxConcurrentSessions > 0 && maxSessions > agent.MaxConcurrentSessions {
		return fmt.Errorf("sessions per card exceeds maximum allowed (%d)", agent.MaxConcurrentSessions)
	}
	if agent.MaxDailySessions > 0 {
		cardsList, err := a.cardsSvc.AgentCards(context.Background(), agent.ID)
		if err != nil {
			return err
		}

		today := a.currentTime().UTC().Truncate(24 * time.Hour)
		dailySessions := 0
		for _, c := range cardsList {
			if c.CreatedAt.UTC().Truncate(24 * time.Hour).Equal(today) {
				dailySessions += c.MaxSessions
			}
		}

		if dailySessions+(count*maxSessions) > agent.MaxDailySessions {
			return fmt.Errorf("exceeds maximum daily sessions limit (%d)", agent.MaxDailySessions)
		}
	}
	return nil
}

func (a *agentCardManagerAdapter) AgentStats(agentID string) (total, active, expired int) {
	t, act, exp, err := a.cardsSvc.AgentStats(context.Background(), agentID)
	if err != nil {
		return 0, 0, 0
	}
	return t, act, exp
}

func (a *agentCardManagerAdapter) GenerateCard(duration time.Duration, note string, maxSessions int, agentID string) (agents.CardSnapshot, error) {
	// Enforce agent active checks
	if agentID != "" {
		agent, err := a.agentsSvc.GetAgent(context.Background(), agentID)
		if err != nil {
			return agents.CardSnapshot{}, err
		}
		if agent == nil {
			return agents.CardSnapshot{}, fmt.Errorf("agent not found")
		}
		if agent.Disabled {
			return agents.CardSnapshot{}, fmt.Errorf("agent is disabled")
		}
		if err := a.checkLimits(agent, 1, duration, maxSessions); err != nil {
			return agents.CardSnapshot{}, err
		}
	}

	c, err := a.cardsSvc.Create(context.Background(), duration, note, maxSessions, agentID)
	if err != nil {
		return agents.CardSnapshot{}, err
	}
	return toAgentCardSnapshot(c), nil
}

func (a *agentCardManagerAdapter) BatchGenerateCards(count int, duration time.Duration, note string, maxSessions int, agentID string) ([]agents.CardSnapshot, error) {
	// Enforce agent active checks
	if agentID != "" {
		agent, err := a.agentsSvc.GetAgent(context.Background(), agentID)
		if err != nil {
			return nil, err
		}
		if agent == nil {
			return nil, fmt.Errorf("agent not found")
		}
		if agent.Disabled {
			return nil, fmt.Errorf("agent is disabled")
		}
		if err := a.checkLimits(agent, count, duration, maxSessions); err != nil {
			return nil, err
		}
	}

	cardsList, err := a.cardsSvc.BatchCreate(context.Background(), count, duration, note, maxSessions, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]agents.CardSnapshot, 0, len(cardsList))
	for _, c := range cardsList {
		result = append(result, toAgentCardSnapshot(c))
	}
	return result, nil
}

func (a *agentCardManagerAdapter) AllCards() []agents.CardSnapshot {
	cardsList, err := a.cardsSvc.AllCards(context.Background())
	if err != nil {
		return nil
	}
	result := make([]agents.CardSnapshot, 0, len(cardsList))
	for _, c := range cardsList {
		result = append(result, toAgentCardSnapshot(c))
	}
	return result
}

func toAgentCardSnapshot(c cards.Card) agents.CardSnapshot {
	return agents.CardSnapshot{
		Code:        c.Code,
		MachineID:   c.MachineID,
		AgentID:     c.AgentID,
		CreatedAt:   c.CreatedAt,
		ActivatedAt: c.ActivatedAt,
		ExpiresAt:   c.ExpiresAt,
		Status:      string(c.Status),
		Note:        c.Note,
		MaxSessions: c.MaxSessions,
	}
}
