package auth

import (
	"strings"
	"testing"
	"time"
)

func TestSessionStoreCreatesAndChecksSession(t *testing.T) {
	store := NewSessionStore("admin", time.Hour)

	token, err := store.Create("actor-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !strings.HasPrefix(token, "admin_") {
		t.Fatalf("token = %q, want admin_ prefix", token)
	}
	actor, ok := store.Check(token)
	if !ok {
		t.Fatal("Check should accept fresh token")
	}
	if actor != "actor-1" {
		t.Fatalf("actor = %q, want actor-1", actor)
	}
	if store.RawTokenStoredForTest(token) {
		t.Fatal("session store must not store raw token")
	}
}

func TestSessionStoreRejectsExpiredSession(t *testing.T) {
	store := NewSessionStore("agent", time.Nanosecond)

	token, err := store.Create("agent-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	time.Sleep(time.Millisecond)

	if _, ok := store.Check(token); ok {
		t.Fatal("Check should reject expired token")
	}
}

func TestSessionStoreInvalidate(t *testing.T) {
	store := NewSessionStore("agent", time.Hour)

	token, err := store.Create("agent-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	store.Invalidate(token)

	if _, ok := store.Check(token); ok {
		t.Fatal("Check should reject invalidated token")
	}
}
