package payloads

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
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
	"github.com/lingqiao/server/internal/utils"
)

type Handler struct {
	store       *PayloadStore
	sessionsSvc *sessions.Service
	dataDir     string
	uploadKey   string
	auditSink   func(audit.Event)
}

func NewHandler(store *PayloadStore, sessionsSvc *sessions.Service, dataDir string) (*Handler, error) {
	key := os.Getenv("UPLOAD_KEY")
	if key == "" {
		keyPath := filepath.Join(dataDir, "upload_key")
		if data, err := os.ReadFile(keyPath); err == nil {
			key = strings.TrimSpace(string(data))
		}
		if key == "" {
			b := make([]byte, 24)
			if _, err := rand.Read(b); err != nil {
				return nil, fmt.Errorf("failed to generate upload key: %w", err)
			}
			key = hex.EncodeToString(b)
			os.MkdirAll(dataDir, 0755)
			if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
				return nil, fmt.Errorf("failed to save upload key: %w", err)
			}
			log.Printf("[PAYLOAD] Generated upload key and saved to %s", keyPath)
		}
	}
	if key == "" {
		return nil, fmt.Errorf("upload key is empty")
	}
	return &Handler{store: store, sessionsSvc: sessionsSvc, dataDir: dataDir, uploadKey: key}, nil
}

func (h *Handler) SetAuditSink(auditSink func(audit.Event)) {
	h.auditSink = auditSink
}

// HandleUpload receives encrypted payload chunks from the pack script.
func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uploadKey := r.Header.Get("X-Upload-Key")
	if uploadKey != h.uploadKey {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid upload key")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 100<<20))
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "failed to read body")
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.PayloadID == "" || req.AesKey == "" || len(req.Chunks) == 0 {
		httpapi.WriteError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	if err := validatePayloadID(req.PayloadID); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid payload_id")
		return
	}

	if _, _, _, err := decodePayloadKeyMaterial(req.AesKey, req.HmacKey, req.IV); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid key material")
		return
	}
	if _, err := decodeFixedHex("exe_hash", req.ExeHash, 32); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid exe_hash")
		return
	}
	if req.ChunkCount != len(req.Chunks) {
		httpapi.WriteError(w, http.StatusBadRequest, "chunk_count does not match chunks length")
		return
	}
	if req.ChunkSize <= 0 {
		httpapi.WriteError(w, http.StatusBadRequest, "chunk_size must be positive")
		return
	}
	if req.TotalSize <= 0 {
		httpapi.WriteError(w, http.StatusBadRequest, "total_size must be positive")
		return
	}

	decodedChunks := make([][]byte, len(req.Chunks))
	actualTotal := 0
	for i, chunkB64 := range req.Chunks {
		chunkData, err := base64.StdEncoding.DecodeString(chunkB64)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid chunk %d", i))
			return
		}
		if len(chunkData) == 0 {
			httpapi.WriteError(w, http.StatusBadRequest, fmt.Sprintf("chunk %d is empty", i))
			return
		}
		if len(chunkData) > req.ChunkSize {
			httpapi.WriteError(w, http.StatusBadRequest, fmt.Sprintf("chunk %d exceeds chunk_size", i))
			return
		}
		if i < len(req.Chunks)-1 && len(chunkData) != req.ChunkSize {
			httpapi.WriteError(w, http.StatusBadRequest, fmt.Sprintf("chunk %d size does not match chunk_size", i))
			return
		}
		decodedChunks[i] = chunkData
		actualTotal += len(chunkData)
	}
	if actualTotal != req.TotalSize {
		httpapi.WriteError(w, http.StatusBadRequest, "total_size does not match decoded chunks")
		return
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

	if err := savePayloadChunks(h.dataDir, info, decodedChunks); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to save payload")
		return
	}
	if err := h.store.Add(info); err != nil {
		_ = os.RemoveAll(filepath.Join(h.dataDir, "payloads", req.PayloadID))
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to persist payload metadata")
		return
	}
	h.recordPayloadAudit("payload_uploaded", info, "")

	log.Printf("[PAYLOAD] Uploaded: id=%s chunks=%d size=%d", req.PayloadID, len(decodedChunks), actualTotal)
	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"payload_id":  req.PayloadID,
		"chunk_count": len(decodedChunks),
	})
}

