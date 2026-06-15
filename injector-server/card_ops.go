package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	auditsvc "github.com/lingqiao/server/internal/audit"
	cardops "github.com/lingqiao/server/internal/cards"
)

func NewCardManager(storage *JSONStorage) *CardManager {
	cm := &CardManager{
		cards:     make(map[string]*Card),
		sessions:  make(map[string]*Session),
		blacklist: make(map[string]*BlacklistEntry),
		agents:    make(map[string]*Agent),
		auditLog:  make([]AuditEntry, 0),
		storage:   storage,
	}
	cm.load()
	return cm
}

func (cm *CardManager) SetAuditRecorder(recorder *auditsvc.Recorder) {
	cm.mu.Lock()
	cm.recorder = recorder
	cm.mu.Unlock()
}

func (cm *CardManager) appendAuditLocked(entry AuditEntry) {
	cm.auditLog = append(cm.auditLog, entry)
	if cm.recorder != nil {
		cm.recorder.Append(auditsvc.Event{
			Time:    entry.Time,
			Action:  entry.Action,
			Card:    entry.Card,
			AgentID: entry.AgentID,
			Machine: entry.Machine,
			IP:      entry.Addr,
			Detail:  entry.Detail,
		})
	}
}

func (cm *CardManager) RecordAudit(entry AuditEntry) {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	cm.mu.Lock()
	cm.appendAuditLocked(entry)
	cm.mu.Unlock()
	cm.save()
}

func (cm *CardManager) load() {
	var data struct {
		Cards     map[string]*Card    `json:"cards"`
		Sessions  map[string]*Session `json:"sessions"`
		Blacklist []BlacklistEntry    `json:"blacklist"`
		Agents    map[string]*Agent   `json:"agents"`
		Audit     []AuditEntry        `json:"audit"`
	}
	if err := cm.storage.Load("data", &data); err == nil {
		cm.mu.Lock()
		if data.Cards != nil {
			cm.cards = make(map[string]*Card, len(data.Cards))
			for code, card := range data.Cards {
				cm.cards[normalizeCardCode(code)] = card
			}
		}
		if data.Sessions != nil {
			cm.sessions = data.Sessions
		}
		if data.Blacklist != nil {
			for i := range data.Blacklist {
				cm.blacklist[data.Blacklist[i].Value] = &data.Blacklist[i]
			}
		}
		if data.Agents != nil {
			cm.agents = data.Agents
		}
		if data.Audit != nil {
			cm.auditLog = data.Audit
		}
		cm.mu.Unlock()
	}
}

// save persists current state. Must NOT be called while holding cm.mu.Lock().
func (cm *CardManager) save() {
	cm.mu.RLock()
	// Deep copy all maps to avoid concurrent map iteration/write during JSON marshal
	cards := make(map[string]*Card, len(cm.cards))
	for k, v := range cm.cards {
		cards[k] = cloneCard(v)
	}
	sessions := make(map[string]*Session, len(cm.sessions))
	for k, v := range cm.sessions {
		sessions[k] = cloneSession(v)
	}
	agents := make(map[string]*Agent, len(cm.agents))
	for k, v := range cm.agents {
		agents[k] = cloneAgent(v)
	}
	bl := make([]BlacklistEntry, 0, len(cm.blacklist))
	for _, e := range cm.blacklist {
		bl = append(bl, *e)
	}
	audit := make([]AuditEntry, len(cm.auditLog))
	copy(audit, cm.auditLog)
	cm.mu.RUnlock()

	data := struct {
		Cards     map[string]*Card    `json:"cards"`
		Sessions  map[string]*Session `json:"sessions"`
		Blacklist []BlacklistEntry    `json:"blacklist"`
		Agents    map[string]*Agent   `json:"agents"`
		Audit     []AuditEntry        `json:"audit"`
	}{
		Cards:     cards,
		Sessions:  sessions,
		Blacklist: bl,
		Agents:    agents,
		Audit:     audit,
	}

	if err := cm.storage.Save("data", &data); err != nil {
		log.Printf("[ERROR] Failed to persist data: %v", err)
	}
	cm.rotateAuditLog()
}

