package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Session ...
type Session struct {
	ActorID   string
	ExpiresAt time.Time
}

// SessionStore ...
type SessionStore struct {
	prefix string
	ttl    time.Duration
	mu     sync.Mutex
	items  map[string]Session
}

// NewSessionStore ...
func NewSessionStore(prefix string, ttl time.Duration) *SessionStore {
	return &SessionStore{
		prefix: prefix,
		ttl:    ttl,
		items:  make(map[string]Session),
	}
}

// Create ...
func (s *SessionStore) Create(actorID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := s.prefix + "_" + hex.EncodeToString(b)

	s.mu.Lock()
	s.items[HashToken(token)] = Session{ActorID: actorID, ExpiresAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()

	return token, nil
}

// Check ...
func (s *SessionStore) Check(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.items[HashToken(token)]
	if !ok || !time.Now().Before(session.ExpiresAt) {
		return "", false
	}
	return session.ActorID, true
}

// Invalidate ...
func (s *SessionStore) Invalidate(token string) {
	s.mu.Lock()
	delete(s.items, HashToken(token))
	s.mu.Unlock()
}

// Cleanup ...
func (s *SessionStore) Cleanup(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for tokenHash, session := range s.items {
		if !now.Before(session.ExpiresAt) {
			delete(s.items, tokenHash)
			removed++
		}
	}
	return removed
}

// RawTokenStoredForTest ...
func (s *SessionStore) RawTokenStoredForTest(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.items[token]
	return ok
}

// HashToken ...
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
