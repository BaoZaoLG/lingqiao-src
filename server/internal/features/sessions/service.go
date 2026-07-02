package sessions

import (
	"context"
	"errors"
	"fmt"
	"time"

	cardops "github.com/lingqiao/server/internal/cards"
	"github.com/lingqiao/server/internal/platform/cardcode"
)

type Clock func() time.Time
type TokenGenerator func() string
type AuditSink func(action, cardCode, machineID, remoteAddr string)

type Service struct {
	store         Store
	now           Clock
	sessionTTL    time.Duration
	generateToken TokenGenerator
	auditSink     AuditSink
}

func NewService(store Store, sessionTTL time.Duration, generateToken TokenGenerator, now Clock) *Service {
	if now == nil {
		now = time.Now
	}
	if sessionTTL <= 0 {
		sessionTTL = 30 * time.Minute
	}
	if generateToken == nil {
		generateToken = GenerateToken
	}
	return &Service{store: store, now: now, sessionTTL: sessionTTL, generateToken: generateToken}
}

func (s *Service) SetAuditSink(sink AuditSink) {
	s.auditSink = sink
}

func (s *Service) Activate(ctx context.Context, input ActivationInput) (Session, CardSnapshot, error) {
	var session Session
	var card CardSnapshot

	err := s.store.RunInTx(ctx, func(tx Tx) error {
		c, err := tx.GetCard(ctx, cardcode.Normalize(input.CardCode))
		if err != nil {
			return err
		}
		if c.Status == "disabled" || c.Status == "disable" {
			return ErrCardDisabled
		}
		if c.Status == "expired" || c.ExpiresAt.Before(s.now()) {
			return ErrCardExpired
		}
		blacklisted, err := tx.IsCardBlacklisted(ctx, c.Code)
		if err != nil {
			return err
		}
		if blacklisted {
			return ErrCardBlacklisted
		}

		activeSessions, err := tx.ListSessionsByCard(ctx, c.Code)
		if err != nil {
			return err
		}

		lifecycle := cardops.NewSessionService(cardops.Clock(s.now), s.sessionTTL)

		cardIn := cardops.Card{
			Code:        c.Code,
			MachineID:   c.MachineID,
			ActivatedAt: c.ActivatedAt,
			CreatedAt:   c.CreatedAt,
			ExpiresAt:   c.ExpiresAt,
			MaxSessions: c.MaxSessions,
		}

		var activeSessIn []cardops.Session
		for _, as := range activeSessions {
			activeSessIn = append(activeSessIn, cardops.Session{
				CardCode:  as.CardCode,
				ExpiresAt: as.ExpiresAt,
			})
		}

		token := s.generateToken()
		actIn := cardops.ActivationInput{
			Card:           cardIn,
			Token:          token,
			MachineID:      input.MachineID,
			Fingerprint:    input.Fingerprint,
			RemoteAddr:     input.RemoteAddr,
			ClientVersion:  input.ClientVersion,
			ActiveSessions: activeSessIn,
		}

		cardOut, sessOut, err := lifecycle.Activate(actIn)
		if err != nil {
			if errors.Is(err, cardops.ErrMaxSessionsReached) {
				return fmt.Errorf("%w: %v", ErrMaxSessionsReached, err)
			}
			if errors.Is(err, cardops.ErrCardBoundToOtherMachine) {
				return fmt.Errorf("%w: %v", ErrBoundToOtherMachine, err)
			}
			return err
		}

		c.MachineID = cardOut.MachineID
		c.ActivatedAt = cardOut.ActivatedAt
		c.ExpiresAt = cardOut.ExpiresAt

		if err := tx.UpdateCardActivation(ctx, c); err != nil {
			return err
		}

		session = Session{
			Token:         sessOut.Token,
			CardCode:      sessOut.CardCode,
			MachineID:     sessOut.MachineID,
			Fingerprint:   sessOut.Fingerprint,
			ClientVersion: sessOut.ClientVersion,
			CreatedAt:     sessOut.CreatedAt,
			LastSeen:      sessOut.LastSeen,
			ExpiresAt:     sessOut.ExpiresAt,
			RemoteAddr:    sessOut.RemoteAddr,
		}

		if err := tx.InsertSession(ctx, session); err != nil {
			return err
		}
		card = c
		return nil
	})

	if err == nil && s.auditSink != nil {
		s.auditSink("card_activated", card.Code, session.MachineID, session.RemoteAddr)
	}
	return session, card, err
}