// HandleAdminDLLUpload accepts a raw DLL from the admin panel, encrypts/chunks it, and optionally activates it.
func (h *Handler) HandleAdminDLLUpload(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	if err := r.ParseMultipartForm(200 << 20); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".dll") {
		httpapi.WriteError(w, http.StatusBadRequest, "only .dll payloads are supported")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	if !isPEPayload(data) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid DLL payload")
		return
	}

	payloadID := r.FormValue("payload_id")
	if payloadID == "" {
		payloadID = generatePayloadID()
	}
	if err := validatePayloadID(payloadID); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid payload_id")
		return
	}
	if h.store.Get(payloadID) != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "payload_id already exists")
		return
	}

	info, chunks, err := PackPlainDLL(payloadID, data, defaultChunkSize)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "failed to pack dll payload")
		return
	}

	if err := savePayloadChunks(h.dataDir, info, chunks); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to save payload")
		return
	}
	if err := h.store.Add(info); err != nil {
		_ = os.RemoveAll(filepath.Join(h.dataDir, "payloads", payloadID))
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to persist payload metadata")
		return
	}
	h.recordPayloadAudit("payload_uploaded", info, "")

	activate := r.FormValue("activate") == "true" || r.FormValue("activate") == "1"
	if activate {
		if err := h.store.SetActive(payloadID); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to activate payload")
			return
		}
		h.recordPayloadAudit("payload_activated", info, "")
	}

	log.Printf("[PAYLOAD] Admin DLL uploaded: id=%s chunks=%d size=%d activate=%v", payloadID, info.ChunkCount, info.TotalSize, activate)
	httpapi.WriteOK(w, map[string]interface{}{"payload": payloadSummaryFromInfo(h.store.Get(payloadID))})
}

