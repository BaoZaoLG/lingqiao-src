package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo       *RepositorySQLite
	sessionTTL time.Duration
}

func NewService(repo *RepositorySQLite, sessionTTL time.Duration, dataDir string) *Service {
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
				rand.Read(b)
				pass = hex.EncodeToString(b)
				log.Printf("[ADMIN] Generated random password: %s****", pass[:4])
			}
			passHash = s.hashPassword(pass)
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

	return s
}

func (s *Service) CreateSession(ctx context.Context, adminID string, remoteAddr, userAgent string) (string, int) {
	b := make([]byte, 32)
	rand.Read(b)
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
	_ = s.repo.CreateSession(ctx, session)
	return token, int(s.sessionTTL.Seconds())
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
	if len(newPassword) < 6 || len(newPassword) > 128 {
		return errors.New("password length invalid")
	}

	admin, err := s.repo.GetAdmin(ctx, adminID)
	if err != nil {
		return err
	}

	match, _ := s.verifyPassword(oldPassword, admin.PasswordHash)
	if !match {
		return errors.New("旧密码错误")
	}

	admin.PasswordHash = s.hashPassword(newPassword)
	return s.repo.UpdateAdmin(ctx, admin)
}

func (s *Service) hashPassword(pw string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		h := sha256.Sum256([]byte(pw))
		return hex.EncodeToString(h[:])
	}
	return string(hash)
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
	admin.PasswordHash = s.hashPassword(admin.PasswordHash)
	return s.repo.CreateAdmin(ctx, admin)
}

func (s *Service) UpdateAdmin(ctx context.Context, admin *AdminUser) error {
	return s.repo.UpdateAdmin(ctx, admin)
}

func (s *Service) HashPassword(pw string) string {
	return s.hashPassword(pw)
}
