package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lingqiao/server/internal/storage"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lingqiao/server/internal/auth"
	cardops "github.com/lingqiao/server/internal/cards"
	"github.com/lingqiao/server/internal/features/payloads"
	"github.com/lingqiao/server/internal/features/security/blacklist"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
)

// APIHandler handles all HTTP API requests.
type APIHandler struct {
	blacklistSvc   *blacklist.Service
	sessionsSvc    *sessions.Service
	sessionHandler *sessions.Handler
	versionCheck   sessions.VersionChecker
	payloadStore   activePayloadStore
	dataDir        string
}

func NewAPIHandler(blacklistSvc *blacklist.Service, storage storage.Store, sessionsSvc *sessions.Service, versionCheck sessions.VersionChecker) *APIHandler {
	h := &APIHandler{blacklistSvc: blacklistSvc, sessionsSvc: sessionsSvc, versionCheck: versionCheck, dataDir: dataDir()}
	h.sessionHandler = sessions.NewHandler(sessionsSvc, h, h, httpapi.GetClientIP)
	return h
}

type activePayloadStore interface {
	Active() *payloads.PayloadInfo
}

type dllSource struct {
	Bytes     []byte
	Kind      string
	PayloadID string
	SHA256    string
}

func (h *APIHandler) SetPayloadStore(store activePayloadStore, dataDir string) {
	h.payloadStore = store
	if dataDir != "" {
		h.dataDir = dataDir
	}
}

// ReadSignedRequest implements sessions.SignedRequestReader
func (h *APIHandler) ReadSignedRequest(r *http.Request) (sessions.ClientRequest, error) {
	req, err := h.readSignedRequest(r)
	if err != nil {
		return sessions.ClientRequest{}, err
	}
	return sessions.ClientRequest{
		Card:          req.Card,
		MachineID:     req.MachineID,
		Fingerprint:   req.Fingerprint,
		ClientVersion: req.ClientVersion,
		SessionToken:  req.SessionToken,
	}, nil
}

// CheckVersion implements sessions.VersionChecker
func (h *APIHandler) CheckVersion(w http.ResponseWriter, req sessions.ClientRequest) (bool, map[string]interface{}) {
	return h.checkVersion(w, req)
}

type request struct {
	ClientID      string `json:"client_id"`
	Card          string `json:"card"`
	MachineID     string `json:"machine_id"`
	Fingerprint   string `json:"fingerprint"`
	SessionToken  string `json:"session_token"`
	ClientVersion string `json:"client_version"`
	Content       string `json:"content"`
	Nickname      string `json:"nickname"`
	AfterID       int64  `json:"after_id"`
	ReplyToID     int64  `json:"reply_to_id"`
	MessageID     int64  `json:"message_id"`
	Reaction      string `json:"reaction"`
}

type response struct {
	Status        string `json:"status"`
	Message       string `json:"message,omitempty"`
	SessionToken  string `json:"session_token,omitempty"`
	ExpiresAt     int64  `json:"expires_at,omitempty"`
	CardExpiresAt int64  `json:"card_expires_at,omitempty"`
}

func (h *APIHandler) readSignedRequest(r *http.Request) (*request, error) {
	body, err := httpapi.ReadBody(r)
	if err != nil {
		return nil, err
	}

	if err := auth.VerifyRequest(r, string(body)); err != nil {
		return nil, err
	}

	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	isBlacklisted, err := h.blacklistSvc.IsBlacklisted(r.Context(), req.MachineID)
	if err != nil {
		return nil, fmt.Errorf("failed to check blacklist status: %v", err)
	}
	if isBlacklisted {
		return nil, fmt.Errorf("machine is blacklisted")
	}
	return &req, nil
}

// checkVersion enforces minimum version and returns update info if applicable.
// Returns (blocked, updateInfo) — if blocked is true, the response has already been written.
func (h *APIHandler) checkVersion(w http.ResponseWriter, req sessions.ClientRequest) (blocked bool, updateInfo map[string]interface{}) {
	if h.versionCheck != nil {
		return h.versionCheck.CheckVersion(w, req)
	}
	return false, nil
}

func (h *APIHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	h.sessionHandler.HandleActivate(w, r)
}

func (h *APIHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	h.sessionHandler.HandleHeartbeat(w, r)
}

func (h *APIHandler) HandleDeactivate(w http.ResponseWriter, r *http.Request) {
	h.sessionHandler.HandleDeactivate(w, r)
}

func (h *APIHandler) HandleAnnouncement(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	a := GetAnnouncement()
	if a == nil {
		httpapi.WriteOK(w, map[string]interface{}{"announcement": nil})
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"announcement": a})
}