func (h *Handler) HandleAdminList(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	payloads := h.store.List()
	var latest *PayloadSummary
	if len(payloads) > 0 {
		latest = &payloads[0]
	}
	active := payloadSummaryFromInfo(h.store.Active())
	httpapi.WriteOK(w, map[string]interface{}{
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

func (h *Handler) HandleAdminManage(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Action    string `json:"action"`
		PayloadID string `json:"payload_id"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch req.Action {
	case "activate":
		if err := h.store.SetActive(req.PayloadID); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "failed to activate payload")
			return
		}
		info := h.store.Get(req.PayloadID)
		h.recordPayloadAudit("payload_activated", info, "")
		httpapi.WriteOK(w, map[string]interface{}{"payload": payloadSummaryFromInfo(info)})
	case "delete":
		info := h.store.Get(req.PayloadID)
		if err := h.store.Delete(req.PayloadID); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "failed to delete payload")
			return
		}
		if err := os.RemoveAll(filepath.Join(h.dataDir, "payloads", req.PayloadID)); err != nil {
			log.Printf("[PAYLOAD] metadata deleted but payload files remain for id=%s: %v", req.PayloadID, err)
		}
		h.recordPayloadAudit("payload_deleted", info, "")
		httpapi.WriteOK(w, nil)
	default:
		httpapi.WriteError(w, http.StatusBadRequest, "unknown action")
	}
}

// HandlePayloadInfo returns metadata about a payload (requires HMAC auth).
func (h *Handler) HandlePayloadInfo(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	if err := auth.VerifyRequest(r, r.URL.Path); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "missing auth")
		return
	}
	payloadID := extractPathParam(r.URL.Path, "/api/v1/payload/", "/info")
	if payloadID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing payload_id")
		return
	}

	info := h.store.Get(payloadID)
	if info == nil {
		httpapi.WriteError(w, http.StatusNotFound, "payload not found")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"payload_id":  info.PayloadID,
		"chunk_count": info.ChunkCount,
		"chunk_size":  info.ChunkSize,
		"total_size":  info.TotalSize,
		"exe_hash":    info.ExeHash,
		"created_at":  info.CreatedAt,
	})
}

func (h *Handler) recordPayloadAudit(action string, info *PayloadInfo, extra string) {
	if h == nil || h.auditSink == nil || info == nil {
		return
	}
	detail := fmt.Sprintf("id=%s sha256=%s chunks=%d size=%d", info.PayloadID, info.ExeHash, info.ChunkCount, info.TotalSize)
	if extra != "" {
		detail += " " + extra
	}
	h.auditSink(audit.Event{
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
func (h *Handler) HandleChunkDownload(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	parts := splitPayloadPath(r.URL.Path)
	if len(parts) < 2 {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid path")
		return
	}
	payloadID := parts[0]
	chunkIdxStr := parts[1]

	if err := validatePayloadID(payloadID); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid payload_id")
		return
	}

	chunkIdx, err := strconv.Atoi(chunkIdxStr)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid chunk index")
		return
	}

	info := h.store.Get(payloadID)
	if info == nil {
		httpapi.WriteError(w, http.StatusNotFound, "payload not found")
		return
	}

	if chunkIdx < 0 || chunkIdx >= info.ChunkCount {
		httpapi.WriteError(w, http.StatusBadRequest, "chunk index out of range")
		return
	}

	chunkPath := filepath.Join(h.dataDir, "payloads", payloadID, fmt.Sprintf("chunk_%04d.bin", chunkIdx))
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "chunk file not found")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// HandleKeyExchange: client sends session_token + machine_id + payload_id,
// server returns the AES key encrypted with KEK derived from card_code + HWID.
func (h *Handler) HandleKeyExchange(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	sig := r.Header.Get("X-HMAC-Signature")
	clientID := r.Header.Get("X-Client-ID")
	timestamp := r.Header.Get("X-Timestamp")
	nonce := r.Header.Get("X-Nonce")
	if sig == "" || clientID == "" || timestamp == "" || nonce == "" {
		httpapi.WriteError(w, http.StatusUnauthorized, "missing auth headers")
		return
	}
	if err := auth.VerifyHMAC(clientID, string(body), sig, timestamp, nonce); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "HMAC verification failed")
		return
	}

	var req struct {
		SessionToken string `json:"session_token"`
		MachineID    string `json:"machine_id"`
		PayloadID    string `json:"payload_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Verify session
	session, err := h.sessionsSvc.ValidateBound(r.Context(), req.SessionToken, req.MachineID, "")
	if err != nil {
		httpapi.WriteError(w, http.StatusForbidden, "invalid or expired session")
		return
	}

	info := h.store.Get(req.PayloadID)
	if info == nil {
		httpapi.WriteError(w, http.StatusNotFound, "payload not found")
		return
	}

	// Derive KEK from card_code + HWID
	normalizedCard := cards.NormalizeCode(session.CardCode)
	kekMaterial := normalizedCard + req.MachineID
	salt := []byte("payload_key_" + req.PayloadID)
	kek := pbkdf2DeriveKey([]byte(kekMaterial), salt, 100000, 32)

	// Combine key material: aesKey(32) + hmacKey(32) + iv(16) = 80 bytes
	aesKeyBytes, hmacKeyBytes, ivBytes, err := decodePayloadKeyMaterial(info.AesKey, info.HmacKey, info.IV)
	if err != nil {
		log.Printf("[KEY-EXCHANGE] Invalid payload key material: payload=%s error=%v", req.PayloadID, err)
		httpapi.WriteError(w, http.StatusInternalServerError, "invalid payload key material")
		return
	}

	combined := make([]byte, 0, 80)
	combined = append(combined, aesKeyBytes...)
	combined = append(combined, hmacKeyBytes...)
	combined = append(combined, ivBytes...)

	// Encrypt combined key material with KEK using AES-256-CBC
	encIV := make([]byte, 16)
	if _, err := rand.Read(encIV); err != nil {
		log.Printf("[KEY-EXCHANGE] Failed to generate IV: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to generate IV")
		return
	}

	block, err := aes.NewCipher(kek)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "key derivation error")
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
		utils.Short(session.CardCode, 8)+"...", utils.Short(req.MachineID, 8)+"...", req.PayloadID)

	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
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

// DecodePayloadKeyMaterialForServer exposes the same validation used by the
// payload handlers for server-side consumers that need to read active payloads.
func DecodePayloadKeyMaterialForServer(aesKey, hmacKey, iv string) ([]byte, []byte, []byte, error) {
	return decodePayloadKeyMaterial(aesKey, hmacKey, iv)
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
