package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	cardops "github.com/lingqiao/server/internal/cards"
	"github.com/lingqiao/server/internal/features/security/blacklist"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
	releasesvc "github.com/lingqiao/server/internal/releases"
)

// APIHandler handles all HTTP API requests.
type APIHandler struct {
	blacklistSvc   *blacklist.Service
	sessionsSvc    *sessions.Service
	sessionHandler *sessions.Handler
	chat           *ChatStore
}

func NewAPIHandler(blacklistSvc *blacklist.Service, storage *JSONStorage, sessionsSvc *sessions.Service) *APIHandler {
	h := &APIHandler{blacklistSvc: blacklistSvc, sessionsSvc: sessionsSvc, chat: NewChatStore(storage, time.Now)}
	h.sessionHandler = sessions.NewHandler(sessionsSvc, h, h, httpapi.GetClientIP)
	return h
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

	if err := verifyRequestHMAC(r, string(body)); err != nil {
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
	if req.ClientVersion == "" {
		return false, nil
	}
	svc := currentReleaseService()
	if svc == nil {
		return false, nil
	}

	selected, err := releasesvc.NewSelector(svc.store).Select(context.Background(), releasesvc.ClientContext{
		Version:   req.ClientVersion,
		Channel:   releasesvc.ChannelStable,
		MachineID: req.MachineID,
		CardCode:  req.Card,
		AgentID:   "",
	})
	if err != nil || selected == nil {
		return false, nil
	}

	manifest := manifestFromSelection(*selected)

	info := map[string]interface{}{
		"update_available": true,
		"latest_version":   selected.Release.Version,
		"force_update":     selected.Release.ForceUpdate,
		"download_url":     manifest.PackageURL,
		"sha256":           manifest.PackageSHA256,
	}

	if selected.Release.MinVersion != "" && compareVersion(req.ClientVersion, selected.Release.MinVersion) {
		resp := map[string]interface{}{
			"status":       "error",
			"message":      fmt.Sprintf("版本过低，请更新到 v%s 或更高版本", selected.Release.MinVersion),
			"download_url": manifest.PackageURL,
			"sha256":       manifest.PackageSHA256,
		}
		httpapi.WriteJSON(w, http.StatusForbidden, resp)
		return true, nil
	}

	return false, info
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
	a := GetAnnouncement()
	if a == nil {
		httpapi.WriteOK(w, map[string]interface{}{"announcement": nil})
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"announcement": a})
}

func (h *APIHandler) HandleDllDownload(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	// Verify HMAC signature — mandatory for all DLL downloads
	if err := verifyRequestHMAC(r, r.URL.Path); err != nil {
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

	dllPath := dataPath("CefHook.dll")
	dllBytes, err := os.ReadFile(dllPath)
	if err != nil {
		log.Printf("[DLL] Failed to read %s: %v", dllPath, err)
		httpapi.WriteError(w, http.StatusInternalServerError, "DLL not found on server")
		return
	}

	// Encrypt with the client-compatible DLL key. The client expects:
	// [12-byte GCM nonce][ciphertext][16-byte GCM tag].
	clientSecret := defaultClientSecret()
	salt := []byte("CefBridge-DLL-Salt-v1")
	key := pbkdf2([]byte(clientSecret), salt, 100000, 32)
	encrypted, err := encryptDLLForClient(dllBytes, key)
	if err != nil {
		log.Printf("[DLL] Failed to encrypt response: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "DLL encryption failed")
		return
	}

	mid := session.MachineID
	if len(mid) > 8 {
		mid = short(mid, 8) + "..."
	}
	log.Printf("[DLL] Served to machine=%s size=%d", mid, len(encrypted))

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(encrypted)))
	w.Write(encrypted)
}

func encryptDLLForClient(plain, key []byte) ([]byte, error) {
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
	default:
		return err.Error()
	}
}
