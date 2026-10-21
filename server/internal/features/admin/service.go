package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordBytes = 6
	MaxPasswordBytes = 72
)

var (
	ErrInvalidAdminRole = errors.New("invalid admin role")
	ErrLastActiveOwner  = errors.New("cannot disable or demote the last active owner")
	ErrWrongOldPassword = errors.New("旧密码错误")
)

type Service struct {
	repo       *RepositorySQLite
	sessionTTL time.Duration
}

func NewService(repo *RepositorySQLite, sessionTTL time.Duration, dataDir string) (*Service, error) {
	if sessionTTL <= 0 {
		sessionTTL = 4 * time.Hour
	}
	s := &Service{
		repo:       repo,
		sessionTTL: sessionTTL,
	}

	// Seed owner admin if none exists
	count, err := repo.CountAdmins(context.Background())
	if err == nil && count == 0 {
		var passHash string
		hashFile := filepath.Join(dataDir, "admin_password.hash")
		if savedHash, err := os.ReadFile(hashFile); err == nil {
			trimmed := strings.TrimSpace(string(savedHash))
			if s.isBcryptHash(trimmed) || len(trimmed) == 64 {
				passHash = trimmed
			}
		}

		if passHash == "" {
			pass := os.Getenv("ADMIN_PASSWORD")
			if pass == "" {
				b := make([]byte, 16)
				if _, err := rand.Read(b); err != nil {
					return nil, fmt.Errorf("failed to generate owner admin password: %w", err)
				}
				pass = hex.EncodeToString(b)
				log.Printf("[ADMIN] Generated random password: %s****", pass[:4])
			}
			passHash, err = s.hashPassword(pass)
			if err != nil {
				return nil, err
			}
		}

		owner := &AdminUser{
			ID:           "adm_owner",
			Username:     DefaultOwnerUsername,
			PasswordHash: passHash,
			Role:         RoleOwner,
			Disabled:     false,
			CreatedAt:    time.Now().UTC(),
		}
		if err := repo.CreateAdmin(context.Background(), owner); err != nil {
			log.Printf("[ADMIN] Failed to seed default owner admin: %v", err)
		}
	}

	return s, nil
}

func (s *Service) CreateSession(ctx context.Context, adminID string, remoteAddr, userAgent string) (string, int, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("[ADMIN] Failed to generate session token: %v", err)
		return "", 0, err
	}
	token := "admin_" + hex.EncodeToString(b)

	now := time.Now().UTC()
	session := &AdminSession{
		TokenHash:  s.hashToken(token),
		AdminID:    adminID,
		CreatedAt:  now,
		ExpiresAt:  now.Add(s.sessionTTL),
		RemoteAddr: remoteAddr,
		UserAgent:  userAgent,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return "", 0, err
	}
	return token, int(s.sessionTTL.Seconds()), nil
}

func (s *Service) CheckSession(ctx context.Context, token string) (*AdminUser, bool) {
	if token == "" {
		return nil, false
	}
	session, err := s.repo.GetSession(ctx, s.hashToken(token))
	if err != nil || time.Now().UTC().After(session.ExpiresAt) {
		return nil, false
	}

	admin, err := s.repo.GetAdmin(ctx, session.AdminID)
	if err != nil || admin.Disabled {
		return nil, false
	}

	return admin, true
}

func (s *Service) VerifyLogin(ctx context.Context, username, password string) (*AdminUser, error) {
	admin, err := s.repo.GetAdminByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("wrong username or password")
	}

	if admin.Disabled {
		return nil, errors.New("account disabled")
	}

	match, newHash := s.verifyPassword(password, admin.PasswordHash)
	if !match {
		return nil, errors.New("wrong username or password")
	}

	if newHash != "" {
		admin.PasswordHash = newHash
		s.repo.UpdateAdmin(ctx, admin)
		log.Printf("[ADMIN] Password hash migrated to bcrypt for user: %s", admin.Username)
	}

	now := time.Now().UTC()
	admin.LastLoginAt = &now
	s.repo.UpdateAdmin(ctx, admin)

	return admin, nil
}

func (s *Service) ChangePassword(ctx context.Context, adminID, oldPassword, newPassword string) error {
	if len(newPassword) < MinPasswordBytes || len(newPassword) > MaxPasswordBytes {
		return errors.New("password length invalid")
	}

	admin, err := s.repo.GetAdmin(ctx, adminID)
	if err != nil {
		return err
	}

	match, _ := s.verifyPassword(oldPassword, admin.PasswordHash)
	if !match {
		return ErrWrongOldPassword
	}

	passwordHash, err := s.hashPassword(newPassword)
	if err != nil {
		return err
	}
	admin.PasswordHash = passwordHash
	return s.repo.UpdateAdmin(ctx, admin)
}

func (s *Service) hashPassword(pw string) (string, error) {
	if len(pw) < MinPasswordBytes || len(pw) > MaxPasswordBytes {
		return "", errors.New("password length invalid")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("password hashing failed: %w", err)
	}
	return string(hash), nil
}

func (s *Service) isBcryptHash(h string) bool {
	return len(h) >= 4 && h[0] == '$' && h[1] == '2'
}

func (s *Service) verifyPassword(pw, storedHash string) (match bool, newHash string) {
	if s.isBcryptHash(storedHash) {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(pw)) == nil, ""
	}
	h := sha256.Sum256([]byte(pw))
	legacyHash := hex.EncodeToString(h[:])
	if subtle.ConstantTimeCompare([]byte(legacyHash), []byte(storedHash)) != 1 {
		return false, ""
	}
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return true, ""
	}
	return true, string(bcryptHash)
}

func (s *Service) hashToken(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:])
}

func (s *Service) GetAdmin(ctx context.Context, id string) (*AdminUser, error) {
	return s.repo.GetAdmin(ctx, id)
}

func (s *Service) ListAdmins(ctx context.Context) ([]AdminUser, error) {
	return s.repo.ListAdmins(ctx)
}

func (s *Service) CreateAdmin(ctx context.Context, admin *AdminUser) error {
	if admin == nil {
		return errors.New("admin is required")
	}
	if !IsValidRole(admin.Role) {
		return ErrInvalidAdminRole
	}
	passwordHash, err := s.hashPassword(admin.PasswordHash)
	if err != nil {
		return err
	}
	admin.PasswordHash = passwordHash
	return s.repo.CreateAdmin(ctx, admin)
}

func (s *Service) UpdateAdmin(ctx context.Context, admin *AdminUser) error {
	if admin == nil {
		return errors.New("admin is required")
	}
	if !IsValidRole(admin.Role) {
		return ErrInvalidAdminRole
	}

	current, err := s.repo.GetAdmin(ctx, admin.ID)
	if err != nil {
		return err
	}
	if current.Role == RoleOwner && !current.Disabled &&
		(admin.Role != RoleOwner || admin.Disabled) {
		admins, err := s.repo.ListAdmins(ctx)
		if err != nil {
			return err
		}
		activeOwners := 0
		for _, candidate := range admins {
			if candidate.Role == RoleOwner && !candidate.Disabled {
				activeOwners++
			}
		}
		if activeOwners <= 1 {
			return ErrLastActiveOwner
		}
	}

	return s.repo.UpdateAdmin(ctx, admin)
}

func (s *Service) HashPassword(pw string) (string, error) {
	return s.hashPassword(pw)
}
