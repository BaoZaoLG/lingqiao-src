package blacklist

import (
	"context"
	"time"
)

// Entry represents a blacklist entry.
type Entry struct {
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository defines the storage interface for the blacklist.
type Repository interface {
	Add(ctx context.Context, entry Entry) error
	Remove(ctx context.Context, value string) error
	List(ctx context.Context) ([]Entry, error)
	IsBlacklisted(ctx context.Context, value string) (bool, error)
}

// Service provides business logic for blacklisting.
type Service struct {
	repo Repository
}

// NewService creates a new blacklist service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Add adds an entry to the blacklist.
func (s *Service) Add(ctx context.Context, typ, value, reason string) error {
	return s.repo.Add(ctx, Entry{
		Type:      typ,
		Value:     value,
		Reason:    reason,
		CreatedAt: time.Now(),
	})
}

// Remove removes an entry from the blacklist.
func (s *Service) Remove(ctx context.Context, value string) error {
	return s.repo.Remove(ctx, value)
}

// List returns all blacklist entries.
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	return s.repo.List(ctx)
}

// IsBlacklisted checks if a value is blacklisted.
func (s *Service) IsBlacklisted(ctx context.Context, value string) (bool, error) {
	return s.repo.IsBlacklisted(ctx, value)
}