func (s *Service) Heartbeat(ctx context.Context, input HeartbeatInput) (Session, CardSnapshot, error) {
	var session Session
	var card CardSnapshot

	err := s.store.RunInTx(ctx, func(tx Tx) error {
		sess, err := tx.GetSession(ctx, input.SessionToken)
		if err != nil {
			return err
		}
		if input.MachineID != "" && sess.MachineID != input.MachineID {
			return ErrMachineMismatch
		}
		if sess.ExpiresAt.Before(s.now()) {
			tx.DeleteSession(ctx, input.SessionToken)
			return ErrSessionExpired
		}

		c, err := tx.GetCard(ctx, sess.CardCode)
		if err != nil || c.Status == "disabled" || c.Status == "disable" || c.Status == "expired" || c.ExpiresAt.Before(s.now()) {
			tx.DeleteSession(ctx, input.SessionToken)
			return ErrCardNoLongerValid
		}

		sess.LastSeen = s.now()
		sess.ExpiresAt = s.now().Add(s.sessionTTL)
		sess.RemoteAddr = input.RemoteAddr
		if input.ClientVersion != "" {
			sess.ClientVersion = input.ClientVersion
		}

		if err := tx.UpdateSession(ctx, sess); err != nil {
			return err
		}
		session = sess
		card = c
		return nil
	})
	return session, card, err
}

func (s *Service) Deactivate(ctx context.Context, input DeactivateInput) error {
	var sess Session
	err := s.store.RunInTx(ctx, func(tx Tx) error {
		var err error
		sess, err = tx.GetSession(ctx, input.SessionToken)
		if err != nil {
			return err
		}
		if !input.IsAdminForce {
			if input.MachineID != "" && sess.MachineID != input.MachineID {
				return ErrMachineMismatch
			}
			if input.CardCode != "" && sess.CardCode != input.CardCode {
				return ErrCardMismatch
			}
		}

		if input.Unbind {
			if err := tx.UnbindCardPreservingRemaining(ctx, sess.CardCode, s.now()); err != nil {
				return err
			}
		}

		return tx.DeleteSession(ctx, input.SessionToken)
	})
	if err == nil && s.auditSink != nil {
		s.auditSink("card_deactivated", sess.CardCode, sess.MachineID, sess.RemoteAddr)
	}
	return err
}

func (s *Service) ValidateActiveToken(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrTokenRequired
	}
	sess, err := s.store.GetSession(ctx, token)
	if err != nil {
		return Session{}, err
	}
	if sess.ExpiresAt.Before(s.now()) {
		return Session{}, ErrSessionExpired
	}
	return sess, nil
}

func (s *Service) ValidateBound(ctx context.Context, token, machineID, cardCode string) (Session, error) {
	if token == "" {
		return Session{}, ErrTokenRequired
	}
	if machineID == "" {
		return Session{}, ErrMachineRequired
	}

	var session Session
	err := s.store.RunInTx(ctx, func(tx Tx) error {
		sess, err := tx.GetSession(ctx, token)
		if err != nil {
			return err
		}
		if sess.ExpiresAt.Before(s.now()) {
			return ErrSessionExpired
		}
		if sess.MachineID != machineID {
			return ErrMachineMismatch
		}
		if cardCode != "" && !cardcode.Same(sess.CardCode, cardCode) {
			return ErrCardMismatch
		}

		c, err := tx.GetCard(ctx, sess.CardCode)
		if err != nil {
			return err
		}
		if c.Status == "disabled" || c.Status == "disable" {
			return ErrCardDisabled
		}
		if c.Status == "expired" || c.ExpiresAt.Before(s.now()) {
			return ErrCardExpired
		}
		session = sess
		return nil
	})
	return session, err
}

func (s *Service) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 10
	}

	sessions, total, err := s.store.ListSessions(ctx, filter, s.now())
	if err != nil {
		return ListResult{}, err
	}

	// Basic pagination for memory store testing, though actual pagination logic belongs in SQLite repository.
	// We'll leave the array slicing to the repository, but for test coverage we just return the count and sessions.
	// Note: in memoryStore mock ListSessions we just returned all matching sessions.

	return ListResult{
		Items:   sessions,
		Total:   total,
		Page:    filter.Page,
		PerPage: filter.PerPage,
	}, nil
}

func (s *Service) CleanupExpired(ctx context.Context) (int, error) {
	return s.store.DeleteExpired(ctx, s.now())
}
