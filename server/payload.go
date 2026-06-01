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
}

type PayloadStore struct {
	mu       sync.RWMutex
	payloads map[string]*PayloadInfo
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
	}
	if err := ps.storage.Load("payloads", &data); err == nil && data.Payloads != nil {
		ps.payloads = data.Payloads
	}
}

func (ps *PayloadStore) save() {
	ps.mu.RLock()
	data := struct {
		Payloads map[string]*PayloadInfo `json:"payloads"`
	}{Payloads: ps.payloads}
	ps.mu.RUnlock()
	if err := ps.storage.Save("payloads", &data); err != nil {
		log.Printf("[ERROR] Failed to persist payloads: %v", err)
	}
}

func (ps *PayloadStore) Add(info *PayloadInfo) {
	ps.mu.Lock()
	ps.payloads[info.PayloadID] = info
	ps.mu.Unlock()
	ps.save()
}

func (ps *PayloadStore) Get(payloadID string) *PayloadInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.payloads[payloadID]
}

// ── Payload Handler ─────────────────────────────────────────────────────────

type PayloadHandler struct {
	store     *PayloadStore
	uploadKey string
}

func NewPayloadHandler(store *PayloadStore) *PayloadHandler {
	key := os.Getenv("UPLOAD_KEY")
	if key == "" {
		// Try to load persisted key, or generate a new one
		if data, err := os.ReadFile("data/upload_key"); err == nil {
			key = strings.TrimSpace(string(data))
		}
		if key == "" {
			b := make([]byte, 24)
			rand.Read(b)
			key = hex.EncodeToString(b)
			os.MkdirAll("data", 0755)
			os.WriteFile("data/upload_key", []byte(key), 0600)
			log.Printf("[PAYLOAD] Generated upload key and saved to data/upload_key")
		}
	}
	return &PayloadHandler{store: store, uploadKey: key}
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

	chunkDir := filepath.Join("data", "payloads", req.PayloadID)
	os.MkdirAll(chunkDir, 0755)

	for i, chunkB64 := range req.Chunks {
		chunkData, err := base64.StdEncoding.DecodeString(chunkB64)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid chunk %d: %v", i, err))
			return
		}
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
		ChunkCount: req.ChunkCount,
		ChunkSize:  req.ChunkSize,
		TotalSize:  req.TotalSize,
		CreatedAt:  time.Now(),
	}
	h.store.Add(info)

	log.Printf("[PAYLOAD] Uploaded: id=%s chunks=%d size=%d", req.PayloadID, req.ChunkCount, req.TotalSize)
	writeJSON(w, map[string]interface{}{
		"status":      "ok",
		"payload_id":  req.PayloadID,
		"chunk_count": req.ChunkCount,
	})
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

	chunkPath := filepath.Join("data", "payloads", payloadID, fmt.Sprintf("chunk_%04d.bin", chunkIdx))
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
	aesKeyBytes, _ := hex.DecodeString(info.AesKey)
	hmacKeyBytes, _ := hex.DecodeString(info.HmacKey)
	ivBytes, _ := hex.DecodeString(info.IV)

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
