package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Crockford Base32 alphabet (excludes 0O1IL for readability)
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var crockfordDecode [256]int8

func init() {
	for i := range crockfordDecode {
		crockfordDecode[i] = -1
	}
	for i, c := range crockford {
		crockfordDecode[c] = int8(i)
		if c >= 'A' && c <= 'Z' {
			crockfordDecode[c+32] = int8(i)
		}
	}
}

type CardStatus string

const (
	CardActive   CardStatus = "active"
	CardDisabled CardStatus = "disabled"
	CardExpired  CardStatus = "expired"
)

type Card struct {
	Code        string     `json:"code"`
	MachineID   string     `json:"machine_id,omitempty"`
	AgentID     string     `json:"agent_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Status      CardStatus `json:"status"`
	Note        string     `json:"note,omitempty"`
	MaxSessions int        `json:"max_sessions"`
}

type Session struct {
	Token         string    `json:"token"`
	CardCode      string    `json:"card_code"`
	MachineID     string    `json:"machine_id"`
	Fingerprint   string    `json:"fingerprint"`
	ClientVersion string    `json:"client_version,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	LastSeen      time.Time `json:"last_seen"`
	ExpiresAt     time.Time `json:"expires_at"`
	RemoteAddr    string    `json:"remote_addr"`
}

type AuditEntry struct {
	Time    time.Time `json:"time"`
	Action  string    `json:"action"`
	Card    string    `json:"card,omitempty"`
	Machine string    `json:"machine,omitempty"`
	AgentID string    `json:"agent_id,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	Addr    string    `json:"addr,omitempty"`
}

type Agent struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"` // SHA-256 hash
	Prefix    string    `json:"prefix"`   // card code prefix for identification
	CreatedAt time.Time `json:"created_at"`
	Disabled  bool      `json:"disabled"`
}

