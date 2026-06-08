package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

type PayloadHandler struct {
	store     *PayloadStore
	uploadKey string
	audit     func(AuditEntry)
}

func NewPayloadHandler(store *PayloadStore) *PayloadHandler {
	key := os.Getenv("UPLOAD_KEY")
	if key == "" {
		// Try to load persisted key, or generate a new one
		if data, err := os.ReadFile(dataPath("upload_key")); err == nil {
			key = strings.TrimSpace(string(data))
		}
		if key == "" {
			b := make([]byte, 24)
			rand.Read(b)
			key = hex.EncodeToString(b)
			os.MkdirAll(dataDir(), 0755)
			os.WriteFile(dataPath("upload_key"), []byte(key), 0600)
			log.Printf("[PAYLOAD] Generated upload key and saved to %s", dataPath("upload_key"))
		}
	}
	return &PayloadHandler{store: store, uploadKey: key}
}

func (h *PayloadHandler) SetAuditSink(audit func(AuditEntry)) {
	h.audit = audit
}

// HandleUpload receives encrypted payload chunks from the pack script.
func (h *PayloadHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uploadKey := r.Header.Get("X-Upload-Key")
	if uploadKey == "" {
		uploadKey = r.URL.Query().Get("key")
	}
	if uploadKey != h.uploadKey {
		writeError(w, http.StatusUnauthorized, "invalid upload key")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 100<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var req struct {
		PayloadID  string   `json:"payload_id"`
		AesKey     string   `json:"aes_key"`
		HmacKey    string   `json:"hmac_key"`
		IV         string   `json:"iv"`
		ExeHash    string   `json:"exe_hash"`
		ChunkCount int      `json:"chunk_count"`
		ChunkSize  int      `json:"chunk_size"`
		TotalSize  int      `json:"total_size"`
		Chunks     []string `json:"chunks"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.PayloadID == "" || req.AesKey == "" || len(req.Chunks) == 0 {
		writeError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	// Sanitize payload ID — reject path traversal
	if strings.ContainsAny(req.PayloadID, "/\\..") || len(req.PayloadID) > 64 {
		writeError(w, http.StatusBadRequest, "invalid payload_id")
		return
	}

	if _, _, _, err := decodePayloadKeyMaterial(req.AesKey, req.HmacKey, req.IV); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := decodeFixedHex("exe_hash", req.ExeHash, 32); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ChunkCount != len(req.Chunks) {
		writeError(w, http.StatusBadRequest, "chunk_count does not match chunks length")
		return
	}
	if req.ChunkSize <= 0 {
		writeError(w, http.StatusBadRequest, "chunk_size must be positive")
		return
	}
	if req.TotalSize <= 0 {
		writeError(w, http.StatusBadRequest, "total_size must be positive")
		return
	}

	decodedChunks := make([][]byte, len(req.Chunks))
	actualTotal := 0
	for i, chunkB64 := range req.Chunks {
		chunkData, err := base64.StdEncoding.DecodeString(chunkB64)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid chunk %d: %v", i, err))
			return
		}
		if len(chunkData) == 0 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("chunk %d is empty", i))
			return
		}
		if len(chunkData) > req.ChunkSize {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("chunk %d exceeds chunk_size", i))
			return
		}
		if i < len(req.Chunks)-1 && len(chunkData) != req.ChunkSize {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("chunk %d size does not match chunk_size", i))
			return
		}
		decodedChunks[i] = chunkData
		actualTotal += len(chunkData)
	}
	if actualTotal != req.TotalSize {
		writeError(w, http.StatusBadRequest, "total_size does not match decoded chunks")
		return
	}

	chunkDir := dataPath("payloads", req.PayloadID)
	os.MkdirAll(chunkDir, 0755)

	for i, chunkData := range decodedChunks {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%04d.bin", i))
		if err := os.WriteFile(chunkPath, chunkData, 0600); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save chunk")
			return
		}
	}

	info := &PayloadInfo{
		PayloadID:  req.PayloadID,
		AesKey:     req.AesKey,
		HmacKey:    req.HmacKey,
		IV:         req.IV,
		ExeHash:    req.ExeHash,
		ChunkCount: len(decodedChunks),
		ChunkSize:  req.ChunkSize,
		TotalSize:  actualTotal,
		CreatedAt:  time.Now(),
	}
	h.store.Add(info)
	h.recordPayloadAudit("payload_uploaded", info, "")

	log.Printf("[PAYLOAD] Uploaded: id=%s chunks=%d size=%d", req.PayloadID, len(decodedChunks), actualTotal)
	writeJSON(w, map[string]interface{}{
		"status":      "ok",
		"payload_id":  req.PayloadID,
		"chunk_count": len(decodedChunks),
	})
}

func (h *PayloadHandler) HandleAdminList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	payloads := h.store.List()
	var latest *PayloadSummary
	if len(payloads) > 0 {
		latest = &payloads[0]
	}
	active := payloadSummaryFromInfo(h.store.Active())
	writeOK(w, map[string]interface{}{
		"payloads": payloads,
		"latest":   latest,
		"active":   active,
		"active_id": func() string {
			if active == nil {
				return ""
			}
			return active.PayloadID
		}(),
		"total": len(payloads),
	})
}

func (h *PayloadHandler) HandleAdminManage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Action    string `json:"action"`
		PayloadID string `json:"payload_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch req.Action {
	case "activate":
		if err := h.store.SetActive(req.PayloadID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		info := h.store.Get(req.PayloadID)
		h.recordPayloadAudit("payload_activated", info, "")
		writeOK(w, map[string]interface{}{"payload": payloadSummaryFromInfo(info)})
	case "delete":
		info := h.store.Get(req.PayloadID)
		if err := h.store.Delete(req.PayloadID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		_ = os.RemoveAll(dataPath("payloads", req.PayloadID))
		h.recordPayloadAudit("payload_deleted", info, "")
		writeOK(w, nil)
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
	}
}

// HandlePayloadInfo returns metadata about a payload.
func (h *PayloadHandler) HandlePayloadInfo(w http.ResponseWriter, r *http.Request) {
	payloadID := extractPathParam(r.URL.Path, "/api/v1/payload/", "/info")
	if payloadID == "" {
		writeError(w, http.StatusBadRequest, "missing payload_id")
		return
	}

	info := h.store.Get(payloadID)
	if info == nil {
		writeError(w, http.StatusNotFound, "payload not found")
		return
	}

	writeJSON(w, map[string]interface{}{
		"status":      "ok",
		"payload_id":  info.PayloadID,
		"chunk_count": info.ChunkCount,
		"chunk_size":  info.ChunkSize,
		"total_size":  info.TotalSize,
		"exe_hash":    info.ExeHash,
		"created_at":  info.CreatedAt,
	})
}

func (h *PayloadHandler) recordPayloadAudit(action string, info *PayloadInfo, extra string) {
	if h == nil || h.audit == nil || info == nil {
		return
	}
	detail := fmt.Sprintf("id=%s sha256=%s chunks=%d size=%d", info.PayloadID, info.ExeHash, info.ChunkCount, info.TotalSize)
	if extra != "" {
		detail += " " + extra
	}
	h.audit(AuditEntry{
		Action: action,
		Detail: detail,
	})
}

func payloadSummaryFromInfo(info *PayloadInfo) *PayloadSummary {
	if info == nil {
		return nil
	}
	return &PayloadSummary{
		PayloadID:  info.PayloadID,
		ExeHash:    info.ExeHash,
		ChunkCount: info.ChunkCount,
		ChunkSize:  info.ChunkSize,
		TotalSize:  info.TotalSize,
		CreatedAt:  info.CreatedAt,
		Active:     info.Active,
	}
}

// HandleChunkDownload serves a single chunk.
func (h *PayloadHandler) HandleChunkDownload(w http.ResponseWriter, r *http.Request) {
	parts := splitPayloadPath(r.URL.Path)
	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	payloadID := parts[0]
	chunkIdxStr := parts[1]

	// Sanitize payload ID — reject path traversal
	if strings.ContainsAny(payloadID, "/\\..") || len(payloadID) > 64 {
		writeError(w, http.StatusBadRequest, "invalid payload_id")
		return
	}

	chunkIdx, err := strconv.Atoi(chunkIdxStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chunk index")
		return
	}

	info := h.store.Get(payloadID)
	if info == nil {
		writeError(w, http.StatusNotFound, "payload not found")
		return
	}

	if chunkIdx < 0 || chunkIdx >= info.ChunkCount {
		writeError(w, http.StatusBadRequest, "chunk index out of range")
		return
	}

	chunkPath := dataPath("payloads", payloadID, fmt.Sprintf("chunk_%04d.bin", chunkIdx))
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "chunk file not found")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// HandleKeyExchange: client sends session_token + machine_id + payload_id,
// server returns the AES key encrypted with KEK derived from card_code + HWID.
func (h *PayloadHandler) HandleKeyExchange(w http.ResponseWriter, r *http.Request, cm *CardManager) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	sig := r.Header.Get("X-HMAC-Signature")
	clientID := r.Header.Get("X-Client-ID")
	timestamp := r.Header.Get("X-Timestamp")
	nonce := r.Header.Get("X-Nonce")
	if sig == "" || clientID == "" || timestamp == "" || nonce == "" {
		writeError(w, http.StatusUnauthorized, "missing auth headers")
		return
	}
	if err := VerifyHMAC(clientID, string(body), sig, timestamp, nonce); err != nil {
		writeError(w, http.StatusUnauthorized, "HMAC verification failed")
		return
	}

	var req struct {
		SessionToken string `json:"session_token"`
		MachineID    string `json:"machine_id"`
		PayloadID    string `json:"payload_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Verify session
	cm.mu.RLock()
	session, exists := cm.sessions[req.SessionToken]
	cm.mu.RUnlock()
	if !exists || session.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusForbidden, "invalid or expired session")
		return
	}
	if session.MachineID != req.MachineID {
		writeError(w, http.StatusForbidden, "machine mismatch")
		return
	}

	info := h.store.Get(req.PayloadID)
	if info == nil {
		writeError(w, http.StatusNotFound, "payload not found")
		return
	}

	// Derive KEK from card_code + HWID
	normalizedCard := normalizeCardCode(session.CardCode)
	kekMaterial := normalizedCard + req.MachineID
	salt := []byte("payload_key_" + req.PayloadID)
	kek := pbkdf2DeriveKey([]byte(kekMaterial), salt, 100000, 32)

	// Combine key material: aesKey(32) + hmacKey(32) + iv(16) = 80 bytes
	aesKeyBytes, hmacKeyBytes, ivBytes, err := decodePayloadKeyMaterial(info.AesKey, info.HmacKey, info.IV)
	if err != nil {
		log.Printf("[KEY-EXCHANGE] Invalid payload key material: payload=%s error=%v", req.PayloadID, err)
		writeError(w, http.StatusInternalServerError, "invalid payload key material")
		return
	}

	combined := make([]byte, 0, 80)
	combined = append(combined, aesKeyBytes...)
	combined = append(combined, hmacKeyBytes...)
	combined = append(combined, ivBytes...)

	// Encrypt combined key material with KEK using AES-256-CBC
	encIV := make([]byte, 16)
	rand.Read(encIV)

	block, err := aes.NewCipher(kek)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key derivation error")
		return
	}
	padded := pkcs7Pad(combined, aes.BlockSize)
	encrypted := make([]byte, len(padded))
	cbc := cipher.NewCBCEncrypter(block, encIV)
	cbc.CryptBlocks(encrypted, padded)

	// HMAC over encrypted key material
	mac := hmac.New(sha256.New, kek)
	mac.Write(encrypted)
	keyHmac := mac.Sum(nil)

	log.Printf("[KEY-EXCHANGE] OK: card=%s machine=%s payload=%s",
		short(session.CardCode, 8)+"...", short(req.MachineID, 8)+"...", req.PayloadID)

	writeJSON(w, map[string]interface{}{
		"status":         "ok",
		"encrypted_key":  hex.EncodeToString(encrypted),
		"encrypted_iv":   hex.EncodeToString(encIV),
		"encrypted_hmac": hex.EncodeToString(keyHmac),
	})
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := make([]byte, padding)
	for i := range pad {
		pad[i] = byte(padding)
	}
	return append(data, pad...)
}

func decodePayloadKeyMaterial(aesKey, hmacKey, iv string) ([]byte, []byte, []byte, error) {
	aesKeyBytes, err := decodeFixedHex("aes_key", aesKey, 32)
	if err != nil {
		return nil, nil, nil, err
	}
	hmacKeyBytes, err := decodeFixedHex("hmac_key", hmacKey, 32)
	if err != nil {
		return nil, nil, nil, err
	}
	ivBytes, err := decodeFixedHex("iv", iv, 16)
	if err != nil {
		return nil, nil, nil, err
	}
	return aesKeyBytes, hmacKeyBytes, ivBytes, nil
}

func decodeFixedHex(name, value string, wantBytes int) ([]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be valid hex", name)
	}
	if len(decoded) != wantBytes {
		return nil, fmt.Errorf("%s must decode to %d bytes", name, wantBytes)
	}
	return decoded, nil
}

func extractPathParam(path, prefix, suffix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	idx := strings.Index(rest, suffix)
	if idx < 0 {
		return rest
	}
	return rest[:idx]
}

// splitPayloadPath extracts payload_id and chunk index from:
// /api/v1/payload/{id}/chunk/{index}
func splitPayloadPath(path string) []string {
	const prefix = "/api/v1/payload/"
	if !strings.HasPrefix(path, prefix) {
		return nil
	}
	rest := strings.TrimPrefix(path, prefix)
	// Find "/chunk/" separator
	chunkIdx := strings.Index(rest, "/chunk/")
	if chunkIdx < 0 {
		return nil
	}
	payloadID := rest[:chunkIdx]
	chunkPart := rest[chunkIdx+7:] // after "/chunk/"
	return []string{payloadID, chunkPart}
}

// pbkdf2DeriveKey derives a key using PBKDF2-HMAC-SHA256.
// Must produce identical output to Go's golang.org/x/crypto/pbkdf2.Key
// and C#'s manual HMAC-SHA256 PBKDF2 implementation.
func pbkdf2DeriveKey(password, salt []byte, iterations, keyLen int) []byte {
	hashLen := 32
	numBlocks := (keyLen + hashLen - 1) / hashLen

	result := make([]byte, 0, numBlocks*hashLen)
	for block := 1; block <= numBlocks; block++ {
		blockSalt := make([]byte, len(salt)+4)
		copy(blockSalt, salt)
		blockSalt[len(salt)] = byte(block >> 24)
		blockSalt[len(salt)+1] = byte(block >> 16)
		blockSalt[len(salt)+2] = byte(block >> 8)
		blockSalt[len(salt)+3] = byte(block)

		mac := hmac.New(sha256.New, password)
		mac.Write(blockSalt)
		u := mac.Sum(nil)
		f := make([]byte, len(u))
		copy(f, u)

		for i := 1; i < iterations; i++ {
			mac.Reset()
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range f {
				f[j] ^= u[j]
			}
		}
		result = append(result, f...)
	}
	return result[:keyLen]
}