const maxAuditEntries = 10000

// rotateAuditLog archives old audit entries to file and keeps only the latest maxAuditEntries in memory.
func (cm *CardManager) rotateAuditLog() {
	cm.mu.Lock()
	if len(cm.auditLog) <= maxAuditEntries {
		cm.mu.Unlock()
		return
	}
	overflow := cm.auditLog[:len(cm.auditLog)-maxAuditEntries]
	cm.auditLog = cm.auditLog[len(cm.auditLog)-maxAuditEntries:]
	cm.mu.Unlock()

	// Append overflow to audit archive file
	f, err := os.OpenFile(dataPath("audit_archive.json"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[STORAGE] Failed to open audit archive: %v", err)
		return
	}
	defer f.Close()
	for _, entry := range overflow {
		data, _ := json.Marshal(entry)
		f.Write(append(data, '\n'))
	}
	log.Printf("[STORAGE] Rotated %d audit entries to archive", len(overflow))
}

// GenerateCard creates a new card with the given duration.
func (cm *CardManager) GenerateCard(duration time.Duration, note string, maxSessions int, agentID string) (*Card, error) {
	lifecycleCard, err := newCardLifecycleService().Create(duration, note, maxSessions, agentID)
	if err != nil {
		return nil, err
	}
	card := cardFromLifecycleCard(lifecycleCard)

	cm.mu.Lock()
	cm.cards[normalizeCardCode(card.Code)] = card
	cm.appendAuditLocked(AuditEntry{
		Time:    time.Now(),
		Action:  "card_generated",
		Card:    card.Code,
		AgentID: agentID,
		Detail:  fmt.Sprintf("duration=%s max_sessions=%d", duration, maxSessions),
	})
	cm.mu.Unlock()
	cm.save()
	return card, nil
}

// GenerateCardWithCode creates a new card using an explicit code, normally from import.
func (cm *CardManager) GenerateCardWithCode(code string, duration time.Duration, note string, maxSessions int, agentID string) (*Card, error) {
	normalized := normalizeCardCode(code)
	if len(normalized) != 18 {
		return nil, fmt.Errorf("card code must contain 18 base32 characters")
	}
	for i := 0; i < len(normalized); i++ {
		if crockfordDecode[normalized[i]] < 0 {
			return nil, fmt.Errorf("card code contains invalid character %q", normalized[i])
		}
	}
	if duration <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	if maxSessions < 1 {
		maxSessions = 1
	}

	now := time.Now()
	card := &Card{
		Code:        FormatCardCode(normalized),
		AgentID:     agentID,
		CreatedAt:   now,
		ExpiresAt:   now.Add(duration),
		Status:      CardActive,
		Note:        note,
		MaxSessions: maxSessions,
	}

	cm.mu.Lock()
	if _, exists := cm.cards[normalized]; exists {
		cm.mu.Unlock()
		return nil, fmt.Errorf("card already exists")
	}
	cm.cards[normalized] = card
	cm.appendAuditLocked(AuditEntry{
		Time:    now,
		Action:  "card_imported",
		Card:    card.Code,
		AgentID: agentID,
		Detail:  fmt.Sprintf("duration=%s max_sessions=%d", duration, maxSessions),
	})
	cm.mu.Unlock()
	cm.save()
	return cloneCard(card), nil
}

// ValidateCard checks if a card is valid and returns it.
func (cm *CardManager) ValidateCard(code string) (*Card, error) {
	code = normalizeCardCode(code)
	cm.mu.RLock()
	card, exists := cm.cards[code]
	_, blacklisted := cm.blacklist[code]
	cm.mu.RUnlock()

	if !exists {
		return nil, cardops.ErrCardNotFound
	}
	if blacklisted {
		return nil, cardops.ErrCardBlacklisted
	}
	if card.Status == CardDisabled {
		return nil, cardops.ErrCardDisabled
	}
	if card.Status == CardExpired || time.Now().After(card.ExpiresAt) {
		return nil, cardops.ErrCardExpired
	}
	return card, nil
}

// ActivateCard binds a card to a machine and creates a session.
func (cm *CardManager) ActivateCard(code, machineID, fingerprint, addr, clientVersion string) (*Session, error) {
	code = normalizeCardCode(code)
	card, err := cm.ValidateCard(code)
	if err != nil {
		return nil, err
	}

	// Single lock for both count check and session creation (prevents TOCTOU race)
	cm.mu.Lock()

	activeSessions := make([]cardops.Session, 0, len(cm.sessions))
	for _, s := range cm.sessions {
		activeSessions = append(activeSessions, lifecycleSessionFromSession(s))
	}

	token := generateSessionToken()
	updatedCard, lifecycleSession, err := newCardSessionService().Activate(cardops.ActivationInput{
		Card:           lifecycleCardFromCard(card),
		Token:          token,
		MachineID:      machineID,
		Fingerprint:    fingerprint,
		RemoteAddr:     addr,
		ClientVersion:  clientVersion,
		ActiveSessions: activeSessions,
	})
	if err != nil {
		cm.mu.Unlock()
		return nil, err
	}
	lifecycleSession.CardCode = code
	applyLifecycleCard(card, updatedCard)
	session := sessionFromLifecycleSession(lifecycleSession)
	cm.sessions[token] = session
	cm.appendAuditLocked(AuditEntry{
		Time:    time.Now(),
		Action:  "card_activated",
		Card:    code,
		Machine: machineID,
		Addr:    addr,
	})
	cm.mu.Unlock()
	cm.save()
	return session, nil
}

// Heartbeat extends a session's expiry.
func (cm *CardManager) Heartbeat(token, machineID, addr, clientVersion string) (*Session, error) {
	cm.mu.Lock()

	session, exists := cm.sessions[token]
	if !exists {
		cm.mu.Unlock()
		return nil, fmt.Errorf("session not found")
	}

	card, exists := cm.cards[session.CardCode]
	if !exists {
		delete(cm.sessions, token)
		cm.mu.Unlock()
		cm.save()
		return nil, fmt.Errorf("card no longer valid")
	}

	lifecycleSession, err := newCardSessionService().Heartbeat(cardops.HeartbeatInput{
		Session:       lifecycleSessionFromSession(session),
		Card:          lifecycleCardFromCard(card),
		MachineID:     machineID,
		RemoteAddr:    addr,
		ClientVersion: clientVersion,
	})
	if err != nil {
		if errors.Is(err, cardops.ErrSessionExpired) || errors.Is(err, cardops.ErrCardNoLongerValid) {
			delete(cm.sessions, token)
			cm.mu.Unlock()
			cm.save()
			return nil, err
		}
		cm.mu.Unlock()
		return nil, err
	}
	applyLifecycleSession(session, lifecycleSession)
	cm.mu.Unlock()
	cm.save()
	return session, nil
}

// DeactivateSession ends a session and optionally unbinds the card.
func (cm *CardManager) DeactivateSession(token string, unbind bool) error {
	cm.mu.Lock()

	session, exists := cm.sessions[token]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("session not found")
	}

	cardCode := session.CardCode
	delete(cm.sessions, token)

	if unbind {
		if card, exists := cm.cards[cardCode]; exists {
			if card.ActivatedAt != nil {
				// Preserve remaining time from now, not from creation
				remaining := time.Until(card.ExpiresAt)
				if remaining < 0 {
					remaining = 0
				}
				card.ExpiresAt = time.Now().Add(remaining)
			}
			card.MachineID = ""
			card.ActivatedAt = nil
		}
	}

	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "session_deactivated",
		Card:   cardCode,
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// GetCard returns a card by code.
func (cm *CardManager) GetCard(code string) *Card {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cloneCard(cm.cards[normalizeCardCode(code)])
}

// AllCards returns all cards.
func (cm *CardManager) AllCards() []*Card {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Card, 0, len(cm.cards))
	for _, c := range cm.cards {
		result = append(result, cloneCard(c))
	}
	return result
}

