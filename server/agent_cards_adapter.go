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
}

func (a *agentCardManagerAdapter) AgentCards(agentID string) []agents.CardSnapshot {
	cardsList, err := a.cardsSvc.AgentCards(context.Background(), agentID)
	if err != nil {
		return nil
	}
	result := make([]agents.CardSnapshot, 0, len(cardsList))
	for _, c := range cardsList {
		result = append(result, agents.CardSnapshot{
			Code:        c.Code,
			MachineID:   c.MachineID,
			AgentID:     c.AgentID,
			CreatedAt:   c.CreatedAt,
			ActivatedAt: c.ActivatedAt,
			ExpiresAt:   c.ExpiresAt,
			Status:      string(c.Status),
			Note:        c.Note,
			MaxSessions: c.MaxSessions,
		})
	}
	return result
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
	}

	c, err := a.cardsSvc.Create(context.Background(), duration, note, maxSessions, agentID)
	if err != nil {
		return agents.CardSnapshot{}, err
	}
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
	}, nil
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
	}

	cardsList, err := a.cardsSvc.BatchCreate(context.Background(), count, duration, note, maxSessions, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]agents.CardSnapshot, 0, len(cardsList))
	for _, c := range cardsList {
		result = append(result, agents.CardSnapshot{
			Code:        c.Code,
			MachineID:   c.MachineID,
			AgentID:     c.AgentID,
			CreatedAt:   c.CreatedAt,
			ActivatedAt: c.ActivatedAt,
			ExpiresAt:   c.ExpiresAt,
			Status:      string(c.Status),
			Note:        c.Note,
			MaxSessions: c.MaxSessions,
		})
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
		result = append(result, agents.CardSnapshot{
			Code:        c.Code,
			MachineID:   c.MachineID,
			AgentID:     c.AgentID,
			CreatedAt:   c.CreatedAt,
			ActivatedAt: c.ActivatedAt,
			ExpiresAt:   c.ExpiresAt,
			Status:      string(c.Status),
			Note:        c.Note,
			MaxSessions: c.MaxSessions,
		})
	}
	return result
}
