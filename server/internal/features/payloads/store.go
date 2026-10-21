package payloads

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/lingqiao/server/internal/storage"
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
	storage  storage.Store
}

func NewStore(storage storage.Store) *PayloadStore {
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

func (ps *PayloadStore) snapshotLocked() struct {
	Payloads map[string]*PayloadInfo `json:"payloads"`
	ActiveID string                  `json:"active_id"`
} {
	payloads := make(map[string]*PayloadInfo, len(ps.payloads))
	for id, info := range ps.payloads {
		payloads[id] = clonePayloadInfo(info)
	}
	return struct {
		Payloads map[string]*PayloadInfo `json:"payloads"`
		ActiveID string                  `json:"active_id"`
	}{Payloads: payloads, ActiveID: ps.activeID}
}

func (ps *PayloadStore) Add(info *PayloadInfo) error {
	if info == nil || info.PayloadID == "" {
		return fmt.Errorf("invalid payload")
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	previous, existed := ps.payloads[info.PayloadID]
	cp := clonePayloadInfo(info)
	cp.Active = cp.PayloadID == ps.activeID
	ps.payloads[info.PayloadID] = cp
	if err := ps.storage.Save("payloads", ps.snapshotLocked()); err != nil {
		if existed {
			ps.payloads[info.PayloadID] = previous
		} else {
			delete(ps.payloads, info.PayloadID)
		}
		return fmt.Errorf("persist payload: %w", err)
	}
	return nil
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
	defer ps.mu.Unlock()
	if _, exists := ps.payloads[payloadID]; !exists {
		return fmt.Errorf("payload not found")
	}
	previousActiveID := ps.activeID
	ps.activeID = payloadID
	for id, info := range ps.payloads {
		info.Active = id == payloadID
	}
	if err := ps.storage.Save("payloads", ps.snapshotLocked()); err != nil {
		ps.activeID = previousActiveID
		for id, info := range ps.payloads {
			info.Active = id == previousActiveID
		}
		return fmt.Errorf("persist active payload: %w", err)
	}
	return nil
}

func (ps *PayloadStore) Delete(payloadID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.activeID == payloadID {
		return fmt.Errorf("cannot delete active payload")
	}
	previous, exists := ps.payloads[payloadID]
	if !exists {
		return fmt.Errorf("payload not found")
	}
	delete(ps.payloads, payloadID)
	if err := ps.storage.Save("payloads", ps.snapshotLocked()); err != nil {
		ps.payloads[payloadID] = previous
		return fmt.Errorf("persist payload deletion: %w", err)
	}
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
