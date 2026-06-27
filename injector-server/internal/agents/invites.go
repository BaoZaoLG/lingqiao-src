package agents

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lingqiao/server/internal/storage"
)

// InviteCode ...
type InviteCode struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
	UsedBy    string    `json:"used_by,omitempty"`
	UsedAt    time.Time `json:"used_at,omitempty"`
	MaxUses   int       `json:"max_uses"`
	UseCount  int       `json:"use_count"`
}

// InviteService ...
type InviteService struct {
	store storage.Store
	mu    sync.Mutex
}

// NewInviteService ...
func NewInviteService(store storage.Store) *InviteService {
	return &InviteService{store: store}
}

// List ...
func (s *InviteService) List() ([]InviteCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// Replace ...
func (s *InviteService) Replace(codes []InviteCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(codes)
}

// Create ...
func (s *InviteService) Create(maxUses int, createdBy string) (InviteCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	code, err := generateInviteCode()
	if err != nil {
		return InviteCode{}, err
	}
	invite := InviteCode{
		Code:      code,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		MaxUses:   maxUses,
	}
	codes, err := s.loadLocked()
	if err != nil {
		return InviteCode{}, err
	}
	codes = append(codes, invite)
	if err := s.saveLocked(codes); err != nil {
		return InviteCode{}, err
	}
	return invite, nil
}

// Delete ...
func (s *InviteService) Delete(code string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	codes, err := s.loadLocked()
	if err != nil {
		return false, err
	}
	filtered := make([]InviteCode, 0, len(codes))
	found := false
	for _, invite := range codes {
		if invite.Code == code {
			found = true
			continue
		}
		filtered = append(filtered, invite)
	}
	if found {
		if err := s.saveLocked(filtered); err != nil {
			return false, err
		}
	}
	return found, nil
}

// ValidateAndUse ...
func (s *InviteService) ValidateAndUse(code string, usedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if code == "" {
		return fmt.Errorf("邀请码不能为空")
	}
	codes, err := s.loadLocked()
	if err != nil {
		return err
	}
	for i, invite := range codes {
		if invite.Code != code {
			continue
		}
		if invite.MaxUses > 0 && invite.UseCount >= invite.MaxUses {
			return fmt.Errorf("邀请码已用完")
		}
		codes[i].UseCount++
		if codes[i].UsedBy == "" {
			codes[i].UsedBy = usedBy
		}
		codes[i].UsedAt = time.Now()
		return s.saveLocked(codes)
	}
	return fmt.Errorf("邀请码无效")
}

func (s *InviteService) loadLocked() ([]InviteCode, error) {
	var codes []InviteCode
	if err := s.store.Load("invites", &codes); err != nil {
		return nil, err
	}
	if codes == nil {
		codes = []InviteCode{}
	}
	return codes, nil
}

func (s *InviteService) saveLocked(codes []InviteCode) error {
	return s.store.Save("invites", codes)
}

func generateInviteCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "LQ-" + strings.ToUpper(hex.EncodeToString(b)), nil
}
