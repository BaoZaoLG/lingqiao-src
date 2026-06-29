package main

import (
	"fmt"
	"strings"
	"time"

	agentsvc "github.com/lingqiao/server/internal/agents"
)

func (cm *JSONCardManager) BatchGenerateCards(count int, duration time.Duration, note string, maxSessions int, agentID string) ([]*Card, error) {
	if count < 1 {
		count = 1
	}
	if count > 100 {
		count = 100
	}
	cards := make([]*Card, 0, count)
	cm.mu.Lock()
	for i := 0; i < count; i++ {
		code, err := generateCardCode()
		if err != nil {
			cm.mu.Unlock()
			return nil, err
		}
		card := &Card{
			Code:        code,
			AgentID:     agentID,
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(duration),
			Status:      CardActive,
			Note:        note,
			MaxSessions: maxSessions,
		}
		cm.cards[normalizeCardCode(code)] = card
		cards = append(cards, card)
	}
	cm.appendAuditLocked(AuditEntry{
		Time:    time.Now(),
		Action:  "cards_batch_generated",
		AgentID: agentID,
		Detail:  fmt.Sprintf("count=%d duration=%s max_sessions=%d note=%s", count, duration, maxSessions, note),
	})
	cm.mu.Unlock()
	cm.save()
	return cards, nil
}

// UpdateCardDetails updates a card's note and/or max_sessions.
func (cm *JSONCardManager) UpdateCardDetails(code string, note *string, maxSessions *int) error {
	code = normalizeCardCode(code)
	cm.mu.Lock()
	card, exists := cm.cards[code]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("card not found")
	}
	changes := []string{}
	updated := newCardLifecycleService().UpdateDetails(lifecycleCardFromCard(card), note, maxSessions)
	applyLifecycleCard(card, updated)
	if note != nil {
		changes = append(changes, fmt.Sprintf("note=%s", *note))
	}
	if maxSessions != nil {
		changes = append(changes, fmt.Sprintf("max_sessions=%d", card.MaxSessions))
	}
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "card_details_updated",
		Card:   code,
		Detail: strings.Join(changes, " "),
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// SearchCards returns cards matching the given filters.
// query matches code prefix or note substring. status filters by CardStatus.
// machineID filters by bound machine.
func (cm *JSONCardManager) SearchCards(query, status, machineID string) []*Card {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	query = strings.ToUpper(query)
	result := make([]*Card, 0)
	for _, c := range cm.cards {
		if status != "" && string(c.Status) != status {
			continue
		}
		if machineID != "" && c.MachineID != machineID {
			continue
		}
		if query != "" {
			normCode := normalizeCardCode(c.Code)
			normNote := strings.ToUpper(c.Note)
			if !strings.Contains(normCode, query) && !strings.Contains(normNote, query) {
				continue
			}
		}
		result = append(result, cloneCard(c))
	}
	return result
}

// GetUniqueMachines returns all unique machine IDs with stats.
func (cm *JSONCardManager) GetUniqueMachines() []MachineInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	machineMap := make(map[string]*MachineInfo)
	now := time.Now()
	for _, c := range cm.cards {
		if c.MachineID == "" {
			continue
		}
		info, exists := machineMap[c.MachineID]
		if !exists {
			info = &MachineInfo{MachineID: c.MachineID}
			machineMap[c.MachineID] = info
		}
		info.CardCount++
	}
	for _, s := range cm.sessions {
		if info, ok := machineMap[s.MachineID]; ok {
			if info.LastSeen == nil || s.LastSeen.After(*info.LastSeen) {
				info.LastSeen = &s.LastSeen
			}
		}
	}
	for _, entry := range cm.blacklist {
		if entry.Type == "machine" {
			if info, ok := machineMap[entry.Value]; ok {
				info.IsBlacklisted = true
			}
		}
	}
	_ = now
	result := make([]MachineInfo, 0, len(machineMap))
	for _, info := range machineMap {
		result = append(result, *info)
	}
	return result
}

