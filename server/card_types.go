package main

import (
	"sync"
	"time"

	agentsvc "github.com/lingqiao/server/internal/agents"
	auditsvc "github.com/lingqiao/server/internal/audit"
	cardops "github.com/lingqiao/server/internal/cards"
)

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