// AllSessions returns all sessions.
func (cm *CardManager) AllSessions() []*Session {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Session, 0, len(cm.sessions))
	for _, s := range cm.sessions {
		result = append(result, cloneSession(s))
	}
	return result
}

// AllBlacklist returns all blacklist entries.
func (cm *CardManager) AllBlacklist() []*BlacklistEntry {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*BlacklistEntry, 0, len(cm.blacklist))
	for _, e := range cm.blacklist {
		result = append(result, cloneBlacklistEntry(e))
	}
	return result
}

// AuditLog returns all audit entries.
func (cm *CardManager) AuditLog() []AuditEntry {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]AuditEntry, len(cm.auditLog))
	copy(result, cm.auditLog)
	return result
}

// AddBlacklist adds a blacklist entry.
func (cm *CardManager) AddBlacklist(typ, value, reason string) {
	cm.mu.Lock()
	cm.blacklist[value] = &BlacklistEntry{
		Type:      typ,
		Value:     value,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "blacklist_added",
		Detail: fmt.Sprintf("type=%s value=%s reason=%s", typ, value, reason),
	})
	cm.mu.Unlock()
	cm.save()
}

// RemoveBlacklist removes a blacklist entry.
func (cm *CardManager) RemoveBlacklist(value string) {
	cm.mu.Lock()
	delete(cm.blacklist, value)
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "blacklist_removed",
		Detail: fmt.Sprintf("value=%s", value),
	})
	cm.mu.Unlock()
	cm.save()
}

