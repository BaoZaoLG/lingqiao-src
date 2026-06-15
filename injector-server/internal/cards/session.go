package cards

import (
	"fmt"
	"strings"
	"time"
)

type Session struct {
	Token         string
	CardCode      string
	MachineID     string
	Fingerprint   string
	ClientVersion string
	CreatedAt     time.Time
	LastSeen      time.Time
	ExpiresAt     time.Time
	RemoteAddr    string
}

type ActivationInput struct {
	Card           Card
	Token          string
	MachineID      string
	Fingerprint    string
	RemoteAddr     string
	ClientVersion  string
	ActiveSessions []Session
}

type HeartbeatInput struct {
	Session       Session
	Card          Card
	MachineID     string
	RemoteAddr    string
	ClientVersion string
}

type SessionService struct {
	now        Clock
	sessionTTL time.Duration
}

func NewSessionService(now Clock, sessionTTL time.Duration) *SessionService {
	if now == nil {
		now = time.Now
	}
	return &SessionService{now: now, sessionTTL: sessionTTL}
}

func (s *SessionService) Activate(input ActivationInput) (Card, Session, error) {
	now := s.now()
	card := input.Card

	if card.MaxSessions <= 1 && card.MachineID != "" && card.MachineID != input.MachineID {
		return Card{}, Session{}, fmt.Errorf("card already bound to another machine")
	}

	activeCount := 0
	for _, session := range input.ActiveSessions {
		if sameCardCode(session.CardCode, card.Code) && session.ExpiresAt.After(now) {
			activeCount++
		}
	}
	if maxSessions := card.MaxSessions; maxSessions > 0 && activeCount >= maxSessions {
		return Card{}, Session{}, fmt.Errorf("%w: %d/%d", ErrMaxSessionsReached, activeCount, maxSessions)
	}

	if card.MachineID == "" {
		card.MachineID = input.MachineID
	}
	if card.ActivatedAt == nil {
		activatedAt := now
		card.ActivatedAt = &activatedAt
		duration := card.ExpiresAt.Sub(card.CreatedAt)
		card.ExpiresAt = now.Add(duration)
	}

	session := Session{
		Token:         input.Token,
		CardCode:      card.Code,
		MachineID:     input.MachineID,
		Fingerprint:   input.Fingerprint,
		ClientVersion: input.ClientVersion,
		CreatedAt:     now,
		LastSeen:      now,
		ExpiresAt:     now.Add(s.sessionTTL),
		RemoteAddr:    input.RemoteAddr,
	}
	return card, session, nil
}

func sameCardCode(a, b string) bool {
	normalize := func(code string) string {
		code = strings.ToUpper(code)
		code = strings.ReplaceAll(code, "-", "")
		code = strings.ReplaceAll(code, " ", "")
		return code
	}
	return normalize(a) == normalize(b)
}

func (s *SessionService) Heartbeat(input HeartbeatInput) (Session, error) {
	now := s.now()
	session := input.Session

	if session.MachineID != input.MachineID {
		return Session{}, ErrMachineMismatch
	}
	if session.ExpiresAt.Before(now) {
		return Session{}, ErrSessionExpired
	}
	if input.Card.Status == StatusDisabled || now.After(input.Card.ExpiresAt) {
		return Session{}, ErrCardNoLongerValid
	}

	session.LastSeen = now
	session.ExpiresAt = now.Add(s.sessionTTL)
	session.RemoteAddr = input.RemoteAddr
	if input.ClientVersion != "" {
		session.ClientVersion = input.ClientVersion
	}
	return session, nil
}
