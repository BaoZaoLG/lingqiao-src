package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// APIHandler handles all HTTP API requests.
type APIHandler struct {
	cm *CardManager
}

func NewAPIHandler(cm *CardManager) *APIHandler {
	return &APIHandler{cm: cm}
}

type request struct {
	ClientID      string `json:"client_id"`
	Card          string `json:"card"`
	MachineID     string `json:"machine_id"`
	Fingerprint   string `json:"fingerprint"`
	SessionToken  string `json:"session_token"`
	ClientVersion string `json:"client_version"`
}

type response struct {
	Status        string `json:"status"`
	Message       string `json:"message,omitempty"`
	SessionToken  string `json:"session_token,omitempty"`
	ExpiresAt     int64  `json:"expires_at,omitempty"`
	CardExpiresAt int64  `json:"card_expires_at,omitempty"`
}

func (h *APIHandler) readSignedRequest(r *http.Request) (*request, error) {
	body, err := readBody(r)
	if err != nil {
		return nil, err
	}

	if err := verifyRequestHMAC(r, string(body)); err != nil {
		return nil, err
	}

	var req request
	if err := jsonUnmarshal(body, &req); err != nil {
		return nil, err
	}

	if h.cm.IsBlacklisted(req.MachineID) {
		return nil, fmt.Errorf("machine is blacklisted")
	}
	return &req, nil
}

// checkVersion enforces minimum version and returns update info if applicable.
// Returns (blocked, updateInfo) — if blocked is true, the response has already been written.
func (h *APIHandler) checkVersion(w http.ResponseWriter, clientVersion string) (blocked bool, updateInfo map[string]interface{}) {
	a := GetAnnouncement()
	if a == nil || clientVersion == "" {
		return false, nil
	}

	// Hard block if below minimum version
	if a.MinVersion != "" && compareVersion(clientVersion, a.MinVersion) {
		w.WriteHeader(http.StatusForbidden)
		writeJSON(w, map[string]interface{}{
			"status":       "error",
			"message":      fmt.Sprintf("版本过低，请更新到 v%s 或更高版本", a.MinVersion),
			"download_url": "/admin/api/update/download",
		})
		return true, nil
	}

	// Soft push if below latest version
	if a.LatestVersion != "" && compareVersion(clientVersion, a.LatestVersion) {
		return false, map[string]interface{}{
			"update_available": true,
			"latest_version":   a.LatestVersion,
			"force_update":     a.ForceUpdate,
			"download_url":     "/admin/api/update/download",
		}
	}
	return false, nil
}

func (h *APIHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	req, err := h.readSignedRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if req.Card == "" {
		writeError(w, http.StatusBadRequest, "card is required")
		return
	}

	if blocked, _ := h.checkVersion(w, req.ClientVersion); blocked {
		return
	}

	session, err := h.cm.ActivateCard(req.Card, req.MachineID, req.Fingerprint, getClientIP(r), req.ClientVersion)
	if err != nil {
		log.Printf("[ACTIVATE] Failed: card=%s machine=%s error=%v", req.Card, req.MachineID, err)
		writeError(w, http.StatusForbidden, translateCardError(err))
		return
	}

	card := h.cm.GetCard(req.Card)
	var cardExp int64
	if card != nil {
		cardExp = card.ExpiresAt.Unix()
	}

	log.Printf("[ACTIVATE] OK: card=%s machine=%s token=%s", req.Card, req.MachineID, short(session.Token, 8)+"...")

	_, updateInfo := h.checkVersion(w, req.ClientVersion)
	resp := map[string]interface{}{
		"session_token":   session.Token,
		"expires_at":      session.ExpiresAt.Unix(),
		"card_expires_at": cardExp,
		"message":         "激活成功",
	}
	if updateInfo != nil {
		resp["update"] = updateInfo
	}
	writeOK(w, resp)
}

func (h *APIHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	req, err := h.readSignedRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if req.SessionToken == "" {
		writeError(w, http.StatusBadRequest, "session_token is required")
		return
	}

	if blocked, _ := h.checkVersion(w, req.ClientVersion); blocked {
		return
	}

	session, err := h.cm.Heartbeat(req.SessionToken, req.MachineID, getClientIP(r), req.ClientVersion)
	if err != nil {
		log.Printf("[HEARTBEAT] Failed: token=%s error=%v", short(req.SessionToken, 8)+"...", err)
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	card := h.cm.GetCard(session.CardCode)
	var cardExp int64
	if card != nil {
		cardExp = card.ExpiresAt.Unix()
	}

	_, updateInfo := h.checkVersion(w, req.ClientVersion)
	resp := map[string]interface{}{
		"expires_at":      session.ExpiresAt.Unix(),
		"card_expires_at": cardExp,
		"message":         "心跳正常",
	}
	if updateInfo != nil {
		resp["update"] = updateInfo
	}
	writeOK(w, resp)
}

func (h *APIHandler) HandleDeactivate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	req, err := h.readSignedRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if err := h.cm.DeactivateSession(req.SessionToken, false); err != nil {
		writeJSON(w, response{Status: "ok", Message: "会话不存在或已过期"})
		return
	}
	writeJSON(w, response{Status: "ok", Message: "已注销"})
}

func (h *APIHandler) HandleAnnouncement(w http.ResponseWriter, r *http.Request) {
	a := GetAnnouncement()
	if a == nil {
		writeOK(w, map[string]interface{}{"announcement": nil})
		return
	}
	writeOK(w, map[string]interface{}{"announcement": a})
}

func (h *APIHandler) HandleDllDownload(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Verify HMAC signature if present (signed DLL download)
	if r.Header.Get("X-HMAC-Signature") != "" {
		if err := verifyRequestHMAC(r, r.URL.Path); err != nil {
			writeError(w, http.StatusUnauthorized, "signature verification failed")
			return
		}
	}

	token := r.Header.Get("X-Session-Token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	h.cm.mu.RLock()
	session, exists := h.cm.sessions[token]
	h.cm.mu.RUnlock()
	if !exists || session.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	dllPath := "data/CefHook.dll"
	dllBytes, err := os.ReadFile(dllPath)
	if err != nil {
		log.Printf("[DLL] Failed to read %s: %v", dllPath, err)
		writeError(w, http.StatusInternalServerError, "DLL not found on server")
		return
	}

	// XOR encrypt with key derived from client secret + machine ID
	clientSecret := knownClients[0].Secret
	salt := []byte("CefBridge-DLL-Salt-v1")
	key := pbkdf2([]byte(clientSecret), salt, 100000, 32)

	encrypted := make([]byte, len(dllBytes))
	copy(encrypted, dllBytes)
	for i := range encrypted {
		encrypted[i] ^= key[i%len(key)]
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

func (h *APIHandler) SessionCleanupTask() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		h.cm.CleanupExpired()
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func translateCardError(err error) string {
	msg := err.Error()
	switch msg {
	case "card not found":
		return "卡密不存在 — 请检查是否输入正确"
	case "card is blacklisted":
		return "卡密已被列入黑名单"
	case "card is disabled":
		return "卡密已被禁用 — 请联系管理员"
	case "card has expired":
		return "卡密已过期 — 请续费或更换卡密"
	case "card already bound to another machine":
		return "卡密已绑定到其他机器 — 请联系管理员解绑"
	default:
		return msg
	}
}

