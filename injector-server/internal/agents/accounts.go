package agents

import (
	"fmt"
	"time"
)

// Account ...
type Account struct {
	ID        string
	Username  string
	Password  string
	Prefix    string
	CreatedAt time.Time
	Disabled  bool
}

// IDGenerator ...
type IDGenerator func() (string, error)

// AccountService ...
type AccountService struct {
	generateID IDGenerator
}

// NewAccountService ...
func NewAccountService(generateID IDGenerator) *AccountService {
	return &AccountService{generateID: generateID}
}

// Create ...
func (s *AccountService) Create(existing []Account, username, passwordHash, prefix string) (Account, error) {
	if username == "" {
		return Account{}, fmt.Errorf("username is required")
	}
	for _, account := range existing {
		if account.Username == username {
			return Account{}, fmt.Errorf("username already exists")
		}
	}
	id, err := s.generateID()
	if err != nil {
		return Account{}, err
	}
	return Account{
		ID:        "AGT-" + id,
		Username:  username,
		Password:  passwordHash,
		Prefix:    prefix,
		CreatedAt: time.Now(),
	}, nil
}

// UpdatePassword ...
func (s *AccountService) UpdatePassword(account Account, passwordHash string) (Account, error) {
	if account.ID == "" {
		return Account{}, fmt.Errorf("agent not found")
	}
	account.Password = passwordHash
	return account, nil
}

// UpdateStatus ...
func (s *AccountService) UpdateStatus(account Account, disabled bool) Account {
	account.Disabled = disabled
	return account
}

// EnsureCanDelete ...
func (s *AccountService) EnsureCanDelete(account *Account) error {
	if account == nil || account.ID == "" {
		return fmt.Errorf("agent not found")
	}
	return nil
}
