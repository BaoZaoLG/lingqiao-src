package main

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// ── Payload Storage ─────────────────────────────────────────────────────────

type PayloadInfo struct {
	PayloadID  string    `json:"payload_id"`
	AesKey     string    `json:"aes_key"`
	HmacKey    string    `json:"hmac_key"`
	IV         string    `json:"iv"`
	ExeHash    string    `json:"exe_hash"`
	ChunkCount int       `json:"chunk_count"`
	ChunkSize  int       `json:"chunk_size"`
	TotalSize  int       `json:"total_size"`
	CreatedAt  time.Time `json:"created_at"`
	Active     bool      `json:"active,omitempty"`
}

type PayloadSummary struct {
	PayloadID  string    `json:"payload_id"`
	ExeHash    string    `json:"exe_hash"`
	ChunkCount int       `json:"chunk_count"`
	ChunkSize  int       `json:"chunk_size"`
	TotalSize  int       `json:"total_size"`
	CreatedAt  time.Time `json:"created_at"`
	Active     bool      `json:"active"`
}

type PayloadStore struct {
	mu       sync.RWMutex
	payloads map[string]*PayloadInfo
	activeID string
	storage  *JSONStorage
}

func NewPayloadStore(storage *JSONStorage) *PayloadStore {
	ps := &PayloadStore{
		payloads: make(map[string]*PayloadInfo),
		storage:  storage,
	}
	ps.load()
	return ps
}

func (ps *PayloadStore) load() {
	var data struct {
		Payloads map[string]*PayloadInfo `json:"payloads"`
		ActiveID string                  `json:"active_id"`
	}
	if err := ps.storage.Load("payloads", &data); err == nil && data.Payloads != nil {
		ps.payloads = data.Payloads
		ps.activeID = data.ActiveID
	}
}

func (ps *PayloadStore) save() {
	ps.mu.RLock()
	payloads := make(map[string]*PayloadInfo, len(ps.payloads))
	for id, info := range ps.payloads {
		payloads[id] = clonePayloadInfo(info)
	}
	data := struct {
		Payloads map[string]*PayloadInfo `json:"payloads"`
		ActiveID string                  `json:"active_id"`
	}{Payloads: payloads, ActiveID: ps.activeID}
	ps.mu.RUnlock()
	if err := ps.storage.Save("payloads", &data); err != nil {
		log.Printf("[ERROR] Failed to persist payloads: %v", err)
	}
}

func (ps *PayloadStore) Add(info *PayloadInfo) {
	ps.mu.Lock()
	cp := clonePayloadInfo(info)
	cp.Active = cp.PayloadID == ps.activeID
	ps.payloads[info.PayloadID] = cp
	ps.mu.Unlock()
	ps.save()
}

func (ps *PayloadStore) Get(payloadID string) *PayloadInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	info := clonePayloadInfo(ps.payloads[payloadID])
	if info != nil {
		info.Active = info.PayloadID == ps.activeID
	}
	return info
}

func (ps *PayloadStore) List() []PayloadSummary {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	items := make([]PayloadSummary, 0, len(ps.payloads))
	for _, info := range ps.payloads {
		if info == nil {
			continue
		}
		items = append(items, PayloadSummary{
			PayloadID:  info.PayloadID,
			ExeHash:    info.ExeHash,
			ChunkCount: info.ChunkCount,
			ChunkSize:  info.ChunkSize,
			TotalSize:  info.TotalSize,
			CreatedAt:  info.CreatedAt,
			Active:     info.PayloadID == ps.activeID,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].PayloadID > items[j].PayloadID
	})
	return items
}

func (ps *PayloadStore) Active() *PayloadInfo {
	ps.mu.RLock()
	activeID := ps.activeID
	info := clonePayloadInfo(ps.payloads[activeID])
	ps.mu.RUnlock()
	if info != nil {
		info.Active = true
	}
	return info
}

func (ps *PayloadStore) SetActive(payloadID string) error {
	ps.mu.Lock()
	if _, exists := ps.payloads[payloadID]; !exists {
		ps.mu.Unlock()
		return fmt.Errorf("payload not found")
	}
	ps.activeID = payloadID
	for id, info := range ps.payloads {
		info.Active = id == payloadID
	}
	ps.mu.Unlock()
	ps.save()
	return nil
}

func (ps *PayloadStore) Delete(payloadID string) error {
	ps.mu.Lock()
	if ps.activeID == payloadID {
		ps.mu.Unlock()
		return fmt.Errorf("cannot delete active payload")
	}
	if _, exists := ps.payloads[payloadID]; !exists {
		ps.mu.Unlock()
		return fmt.Errorf("payload not found")
	}
	delete(ps.payloads, payloadID)
	ps.mu.Unlock()
	ps.save()
	return nil
}

func clonePayloadInfo(info *PayloadInfo) *PayloadInfo {
	if info == nil {
		return nil
	}
	cp := *info
	return &cp
}

// ── Payload Handler ─────────────────────────────────────────────────────────