// GetCardsByMachine returns all cards bound to a machine.
func (cm *JSONCardManager) GetCardsByMachine(machineID string) []*Card {
	return cm.SearchCards("", "", machineID)
}

// MachineInfo holds aggregated info about a machine.
type MachineInfo struct {
	MachineID     string     `json:"machine_id"`
	CardCount     int        `json:"card_count"`
	LastSeen      *time.Time `json:"last_seen,omitempty"`
	IsBlacklisted bool       `json:"is_blacklisted"`
}

// ========== Agent CRUD ==========

// CreateAgent creates a new agent account.
func (cm *JSONCardManager) CreateAgent(username, passwordHash, prefix string) (*Agent, error) {
	cm.mu.Lock()
	existing := make([]agentsvc.Account, 0, len(cm.agents))
	for _, a := range cm.agents {
		existing = append(existing, accountFromAgent(a))
	}
	account, err := newAccountService().Create(existing, username, passwordHash, prefix)
	if err != nil {
		cm.mu.Unlock()
		return nil, err
	}
	agent := agentFromAccount(account)
	cm.agents[agent.ID] = agent
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "agent_created",
		Detail: fmt.Sprintf("agent_id=%s username=%s", agent.ID, username),
	})
	cm.mu.Unlock()
	cm.save()
	return cloneAgent(agent), nil
}

func (cm *JSONCardManager) UsernameExists(username string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, a := range cm.agents {
		if a.Username == username {
			return true
		}
	}
	return false
}

// GetAgent returns an agent by ID.
func (cm *JSONCardManager) GetAgent(id string) *Agent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cloneAgent(cm.agents[id])
}

func (cm *JSONCardManager) UpdateAgentPassword(id, passwordHash string) error {
	cm.mu.Lock()
	agent, exists := cm.agents[id]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("agent not found")
	}
	account, err := newAccountService().UpdatePassword(accountFromAgent(agent), passwordHash)
	if err != nil {
		cm.mu.Unlock()
		return err
	}
	agent.Password = account.Password
	cm.appendAuditLocked(AuditEntry{
		Time:    time.Now(),
		Action:  "agent_password_updated",
		AgentID: id,
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// AllAgents returns all agents.
func (cm *JSONCardManager) AllAgents() []*Agent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Agent, 0, len(cm.agents))
	for _, a := range cm.agents {
		result = append(result, cloneAgent(a))
	}
	return result
}

// UpdateAgentStatus enables or disables an agent.
func (cm *JSONCardManager) UpdateAgentStatus(id string, disabled bool) error {
	cm.mu.Lock()
	agent, exists := cm.agents[id]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("agent not found")
	}
	account := newAccountService().UpdateStatus(accountFromAgent(agent), disabled)
	agent.Disabled = account.Disabled
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "agent_status_changed",
		Detail: fmt.Sprintf("agent_id=%s disabled=%v", id, disabled),
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// DeleteAgent removes an agent.
func (cm *JSONCardManager) DeleteAgent(id string) error {
	cm.mu.Lock()
	agent, exists := cm.agents[id]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("agent not found")
	}
	account := accountFromAgent(agent)
	if err := newAccountService().EnsureCanDelete(&account); err != nil {
		cm.mu.Unlock()
		return err
	}
	delete(cm.agents, id)
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "agent_deleted",
		Detail: fmt.Sprintf("agent_id=%s", id),
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// AgentCards returns all cards belonging to an agent.
func (cm *JSONCardManager) AgentCards(agentID string) []*Card {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Card, 0)
	for _, c := range cm.cards {
		if c.AgentID == agentID {
			result = append(result, cloneCard(c))
		}
	}
	return result
}

// AgentStats returns stats for an agent.
func (cm *JSONCardManager) AgentStats(agentID string) (total, active, expired int) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	now := time.Now()
	for _, c := range cm.cards {
		if c.AgentID != agentID {
			continue
		}
		total++
		if c.Status == CardActive && now.Before(c.ExpiresAt) {
			active++
		}
		if now.After(c.ExpiresAt) || c.Status == CardExpired {
			expired++
		}
	}
	return
}