type BlacklistEntry struct {
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type CardManager struct {
	mu        sync.RWMutex
	cards     map[string]*Card
	sessions  map[string]*Session
	blacklist map[string]*BlacklistEntry
	agents    map[string]*Agent
	auditLog  []AuditEntry
	storage   *JSONStorage
}

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
		cp := *v
		cards[k] = &cp
	}
	sessions := make(map[string]*Session, len(cm.sessions))
	for k, v := range cm.sessions {
		cp := *v
		sessions[k] = &cp
	}
	agents := make(map[string]*Agent, len(cm.agents))
	for k, v := range cm.agents {
		cp := *v
		agents[k] = &cp
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
	f, err := os.OpenFile("data/audit_archive.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	code, err := generateCardCode()
	if err != nil {
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

	cm.mu.Lock()
	cm.cards[normalizeCardCode(code)] = card
	cm.auditLog = append(cm.auditLog, AuditEntry{
		Time:    time.Now(),
		Action:  "card_generated",
		Card:    code,
		AgentID: agentID,
		Detail:  fmt.Sprintf("duration=%s max_sessions=%d", duration, maxSessions),
	})
	cm.mu.Unlock()
	cm.save()
	return card, nil
}

// ValidateCard checks if a card is valid and returns it.
func (cm *CardManager) ValidateCard(code string) (*Card, error) {
	code = normalizeCardCode(code)
	cm.mu.RLock()
	card, exists := cm.cards[code]
	_, blacklisted := cm.blacklist[code]
	cm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("card not found")
	}
	if blacklisted {
		return nil, fmt.Errorf("card is blacklisted")
	}
	if card.Status == CardDisabled {
		return nil, fmt.Errorf("card is disabled")
	}
	if card.Status == CardExpired || time.Now().After(card.ExpiresAt) {
		return nil, fmt.Errorf("card has expired")
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

	if card.MachineID != "" && card.MachineID != machineID {
		return nil, fmt.Errorf("card already bound to another machine")
	}

	// Single lock for both count check and session creation (prevents TOCTOU race)
	cm.mu.Lock()

	activeCount := 0
	now := time.Now()
	for _, s := range cm.sessions {
		if s.CardCode == code && s.ExpiresAt.After(now) {
			activeCount++
		}
	}
	if maxSessions := card.MaxSessions; maxSessions > 0 && activeCount >= maxSessions {
		cm.mu.Unlock()
		return nil, fmt.Errorf("max active sessions reached (%d/%d)", activeCount, maxSessions)
	}

	card.MachineID = machineID
	if card.ActivatedAt == nil {
			card.ActivatedAt = &now
			// Start billing from first activation
			duration := card.ExpiresAt.Sub(card.CreatedAt)
			card.ExpiresAt = now.Add(duration)
	}

	token := generateSessionToken()
	session := &Session{
		Token:         token,
		CardCode:      code,
		MachineID:     machineID,
		Fingerprint:   fingerprint,
		ClientVersion: clientVersion,
		CreatedAt:     time.Now(),
		LastSeen:      time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		RemoteAddr:    addr,
	}
	cm.sessions[token] = session
	cm.auditLog = append(cm.auditLog, AuditEntry{
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
	if session.MachineID != machineID {
		cm.mu.Unlock()
		return nil, fmt.Errorf("machine mismatch")
	}
	if session.ExpiresAt.Before(time.Now()) {
		delete(cm.sessions, token)
		cm.mu.Unlock()
		cm.save()
		return nil, fmt.Errorf("session expired")
	}

	card, exists := cm.cards[session.CardCode]
	if !exists || card.Status == CardDisabled || time.Now().After(card.ExpiresAt) {
		delete(cm.sessions, token)
		cm.mu.Unlock()
		cm.save()
		return nil, fmt.Errorf("card no longer valid")
	}

	session.LastSeen = time.Now()
	session.ExpiresAt = time.Now().Add(24 * time.Hour)
	session.RemoteAddr = addr
	if clientVersion != "" {
		session.ClientVersion = clientVersion
	}
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

	cm.auditLog = append(cm.auditLog, AuditEntry{
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
	return cm.cards[normalizeCardCode(code)]
}

// AllCards returns all cards.
func (cm *CardManager) AllCards() []*Card {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Card, 0, len(cm.cards))
	for _, c := range cm.cards {
		result = append(result, c)
	}
	return result
}

// AllSessions returns all sessions.
func (cm *CardManager) AllSessions() []*Session {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Session, 0, len(cm.sessions))
	for _, s := range cm.sessions {
		result = append(result, s)
	}
	return result
}

// AllBlacklist returns all blacklist entries.
func (cm *CardManager) AllBlacklist() []*BlacklistEntry {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*BlacklistEntry, 0, len(cm.blacklist))
	for _, e := range cm.blacklist {
		result = append(result, e)
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
	cm.auditLog = append(cm.auditLog, AuditEntry{
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
	cm.auditLog = append(cm.auditLog, AuditEntry{
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
	card.Status = status
	cm.auditLog = append(cm.auditLog, AuditEntry{
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
	card.ExpiresAt = card.ExpiresAt.Add(duration)
	if card.Status == CardExpired {
		card.Status = CardActive
	}
	cm.auditLog = append(cm.auditLog, AuditEntry{
		Time:   time.Now(),
		Action: "card_extended",
		Card:   code,
		Detail: fmt.Sprintf("added_duration=%s new_expires=%s", duration, card.ExpiresAt.Format(time.RFC3339)),
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

// Announcement represents a public announcement with optional version push.
type Announcement struct {
	Content       string    `json:"content"`
	UpdatedAt     time.Time `json:"updated_at"`
	LatestVersion string    `json:"latest_version"`
	MinVersion    string    `json:"min_version"`
	ForceUpdate   bool      `json:"force_update"`
	DownloadURL   string    `json:"download_url,omitempty"`
}

var announcement Announcement
var annMu sync.RWMutex

func GetAnnouncement() *Announcement {
	annMu.RLock()
	defer annMu.RUnlock()
	if announcement.Content == "" && announcement.LatestVersion == "" {
		return nil
	}
	a := announcement
	return &a
}

func SetAnnouncement(content, latestVersion, minVersion string, forceUpdate bool) {
	annMu.Lock()
	dlURL := ""
	if latestVersion != "" {
		dlURL = "/admin/api/update/download"
	}
	announcement = Announcement{
		Content:       content,
		UpdatedAt:     time.Now(),
		LatestVersion: latestVersion,
		MinVersion:    minVersion,
		ForceUpdate:   forceUpdate,
		DownloadURL:   dlURL,
	}
	annMu.Unlock()
	data, _ := json.Marshal(announcement)
	os.WriteFile("data/announcement.json", data, 0600)
}

func initAnnouncement() {
	data, err := os.ReadFile("data/announcement.json")
	if err != nil {
		return
	}
	var a Announcement
	if err := json.Unmarshal(data, &a); err != nil {
		return
	}
	if a.LatestVersion != "" && a.DownloadURL == "" {
		a.DownloadURL = "/admin/api/update/download"
	}
	annMu.Lock()
	announcement = a
	annMu.Unlock()
}

func compareVersion(client, latest string) bool {
	parse := func(s string) []int {
		parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
		nums := make([]int, len(parts))
		for i, p := range parts {
			n := 0
			for _, c := range p {
				if c >= '0' && c <= '9' {
					n = n*10 + int(c-'0')
				} else {
					break
				}
			}
			nums[i] = n
		}
		return nums
	}
	cv := parse(client)
	lv := parse(latest)
	maxLen := len(cv)
	if len(lv) > maxLen {
		maxLen = len(lv)
	}
	for i := 0; i < maxLen; i++ {
		a, b := 0, 0
		if i < len(cv) {
			a = cv[i]
		}
		if i < len(lv) {
			b = lv[i]
		}
		if a < b {
			return true
		}
		if a > b {
			return false
		}
	}
	return false
}

func generateCardCode() (string, error) {
	buf := make([]byte, 15)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	var sb strings.Builder
	for i := 0; i < 24 && sb.Len() < 18; i++ {
		idx := int(buf[i%15]) & 31
		if i < 15 {
			idx = int(buf[i]) & 31
		} else {
			idx = int(buf[i-15]>>5 | buf[i%15]<<3) & 31
		}
		sb.WriteByte(crockford[idx])
	}
	result := sb.String()[:18]

	return fmt.Sprintf("%s-%s-%s",
		result[0:6],
		result[6:12],
		result[12:18],
	), nil
}

func generateSessionToken() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}

func normalizeCardCode(code string) string {
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func FormatCardCode(code string) string {
	code = normalizeCardCode(code)
	if len(code) != 18 {
		return code
	}
	return fmt.Sprintf("%s-%s-%s", code[0:6], code[6:12], code[12:18])
}

// BatchGenerateCards creates multiple cards at once.
func (cm *CardManager) BatchGenerateCards(count int, duration time.Duration, note string, maxSessions int, agentID string) ([]*Card, error) {
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
	cm.auditLog = append(cm.auditLog, AuditEntry{
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
func (cm *CardManager) UpdateCardDetails(code string, note *string, maxSessions *int) error {
	code = normalizeCardCode(code)
	cm.mu.Lock()
	card, exists := cm.cards[code]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("card not found")
	}
	changes := []string{}
	if note != nil {
		card.Note = *note
		changes = append(changes, fmt.Sprintf("note=%s", *note))
	}
	if maxSessions != nil {
		if *maxSessions < 1 {
			*maxSessions = 1
		}
		card.MaxSessions = *maxSessions
		changes = append(changes, fmt.Sprintf("max_sessions=%d", *maxSessions))
	}
	cm.auditLog = append(cm.auditLog, AuditEntry{
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
func (cm *CardManager) SearchCards(query, status, machineID string) []*Card {
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
		result = append(result, c)
	}
	return result
}

// GetUniqueMachines returns all unique machine IDs with stats.
func (cm *CardManager) GetUniqueMachines() []MachineInfo {
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
func (cm *CardManager) GetCardsByMachine(machineID string) []*Card {
	return cm.SearchCards("", "", machineID)
}

// BulkUpdateCards applies an action to multiple cards.
func (cm *CardManager) BulkUpdateCards(codes []string, action string, extendHours int) (int, error) {
	cm.mu.Lock()
	affected := 0
	for _, code := range codes {
		code = normalizeCardCode(code)
		card, exists := cm.cards[code]
		if !exists {
			continue
		}
		switch action {
		case "disable":
			card.Status = CardDisabled
		case "enable":
			card.Status = CardActive
		case "expire":
			card.Status = CardExpired
		case "extend":
			card.ExpiresAt = card.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)
			if card.Status == CardExpired {
				card.Status = CardActive
			}
		case "unbind":
			if card.ActivatedAt != nil {
				originalDuration := card.ExpiresAt.Sub(*card.ActivatedAt)
				card.ExpiresAt = card.CreatedAt.Add(originalDuration)
			}
			card.MachineID = ""
			card.ActivatedAt = nil
		default:
			cm.mu.Unlock()
			return 0, fmt.Errorf("unknown action: %s", action)
		}
		affected++
	}
	cm.auditLog = append(cm.auditLog, AuditEntry{
		Time:   time.Now(),
		Action: "bulk_" + action,
		Detail: fmt.Sprintf("count=%d cards=%s", affected, strings.Join(codes, ",")),
	})
	cm.mu.Unlock()
	cm.save()
	return affected, nil
}

// MachineInfo holds aggregated info about a machine.
type MachineInfo struct {
	MachineID    string     `json:"machine_id"`
	CardCount    int        `json:"card_count"`
	LastSeen     *time.Time `json:"last_seen,omitempty"`
	IsBlacklisted bool      `json:"is_blacklisted"`
}

func init() {
	if err := os.MkdirAll("data", 0755); err != nil {
	}
}

// ========== Agent CRUD ==========

// CreateAgent creates a new agent account.
func (cm *CardManager) CreateAgent(username, passwordHash, prefix string) (*Agent, error) {
	cm.mu.Lock()
	for _, a := range cm.agents {
		if a.Username == username {
			cm.mu.Unlock()
			return nil, fmt.Errorf("username already exists")
		}
	}
	id := fmt.Sprintf("AGT-%s", generateShortID())
	agent := &Agent{
		ID:        id,
		Username:  username,
		Password:  passwordHash,
		Prefix:    prefix,
		CreatedAt: time.Now(),
	}
	cm.agents[id] = agent
	cm.auditLog = append(cm.auditLog, AuditEntry{
		Time:   time.Now(),
		Action: "agent_created",
		Detail: fmt.Sprintf("agent_id=%s username=%s", id, username),
	})
	cm.mu.Unlock()
	cm.save()
	return agent, nil
}

// GetAgent returns an agent by ID.
func (cm *CardManager) GetAgent(id string) *Agent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.agents[id]
}

// AllAgents returns all agents.
func (cm *CardManager) AllAgents() []*Agent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Agent, 0, len(cm.agents))
	for _, a := range cm.agents {
		result = append(result, a)
	}
	return result
}

// UpdateAgentStatus enables or disables an agent.
func (cm *CardManager) UpdateAgentStatus(id string, disabled bool) error {
	cm.mu.Lock()
	agent, exists := cm.agents[id]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("agent not found")
	}
	agent.Disabled = disabled
	cm.auditLog = append(cm.auditLog, AuditEntry{
		Time:   time.Now(),
		Action: "agent_status_changed",
		Detail: fmt.Sprintf("agent_id=%s disabled=%v", id, disabled),
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// DeleteAgent removes an agent.
func (cm *CardManager) DeleteAgent(id string) error {
	cm.mu.Lock()
	if _, exists := cm.agents[id]; !exists {
		cm.mu.Unlock()
		return fmt.Errorf("agent not found")
	}
	delete(cm.agents, id)
	cm.auditLog = append(cm.auditLog, AuditEntry{
		Time:   time.Now(),
		Action: "agent_deleted",
		Detail: fmt.Sprintf("agent_id=%s", id),
	})
	cm.mu.Unlock()
	cm.save()
	return nil
}

// AgentCards returns all cards belonging to an agent.
func (cm *CardManager) AgentCards(agentID string) []*Card {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Card, 0)
	for _, c := range cm.cards {
		if c.AgentID == agentID {
			result = append(result, c)
		}
	}
	return result
}

// AgentStats returns stats for an agent.
func (cm *CardManager) AgentStats(agentID string) (total, active, expired int) {
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

func generateShortID() string {
	buf := make([]byte, 6)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}
