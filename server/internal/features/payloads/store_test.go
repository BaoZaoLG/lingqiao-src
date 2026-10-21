package payloads

import (
	"errors"
	"testing"
)

type failingStore struct {
	value any
	err   error
}

func (s *failingStore) Save(_ string, value any) error {
	s.value = value
	return s.err
}

func (s *failingStore) Load(_ string, _ any) error { return nil }

func TestPayloadStoreRollsBackFailedMutations(t *testing.T) {
	backend := &failingStore{}
	store := NewStore(backend)
	payload := &PayloadInfo{PayloadID: "payload-1"}

	backend.err = errors.New("disk full")
	if err := store.Add(payload); err == nil {
		t.Fatal("Add should report persistence failure")
	}
	if got := store.Get(payload.PayloadID); got != nil {
		t.Fatalf("failed Add leaked in-memory state: %#v", got)
	}

	backend.err = nil
	if err := store.Add(payload); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	backend.err = errors.New("disk full")
	if err := store.SetActive(payload.PayloadID); err == nil {
		t.Fatal("SetActive should report persistence failure")
	}
	if got := store.Active(); got != nil {
		t.Fatalf("failed SetActive leaked active state: %#v", got)
	}
	if err := store.Delete(payload.PayloadID); err == nil {
		t.Fatal("Delete should report persistence failure")
	}
	if got := store.Get(payload.PayloadID); got == nil {
		t.Fatal("failed Delete removed in-memory payload")
	}
}
