package agents

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAgentNotFound   = errors.New("agent not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrAgentDisabled   = errors.New("agent is disabled")
	ErrUsernameExists  = errors.New("username already exists")
	ErrPasswordTooLong = errors.New("password too long")
)

type Clock func() time.Time

type AuditSink func(action, agentID, detail string)

type Service struct {
	repo      Repository
	clock     Clock
	auditSink AuditSink
}

func (s *Service) SetAuditSink(sink AuditSink) {
	s.auditSink = sink
}

func NewService(repo Repository, clock Clock) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{repo: repo, clock: clock}
}

// Login verifies credentials. It returns the agent and a boolean indicating if the password was migrated.
func (s *Service) Login(ctx context.Context, username, password string) (*Account, bool, error) {
	// Fetch all agents (legacy logic does this to find by username)
	all, err := s.repo.AllAgents(ctx)
	if err != nil {
		return nil, false, err
	}
	var agent *Account
	for _, a := range all {
		if a.Username == username {
			ac := a
			agent = &ac
			break
		}
	}
	if agent == nil {
		return nil, false, ErrAgentNotFound
	}
	if agent.Disabled {
		return nil, false, ErrAgentDisabled
	}

	match, newHash := verifyPassword(password, agent.Password)
	if !match {
		return nil, false, ErrInvalidPassword
	}

	migrated := false
	if newHash != "" {
		if err := s.repo.UpdateAgentPassword(ctx, agent.ID, newHash); err == nil {
			agent.Password = newHash
			migrated = true
			if s.auditSink != nil {
				s.auditSink("agent_password_updated", agent.ID, "")
			}
		}
	}

	return agent, migrated, nil
}

func (s *Service) CreateAgent(ctx context.Context, username, plainPassword, prefix string) (*Account, error) {
	exists, err := s.repo.UsernameExists(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	if len(plainPassword) > 72 {
		return nil, ErrPasswordTooLong
	}

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("password encryption failed: %w", err)
	}

	acc, err := s.repo.CreateAgent(ctx, username, string(bcryptHash), prefix)
	if err != nil {
		return nil, err
	}
	if s.auditSink != nil {
		s.auditSink("agent_created", acc.ID, fmt.Sprintf("agent_id=%s username=%s", acc.ID, username))
	}
	return &acc, nil
}

func (s *Service) ChangePassword(ctx context.Context, id, oldPassword, newPassword string) error {
	agent, err := s.repo.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	if agent == nil {
		return ErrAgentNotFound
	}

	match, _ := verifyPassword(oldPassword, agent.Password)
	if !match {
		return ErrInvalidPassword
	}

	if len(newPassword) > 72 {
		return ErrPasswordTooLong
	}

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("password encryption failed: %w", err)
	}

	err = s.repo.UpdateAgentPassword(ctx, id, string(bcryptHash))
	if err != nil {
		return err
	}
	if s.auditSink != nil {
		s.auditSink("agent_password_updated", id, "")
	}
	return nil
}

func (s *Service) SetAgentStatus(ctx context.Context, id string, disabled bool) error {
	if err := s.repo.UpdateAgentStatus(ctx, id, disabled); err != nil {
		return err
	}
	action := "agent_enabled"
	if disabled {
		action = "agent_disabled"
	}
	if s.auditSink != nil {
		s.auditSink(action, id, fmt.Sprintf("agent_id=%s", id))
	}
	return nil
}

func (s *Service) SetAgentLimits(ctx context.Context, id string, maxSessionDuration, maxDailySessions, maxConcurrentSessions int) error {
	limits := AgentLimits{
		MaxSessionDurationMinutes: maxSessionDuration,
		MaxDailySessions:          maxDailySessions,
		MaxConcurrentSessions:     maxConcurrentSessions,
	}
	if err := s.repo.UpdateAgentLimits(ctx, id, limits); err != nil {
		return err
	}
	if s.auditSink != nil {
		s.auditSink("agent_limits_updated", id, fmt.Sprintf("duration=%d daily=%d concurrent=%d", maxSessionDuration, maxDailySessions, maxConcurrentSessions))
	}
	return nil
}

func (s *Service) DeleteAgent(ctx context.Context, id string) error {
	agent, err := s.repo.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	if agent == nil {
		return ErrAgentNotFound
	}
	err = s.repo.DeleteAgent(ctx, id)
	if err != nil {
		return err
	}
	if s.auditSink != nil {
		s.auditSink("agent_deleted", id, fmt.Sprintf("agent_id=%s", id))
	}
	return nil
}

func (s *Service) GetAgent(ctx context.Context, id string) (*Account, error) {
	return s.repo.GetAgent(ctx, id)
}

func (s *Service) AllAgents(ctx context.Context) ([]Account, error) {
	return s.repo.AllAgents(ctx)
}

func (s *Service) UsernameExists(ctx context.Context, username string) (bool, error) {
	return s.repo.UsernameExists(ctx, username)
}

// verifyPassword checks a plaintext password against a stored hash (bcrypt or legacy SHA-256).
func verifyPassword(pw, storedHash string) (match bool, newHash string) {
	if strings.HasPrefix(storedHash, "$2") {
		// bcrypt hash
		err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(pw))
		return err == nil, ""
	}
	// legacy SHA-256 hash
	h := sha256.New()
	h.Write([]byte(pw))
	legacyHash := hex.EncodeToString(h.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(legacyHash), []byte(storedHash)) == 1 {
		// Calculate new bcrypt hash to return for auto-migration
		bHash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		if err == nil {
			return true, string(bHash)
		}
		return true, "" // fallback to no migration if error
	}
	return false, ""
}

func (s *Service) ForceResetPassword(ctx context.Context, id, newPassword string) error {
	agent, err := s.repo.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	if agent == nil {
		return ErrAgentNotFound
	}
	if len(newPassword) > 72 {
		return ErrPasswordTooLong
	}

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("password encryption failed: %w", err)
	}
	err = s.repo.UpdateAgentPassword(ctx, id, string(bcryptHash))
	if err != nil {
		return err
	}
	if s.auditSink != nil {
		s.auditSink("agent_password_updated", id, "")
	}
	return nil
}

func (s *Service) RegisterAgent(ctx context.Context, username, plainPassword, prefix string, inviteCode string, inviteSvc *InviteService) (*Account, error) {
	// 1. Validate username uniqueness
	exists, err := s.repo.UsernameExists(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	if len(plainPassword) > 72 {
		return nil, ErrPasswordTooLong
	}

	// 2. Start SQL transaction
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 3. Validate and use invite code in the SAME transaction
	if err := inviteSvc.ValidateAndUseTx(ctx, tx, inviteCode, username); err != nil {
		return nil, err
	}

	// 4. Create the agent in the transaction
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("password encryption failed: %w", err)
	}

	acc, err := s.repo.CreateAgentTx(ctx, tx, username, string(bcryptHash), prefix)
	if err != nil {
		return nil, err
	}

	// 5. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// 6. Record audit after commit
	if s.auditSink != nil {
		s.auditSink("agent_created", acc.ID, fmt.Sprintf("agent_id=%s username=%s", acc.ID, username))
	}

	return &acc, nil
}