func (h *APIHandler) handleAgentUpdateCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *APIHandler) HandleDllDownload(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	// Verify HMAC signature — mandatory for all DLL downloads
	if err := auth.VerifyRequest(r, r.URL.Path); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "signature verification failed")
		return
	}

	token := r.Header.Get("X-Session-Token")
	if token == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "token is required")
		return
	}

	session, err := h.sessionsSvc.ValidateBound(r.Context(), token, r.Header.Get("X-Machine-ID"), r.Header.Get("X-Card-Code"))
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	source, err := h.loadDLLSource(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "DLL not found on server")
		return
	}

	// Encrypt with the client-compatible DLL key. The client expects:
	// [12-byte GCM nonce][ciphertext][16-byte GCM tag].

	key, err := auth.GetDLLKey(r.Header.Get("X-Client-ID"))
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "client key unavailable")
		return
	}
	encrypted, err := encryptDLLForClient(source.Bytes, key)
	if err != nil {
		log.Printf("[DLL] Failed to encrypt response: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "DLL encryption failed")
		return
	}

	mid := session.MachineID
	if len(mid) > 8 {
		mid = mid[:8] + "..."
	}
	log.Printf("[DLL] Served to machine=%s source=%s payload=%s sha256=%s size=%d",
		mid, source.Kind, source.PayloadID, source.SHA256, len(encrypted))

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(encrypted)))
	w.Header().Set("X-DLL-Source", source.Kind)
	w.Header().Set("X-DLL-SHA256", source.SHA256)
	if source.PayloadID != "" {
		w.Header().Set("X-DLL-Payload-ID", source.PayloadID)
	}
	w.Write(encrypted)
}

func (h *APIHandler) loadDLLSource(ctx context.Context) (dllSource, error) {
	_ = ctx
	if h.payloadStore != nil {
		if active := h.payloadStore.Active(); active != nil {
			plain, err := h.loadActivePayloadDLL(active)
			if err != nil {
				log.Printf("[DLL] Active payload unavailable: id=%s err=%v", active.PayloadID, err)
			} else {
				return newDLLSource(plain, "active-payload", active.PayloadID), nil
			}
		}
	}

	baseDir := h.dataDir
	if baseDir == "" {
		baseDir = dataDir()
	}
	dllPath := filepath.Join(baseDir, "CefHook.dll")
	dllBytes, err := os.ReadFile(dllPath)
	if err != nil {
		log.Printf("[DLL] Failed to read %s: %v", dllPath, err)
		return dllSource{}, err
	}
	return newDLLSource(dllBytes, "legacy-file", ""), nil
}

func (h *APIHandler) loadActivePayloadDLL(info *payloads.PayloadInfo) ([]byte, error) {
	if info == nil {
		return nil, fmt.Errorf("active payload is nil")
	}
	baseDir := h.dataDir
	if baseDir == "" {
		baseDir = dataDir()
	}
	aesKey, _, iv, err := payloads.DecodePayloadKeyMaterialForServer(info.AesKey, info.HmacKey, info.IV)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	encrypted := make([]byte, 0, info.TotalSize)
	for i := 0; i < info.ChunkCount; i++ {
		chunkPath := filepath.Join(baseDir, "payloads", info.PayloadID, fmt.Sprintf("chunk_%04d.bin", i))
		chunk, err := os.ReadFile(chunkPath)
		if err != nil {
			return nil, err
		}
		encrypted = append(encrypted, chunk...)
	}
	plain := make([]byte, len(encrypted))
	cipher.NewCTR(block, iv).XORKeyStream(plain, encrypted)
	if len(plain) < 2 || plain[0] != 'M' || plain[1] != 'Z' {
		return nil, fmt.Errorf("active payload is not a DLL")
	}
	sum := sha256.Sum256(plain)
	if hex.EncodeToString(sum[:]) != info.ExeHash {
		return nil, fmt.Errorf("active payload hash mismatch")
	}
	return plain, nil
}

func newDLLSource(data []byte, kind, payloadID string) dllSource {
	sum := sha256.Sum256(data)
	return dllSource{
		Bytes:     data,
		Kind:      kind,
		PayloadID: payloadID,
		SHA256:    hex.EncodeToString(sum[:]),
	}
}

func encryptDLLForClient(plain, key []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("invalid AES key length: %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	encrypted := gcm.Seal(append([]byte{}, nonce...), nonce, plain, nil)
	return encrypted, nil
}

func (h *APIHandler) SessionCleanupTask(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.sessionsSvc.CleanupExpired(ctx)
		}
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func translateCardError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, cardops.ErrCardNotFound):
		return "卡密不存在 — 请检查是否输入正确"
	case errors.Is(err, cardops.ErrCardBlacklisted):
		return "卡密已被列入黑名单"
	case errors.Is(err, cardops.ErrCardDisabled):
		return "卡密已被禁用 — 请联系管理员"
	case errors.Is(err, cardops.ErrCardExpired):
		return "卡密已过期 — 请续费或更换卡密"
	case errors.Is(err, cardops.ErrCardBoundToOtherMachine):
		return "卡密已绑定到其他机器 — 请联系管理员解绑"
	case errors.Is(err, cardops.ErrMaxSessionsReached):
		return "已达到最大同时在线数 — 请先下线其他设备"
	case errors.Is(err, cardops.ErrSessionExpired), errors.Is(err, cardops.ErrSessionNotFound):
		return "会话无效或已过期 — 请重新激活"
	case errors.Is(err, cardops.ErrMachineMismatch):
		return "机器信息不匹配 — 请使用绑定设备"
	case errors.Is(err, cardops.ErrCardNoLongerValid):
		return "卡密已失效 — 请联系管理员"
	default:
		// Never leak raw internal error strings to clients.
		return "操作失败 — 请稍后重试或联系管理员"
	}
}
