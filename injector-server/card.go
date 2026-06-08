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

	agentsvc "github.com/lingqiao/server/internal/agents"
	auditsvc "github.com/lingqiao/server/internal/audit"
	cardops "github.com/lingqiao/server/internal/cards"
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
	recorder  *auditsvc.Recorder
	storage   *JSONStorage
}

func cloneCard(c *Card) *Card {
	if c == nil {
		return nil
	}
	cp := *c
	if c.ActivatedAt != nil {
		t := *c.ActivatedAt
		cp.ActivatedAt = &t
	}
	return &cp
}

func cloneSession(s *Session) *Session {
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

func cloneAgent(a *Agent) *Agent {
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

func cloneBlacklistEntry(e *BlacklistEntry) *BlacklistEntry {
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}

func newAccountService() *agentsvc.AccountService {
	return agentsvc.NewAccountService(func() (string, error) {
		return generateShortID(), nil
	})
}

func accountFromAgent(agent *Agent) agentsvc.Account {
	if agent == nil {
		return agentsvc.Account{}
	}
	return agentsvc.Account{
		ID:        agent.ID,
		Username:  agent.Username,
		Password:  agent.Password,
		Prefix:    agent.Prefix,
		CreatedAt: agent.CreatedAt,
		Disabled:  agent.Disabled,
	}
}

func agentFromAccount(account agentsvc.Account) *Agent {
	return &Agent{
		ID:        account.ID,
		Username:  account.Username,
		Password:  account.Password,
		Prefix:    account.Prefix,
		CreatedAt: account.CreatedAt,
		Disabled:  account.Disabled,
	}
}

func newCardLifecycleService() *cardops.LifecycleService {
	return cardops.NewLifecycleService(generateCardCode, time.Now)
}

func lifecycleCardFromCard(card *Card) cardops.Card {
	if card == nil {
		return cardops.Card{}
	}
	return cardops.Card{
		Code:        card.Code,
		MachineID:   card.MachineID,
		AgentID:     card.AgentID,
		CreatedAt:   card.CreatedAt,
		ActivatedAt: card.ActivatedAt,
		ExpiresAt:   card.ExpiresAt,
		Status:      cardops.Status(card.Status),
		Note:        card.Note,
		MaxSessions: card.MaxSessions,
	}
}

func applyLifecycleCard(dst *Card, src cardops.Card) {
	dst.Code = src.Code
	dst.MachineID = src.MachineID
	dst.AgentID = src.AgentID
	dst.CreatedAt = src.CreatedAt
	dst.ActivatedAt = src.ActivatedAt
	dst.ExpiresAt = src.ExpiresAt
	dst.Status = CardStatus(src.Status)
	dst.Note = src.Note
	dst.MaxSessions = src.MaxSessions
}

func cardFromLifecycleCard(card cardops.Card) *Card {
	return &Card{
		Code:        card.Code,
		MachineID:   card.MachineID,
		AgentID:     card.AgentID,
		CreatedAt:   card.CreatedAt,
		ActivatedAt: card.ActivatedAt,
		ExpiresAt:   card.ExpiresAt,
		Status:      CardStatus(card.Status),
		Note:        card.Note,
		MaxSessions: card.MaxSessions,
	}
}

func newCardSessionService() *cardops.SessionService {
	return cardops.NewSessionService(time.Now, currentSessionTTL())
}

func lifecycleSessionFromSession(session *Session) cardops.Session {
	if session == nil {
		return cardops.Session{}
	}
	return cardops.Session{
		Token:         session.Token,
		CardCode:      session.CardCode,
		MachineID:     session.MachineID,
		Fingerprint:   session.Fingerprint,
		ClientVersion: session.ClientVersion,
		CreatedAt:     session.CreatedAt,
		LastSeen:      session.LastSeen,
		ExpiresAt:     session.ExpiresAt,
		RemoteAddr:    session.RemoteAddr,
	}
}

func sessionFromLifecycleSession(session cardops.Session) *Session {
	return &Session{
		Token:         session.Token,
		CardCode:      session.CardCode,
		MachineID:     session.MachineID,
		Fingerprint:   session.Fingerprint,
		ClientVersion: session.ClientVersion,
		CreatedAt:     session.CreatedAt,
		LastSeen:      session.LastSeen,
		ExpiresAt:     session.ExpiresAt,
		RemoteAddr:    session.RemoteAddr,
	}
}

func applyLifecycleSession(dst *Session, src cardops.Session) {
	dst.Token = src.Token
	dst.CardCode = src.CardCode
	dst.MachineID = src.MachineID
	dst.Fingerprint = src.Fingerprint
	dst.ClientVersion = src.ClientVersion
	dst.CreatedAt = src.CreatedAt
	dst.LastSeen = src.LastSeen
	dst.ExpiresAt = src.ExpiresAt
	dst.RemoteAddr = src.RemoteAddr
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
		if err.Error() == "session expired" || err.Error() == "card no longer valid" {
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

// Announcement represents a public announcement with optional legacy version push.
type Announcement struct {
	ID            string     `json:"id,omitempty"`
	Content       string     `json:"content"`
	UpdatedAt     time.Time  `json:"updated_at"`
	LatestVersion string     `json:"latest_version"`
	MinVersion    string     `json:"min_version"`
	ForceUpdate   bool       `json:"force_update"`
	DownloadURL   string     `json:"download_url,omitempty"`
	SHA256        string     `json:"sha256,omitempty"`
	Status        string     `json:"status,omitempty"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
}

var announcement Announcement
var annMu sync.RWMutex

const (
	announcementStatusDraft      = "draft"
	announcementStatusPublished  = "published"
	announcementStatusSuperseded = "superseded"
	maxAnnouncementBytes         = 64 << 10
)

type announcementIndex struct {
	ActiveID      string         `json:"active_id"`
	Announcements []Announcement `json:"announcements"`
}

func announcementDir() string { return dataPath("announcements") }

func announcementIndexPath() string { return dataPath("announcements", "index.json") }

func GetAnnouncement() *Announcement {
	annMu.RLock()
	if announcement.Content == "" && announcement.LatestVersion == "" {
		annMu.RUnlock()
		return nil
	}
	a := announcement
	annMu.RUnlock()
	if a.SHA256 == "" && a.LatestVersion != "" {
		a.SHA256 = getUpdateSHA256(a.LatestVersion)
	}
	return &a
}

func SetAnnouncement(content, latestVersion, minVersion string, forceUpdate bool) {
	a, err := SaveAnnouncementRevision(content, latestVersion, minVersion, forceUpdate, true)
	if err != nil {
		log.Printf("[ANNOUNCEMENT] Failed to save revision: %v", err)
		return
	}
	setActiveAnnouncement(*a)
}

func SaveAnnouncementRevision(content, latestVersion, minVersion string, forceUpdate, publish bool) (*Announcement, error) {
	if len([]byte(content)) > maxAnnouncementBytes {
		return nil, fmt.Errorf("content exceeds %d bytes", maxAnnouncementBytes)
	}
	now := time.Now().UTC()
	status := announcementStatusDraft
	var publishedAt *time.Time
	if publish {
		status = announcementStatusPublished
		publishedAt = &now
	}
	dlURL := ""
	if latestVersion != "" {
		dlURL = "/admin/api/update/download"
	}
	a := Announcement{
		ID:            newAnnouncementID(now),
		Content:       content,
		UpdatedAt:     now,
		LatestVersion: latestVersion,
		MinVersion:    minVersion,
		ForceUpdate:   forceUpdate,
		DownloadURL:   dlURL,
		Status:        status,
		CreatedAt:     now,
		PublishedAt:   publishedAt,
	}
	idx, err := readAnnouncementIndex()
	if err != nil {
		return nil, err
	}
	idx.Announcements = append([]Announcement{a}, idx.Announcements...)
	if publish {
		idx.ActiveID = a.ID
		markActiveAnnouncement(&idx, a.ID)
	}
	if err := writeAnnouncementIndex(idx); err != nil {
		return nil, err
	}
	if publish {
		setActiveAnnouncement(a)
	}
	return &a, nil
}

func PublishAnnouncementRevision(id string) (*Announcement, error) {
	idx, err := readAnnouncementIndex()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var selected *Announcement
	for i := range idx.Announcements {
		if idx.Announcements[i].ID == id {
			idx.Announcements[i].Status = announcementStatusPublished
			idx.Announcements[i].UpdatedAt = now
			idx.Announcements[i].PublishedAt = &now
			selected = &idx.Announcements[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("announcement revision not found")
	}
	idx.ActiveID = id
	markActiveAnnouncement(&idx, id)
	if err := writeAnnouncementIndex(idx); err != nil {
		return nil, err
	}
	setActiveAnnouncement(*selected)
	return selected, nil
}

func DeleteAnnouncementRevision(id string) error {
	idx, err := readAnnouncementIndex()
	if err != nil {
		return err
	}
	if idx.ActiveID == id {
		return fmt.Errorf("cannot delete active announcement")
	}
	next := make([]Announcement, 0, len(idx.Announcements))
	found := false
	for _, item := range idx.Announcements {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("announcement revision not found")
	}
	idx.Announcements = next
	return writeAnnouncementIndex(idx)
}

func ListAnnouncementRevisions() ([]Announcement, string, error) {
	idx, err := readAnnouncementIndex()
	if err != nil {
		return nil, "", err
	}
	return idx.Announcements, idx.ActiveID, nil
}

func setActiveAnnouncement(a Announcement) {
	annMu.Lock()
	if a.LatestVersion != "" && a.DownloadURL == "" {
		a.DownloadURL = "/admin/api/update/download"
	}
	announcement = a
	annMu.Unlock()
	data, _ := json.Marshal(announcement)
	os.WriteFile(dataPath("announcement.json"), data, 0600)
}

func initAnnouncement() {
	annMu.Lock()
	announcement = Announcement{}
	annMu.Unlock()

	if idx, err := readAnnouncementIndex(); err == nil {
		for _, a := range idx.Announcements {
			if a.ID == idx.ActiveID {
				setActiveAnnouncement(a)
				return
			}
		}
	}
	data, err := os.ReadFile(dataPath("announcement.json"))
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

func readAnnouncementIndex() (announcementIndex, error) {
	if err := migrateLegacyAnnouncement(); err != nil {
		return announcementIndex{}, err
	}
	var idx announcementIndex
	data, err := os.ReadFile(announcementIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return idx, err
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return idx, err
	}
	return idx, nil
}

func writeAnnouncementIndex(idx announcementIndex) error {
	if err := os.MkdirAll(announcementDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(announcementIndexPath(), data, 0600)
}

func migrateLegacyAnnouncement() error {
	if _, err := os.Stat(announcementIndexPath()); err == nil {
		return nil
	}
	data, err := os.ReadFile(dataPath("announcement.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var a Announcement
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	if a.Content == "" && a.LatestVersion == "" {
		return nil
	}
	now := a.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if a.ID == "" {
		a.ID = newAnnouncementID(now)
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.PublishedAt == nil {
		t := now
		a.PublishedAt = &t
	}
	a.Status = announcementStatusPublished
	return writeAnnouncementIndex(announcementIndex{ActiveID: a.ID, Announcements: []Announcement{a}})
}

func newAnnouncementID(t time.Time) string {
	return fmt.Sprintf("ann-%s-%d", t.Format("20060102T150405000Z"), t.UnixNano())
}

func markActiveAnnouncement(idx *announcementIndex, activeID string) {
	for i := range idx.Announcements {
		switch {
		case idx.Announcements[i].ID == activeID:
			idx.Announcements[i].Status = announcementStatusPublished
		case idx.Announcements[i].Status == announcementStatusPublished:
			idx.Announcements[i].Status = announcementStatusSuperseded
		}
	}
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
			idx = int(buf[i-15]>>5|buf[i%15]<<3) & 31
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
func (cm *CardManager) UpdateCardDetails(code string, note *string, maxSessions *int) error {
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
		result = append(result, cloneCard(c))
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

// BulkUpdateCardsDetailed applies an action to multiple cards and reports per-item results.
func (cm *CardManager) BulkUpdateCardsDetailed(codes []string, action string, extendHours int) (cardops.BulkResult, error) {
	return cm.bulkUpdateCardsDetailed(codes, action, extendHours, false)
}

// PreviewBulkUpdateCardsDetailed reports bulk operation impact without mutating state.
func (cm *CardManager) PreviewBulkUpdateCardsDetailed(codes []string, action string, extendHours int) (cardops.BulkResult, error) {
	return cm.bulkUpdateCardsDetailed(codes, action, extendHours, true)
}

func (cm *CardManager) bulkUpdateCardsDetailed(codes []string, action string, extendHours int, dryRun bool) (cardops.BulkResult, error) {
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

	cm.mu.Lock()
	changed := false
	for _, code := range codes {
		normalized := normalizeCardCode(code)
		card, exists := cm.cards[normalized]
		if !exists {
			result.AddItem(cardops.BulkItemResult{
				Code:    code,
				Status:  cardops.BulkItemSkipped,
				Message: "card not found",
			})
			continue
		}
		if !dryRun {
			switch bulkAction {
			case cardops.BulkDisable:
				card.Status = CardDisabled
			case cardops.BulkEnable:
				card.Status = CardActive
			case cardops.BulkExpire:
				card.Status = CardExpired
			case cardops.BulkExtend:
				card.ExpiresAt = card.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)
				if card.Status == CardExpired {
					card.Status = CardActive
				}
			case cardops.BulkUnbind:
				if card.ActivatedAt != nil {
					originalDuration := card.ExpiresAt.Sub(*card.ActivatedAt)
					card.ExpiresAt = card.CreatedAt.Add(originalDuration)
				}
				card.MachineID = ""
				card.ActivatedAt = nil
			}
			changed = true
		}
		result.AddItem(cardops.BulkItemResult{
			Code:   code,
			Status: cardops.BulkItemUpdated,
		})
	}
	if !dryRun && changed {
		cm.appendAuditLocked(AuditEntry{
			Time:   time.Now(),
			Action: "bulk_" + action,
			Detail: fmt.Sprintf("updated=%d skipped=%d failed=%d cards=%s", result.Updated, result.Skipped, result.Failed, strings.Join(codes, ",")),
		})
	}
	cm.mu.Unlock()
	if !dryRun && changed {
		cm.save()
	}
	return result, nil
}

// BulkUpdateCards preserves the legacy count-only bulk operation API.
func (cm *CardManager) BulkUpdateCards(codes []string, action string, extendHours int) (int, error) {
	result, err := cm.BulkUpdateCardsDetailed(codes, action, extendHours)
	return result.Updated, err
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
func (cm *CardManager) CreateAgent(username, passwordHash, prefix string) (*Agent, error) {
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

func (cm *CardManager) UsernameExists(username string) bool {
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
func (cm *CardManager) GetAgent(id string) *Agent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cloneAgent(cm.agents[id])
}

func (cm *CardManager) UpdateAgentPassword(id, passwordHash string) error {
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
func (cm *CardManager) AllAgents() []*Agent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Agent, 0, len(cm.agents))
	for _, a := range cm.agents {
		result = append(result, cloneAgent(a))
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
func (cm *CardManager) DeleteAgent(id string) error {
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
func (cm *CardManager) AgentCards(agentID string) []*Card {
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