// UpdateCardStatus updates a card's status.
func (cm *CardManager) UpdateCardStatus(code string, status CardStatus) error {
	code = normalizeCardCode(code)
	cm.mu.Lock()
	card, exists := cm.cards[code]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("card not found")
	}
	updated := newCardLifecycleService().UpdateStatus(lifecycleCardFromCard(card), cardops.Status(status))
	applyLifecycleCard(card, updated)
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "card_status_changed",
		Card:   code,
		Detail: fmt.Sprintf("new_status=%s", status),
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// ExtendCard extends a card's expiry by the given duration.
func (cm *CardManager) ExtendCard(code string, duration time.Duration) error {
	code = normalizeCardCode(code)
	cm.mu.Lock()
	card, exists := cm.cards[code]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("card not found")
	}
	updated := newCardLifecycleService().Extend(lifecycleCardFromCard(card), duration)
	applyLifecycleCard(card, updated)
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "card_extended",
		Card:   code,
		Detail: fmt.Sprintf("added_duration=%s new_expires=%s", duration, card.ExpiresAt.Format(time.RFC3339)),
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

func (cm *CardManager) UnbindCard(code string) error {
	code = normalizeCardCode(code)
	cm.mu.Lock()
	card, exists := cm.cards[code]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("card not found")
	}
	if card.ActivatedAt != nil {
		originalDuration := card.ExpiresAt.Sub(*card.ActivatedAt)
		card.ExpiresAt = card.CreatedAt.Add(originalDuration)
	}
	card.MachineID = ""
	card.ActivatedAt = nil
	cm.appendAuditLocked(AuditEntry{
		Time:   time.Now(),
		Action: "card_unbound",
		Card:   code,
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// CleanupExpired removes expired sessions.
func (cm *CardManager) CleanupExpired() {
	cm.mu.Lock()
	now := time.Now()
	for token, session := range cm.sessions {
		if session.ExpiresAt.Before(now) {
			delete(cm.sessions, token)
		}
	}
	cm.mu.Unlock()
	cm.save()
}

// IsBlacklisted checks if a value is blacklisted.
func (cm *CardManager) IsBlacklisted(value string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, exists := cm.blacklist[value]
	return exists
}

// BatchGenerateCards creates multiple cards at once.
