package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AgentHandler handles the agent panel API, completely isolated from admin.
type AgentHandler struct {
	cm            *CardManager
	mu            sync.Mutex
	sessions      map[string]time.Time
	fileMu        sync.Mutex // protects JSON file reads/writes for token map
	loginLimiter  *rateLimiter
	regLimiter    *rateLimiter
	pwLimiter     *rateLimiter
}

func NewAgentHandler(cm *CardManager) *AgentHandler {
	h := &AgentHandler{
		cm:           cm,
		sessions:     make(map[string]time.Time),
		loginLimiter: newRateLimiter(15*time.Minute, 5),
		regLimiter:   newRateLimiter(1*time.Hour, 3),
		pwLimiter:    newRateLimiter(1*time.Hour, 3),
	}
	h.loadSessions()
	return h
}

func (h *AgentHandler) loadSessions() {
	data, err := os.ReadFile("data/agent_sessions.json")
	if err != nil {
		return
	}
	var saved map[string]time.Time
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}
	h.mu.Lock()
	now := time.Now()
	for token, expiry := range saved {
		if expiry.After(now) {
			h.sessions[token] = expiry
		}
	}
	h.mu.Unlock()
}

func (h *AgentHandler) saveSessions() {
	h.mu.Lock()
	data, _ := json.Marshal(h.sessions)
	h.mu.Unlock()
	os.WriteFile("data/agent_sessions.json", data, 0600)
}

func (h *AgentHandler) createSession() string {
	b := make([]byte, 32)
	rand.Read(b)
	token := "agent_" + hex.EncodeToString(b)
	h.mu.Lock()
	h.sessions[hashToken(token)] = time.Now().Add(8 * time.Hour)
	h.mu.Unlock()
	h.saveSessions()
	return token
}

func (h *AgentHandler) checkSession(r *http.Request) (string, bool) {
	token := extractToken(r, "agent_token")
	if token == "" || len(token) < 7 || token[:6] != "agent_" {
		return "", false
	}
	h.mu.Lock()
	expiry, exists := h.sessions[hashToken(token)]
	h.mu.Unlock()
	if !exists || !time.Now().Before(expiry) {
		return "", false
	}
	agentID := h.getAgentIDForToken(token)
	return agentID, agentID != ""
}

func (h *AgentHandler) getAgentIDForToken(token string) string {
	h.fileMu.Lock()
	defer h.fileMu.Unlock()
	data, err := os.ReadFile("data/agent_token_map.json")
	if err != nil {
		return ""
	}
	var m map[string]string
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	return m[hashToken(token)]
}

func (h *AgentHandler) mapTokenToAgent(token, agentID string) {
	h.fileMu.Lock()
	defer h.fileMu.Unlock()
	data, _ := os.ReadFile("data/agent_token_map.json")
	m := make(map[string]string)
	json.Unmarshal(data, &m)
	m[hashToken(token)] = agentID
	d, _ := json.Marshal(m)
	os.WriteFile("data/agent_token_map.json", d, 0600)
}

func (h *AgentHandler) agentAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID, ok := h.checkSession(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		r.Header.Set("X-Agent-ID", agentID)
		next(w, r)
	}
}

// ── Invite Code Management ───────────────────────────────────────────────────

type InviteCode struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
	UsedBy    string    `json:"used_by,omitempty"`
	UsedAt    time.Time `json:"used_at,omitempty"`
	MaxUses   int       `json:"max_uses"`
	UseCount  int       `json:"use_count"`
}

func LoadInviteCodes() []InviteCode {
	data, err := os.ReadFile("data/invites.json")
	if err != nil {
		return []InviteCode{}
	}
	var codes []InviteCode
	json.Unmarshal(data, &codes)
	return codes
}

func SaveInviteCodes(codes []InviteCode) {
	os.MkdirAll("data", 0755)
	d, _ := json.MarshalIndent(codes, "", "  ")
	os.WriteFile("data/invites.json", d, 0600)
}

func ValidateAndUseInvite(code string) error {
	if code == "" {
		return fmt.Errorf("邀请码不能为空")
	}
	codes := LoadInviteCodes()
	for i, c := range codes {
		if c.Code == code {
			if c.MaxUses > 0 && c.UseCount >= c.MaxUses {
				return fmt.Errorf("邀请码已用完")
			}
			codes[i].UseCount++
			if codes[i].UsedBy == "" {
				codes[i].UsedBy = code
			}
			codes[i].UsedAt = time.Now()
			SaveInviteCodes(codes)
			return nil
		}
	}
	return fmt.Errorf("邀请码无效")
}

func CreateInviteCode(maxUses int, createdBy string) InviteCode {
	b := make([]byte, 6)
	rand.Read(b)
	code := "LQ-" + strings.ToUpper(hex.EncodeToString(b))
	inv := InviteCode{
		Code:      code,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		MaxUses:   maxUses,
	}
	codes := LoadInviteCodes()
	codes = append(codes, inv)
	SaveInviteCodes(codes)
	return inv
}

// ── Agent Auth Handlers ──────────────────────────────────────────────────────

func (h *AgentHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	ip := getClientIP(r)
	if !h.regLimiter.allow("reg_" + ip) {
		writeError(w, http.StatusTooManyRequests, "注册过于频繁，请稍后再试")
		return
	}

	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		InviteCode string `json:"invite_code"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 || len(req.Username) > 20 {
		writeError(w, http.StatusBadRequest, "用户名长度需在3-20个字符之间")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "密码长度不能少于6位")
		return
	}
	if len(req.Password) > 128 {
		writeError(w, http.StatusBadRequest, "密码长度不能超过128位")
		return
	}
	for _, c := range req.Username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			writeError(w, http.StatusBadRequest, "用户名只能包含字母、数字和下划线")
			return
		}
	}

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "密码加密失败")
		return
	}

	// Consume invite code only after all validation passes
	if err := ValidateAndUseInvite(req.InviteCode); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	agent, err := h.cm.CreateAgent(req.Username, string(bcryptHash), "")
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, "用户名已存在")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	log.Printf("[AGENT] Registered: %s (%s)", agent.Username, agent.ID)
	writeOK(w, map[string]interface{}{
		"agent_id": agent.ID,
		"username": agent.Username,
	})
}

func (h *AgentHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	ip := getClientIP(r)
	if !h.loginLimiter.allow("login_" + ip) {
		writeError(w, http.StatusTooManyRequests, "登录尝试过于频繁，请15分钟后再试")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	agents := h.cm.AllAgents()
	var agent *Agent
	for _, a := range agents {
		if a.Username == req.Username {
			agent = a
			break
		}
	}
	if agent == nil {
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if agent.Disabled {
		writeError(w, http.StatusForbidden, "账号已被禁用，请联系管理员")
		return
	}

	match, newHash := verifyPassword(req.Password, agent.Password)
	if !match {
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	// Auto-migrate legacy SHA-256 hash to bcrypt
	if newHash != "" {
		agent.Password = newHash
		h.cm.save()
		log.Printf("[AGENT] Agent %s password hash migrated to bcrypt", agent.Username)
	}

	h.loginLimiter.clear("login_" + ip)

	token := h.createSession()
	h.mapTokenToAgent(token, agent.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     "agent_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   28800,
	})

	log.Printf("[AGENT] Login: %s (%s)", agent.Username, agent.ID)
	writeOK(w, map[string]interface{}{
		"token":    token,
		"agent_id": agent.ID,
		"username": agent.Username,
	})
}

// ── Agent Dashboard ──────────────────────────────────────────────────────────

func (h *AgentHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	agentID := r.Header.Get("X-Agent-ID")
	total, active, expired := h.cm.AgentStats(agentID)
	writeOK(w, map[string]interface{}{
		"total_cards":   total,
		"active_cards":  active,
		"expired_cards": expired,
	})
}

func (h *AgentHandler) HandleListCards(w http.ResponseWriter, r *http.Request) {
	agentID := r.Header.Get("X-Agent-ID")
	writeOK(w, map[string]interface{}{"cards": h.cm.AgentCards(agentID)})
}

func (h *AgentHandler) HandleAgentInfo(w http.ResponseWriter, r *http.Request) {
	agentID := r.Header.Get("X-Agent-ID")
	agent := h.cm.GetAgent(agentID)
	if agent == nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	writeOK(w, map[string]interface{}{
		"agent": map[string]interface{}{
			"id":         agent.ID,
			"username":   agent.Username,
			"prefix":     agent.Prefix,
			"created_at": agent.CreatedAt,
		},
	})
}

// ── Agent Card Operations ────────────────────────────────────────────────────

func (h *AgentHandler) HandleGenerateCard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	agentID := r.Header.Get("X-Agent-ID")
	var req struct {
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	clampInt(&req.Duration, 1, 8760, 720)
	clampInt(&req.MaxSessions, 1, 5, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	card, err := h.cm.GenerateCard(time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.cm.mu.Lock()
	h.cm.auditLog = append(h.cm.auditLog, AuditEntry{
		Time: time.Now(), Action: "agent_card_generated", Card: card.Code, AgentID: agentID,
		Detail: fmt.Sprintf("duration=%d hours", req.Duration),
	})
	h.cm.mu.Unlock()
	h.cm.save()

	log.Printf("[AGENT] Generated card: %s by agent %s", card.Code, agentID)
	writeOK(w, map[string]interface{}{"card": card})
}

func (h *AgentHandler) HandleBatchGenerate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	agentID := r.Header.Get("X-Agent-ID")
	var req struct {
		Count       int    `json:"count"`
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	clampInt(&req.Count, 1, 50, 1)
	clampInt(&req.Duration, 1, 8760, 720)
	clampInt(&req.MaxSessions, 1, 5, 1)

	cards, err := h.cm.BatchGenerateCards(req.Count, time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.cm.mu.Lock()
	h.cm.auditLog = append(h.cm.auditLog, AuditEntry{
		Time: time.Now(), Action: "agent_batch_generated", AgentID: agentID,
		Detail: fmt.Sprintf("count=%d duration=%d hours", len(cards), req.Duration),
	})
	h.cm.mu.Unlock()
	h.cm.save()

	log.Printf("[AGENT] Batch generated %d cards by agent %s", len(cards), agentID)
	writeOK(w, map[string]interface{}{"cards": cards, "count": len(cards)})
}

// ── Agent Password Change ────────────────────────────────────────────────────

func (h *AgentHandler) HandlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	ip := getClientIP(r)
	if !h.pwLimiter.allow("pwchg_" + ip) {
		writeError(w, http.StatusTooManyRequests, "密码修改过于频繁，请稍后再试")
		return
	}

	agentID := r.Header.Get("X-Agent-ID")
	agent := h.cm.GetAgent(agentID)
	if agent == nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, "新密码长度不能少于6位")
		return
	}
	if len(req.NewPassword) > 128 {
		writeError(w, http.StatusBadRequest, "密码长度不能超过128位")
		return
	}

	match, _ := verifyPassword(req.OldPassword, agent.Password)
	if !match {
		writeError(w, http.StatusUnauthorized, "旧密码错误")
		return
	}

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "密码加密失败")
		return
	}
	h.cm.mu.Lock()
	agent.Password = string(bcryptHash)
	h.cm.auditLog = append(h.cm.auditLog, AuditEntry{
		Time: time.Now(), Action: "agent_password_changed", AgentID: agentID,
	})
	h.cm.mu.Unlock()
	h.cm.save()

	log.Printf("[AGENT] Password changed: %s", agentID)
	writeOK(w, map[string]interface{}{"message": "密码修改成功"})
}

// ── Admin-facing Agent Management ────────────────────────────────────────────

func (h *AdminHandler) HandleAdminListAgents(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]interface{}{"agents": h.cm.AllAgents()})
}

func (h *AdminHandler) HandleAdminUpdateAgent(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		AgentID  string `json:"agent_id"`
		Action   string `json:"action"`
		Password string `json:"password,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch req.Action {
	case "disable":
		if err := h.cm.UpdateAgentStatus(req.AgentID, true); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "enable":
		if err := h.cm.UpdateAgentStatus(req.AgentID, false); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "delete":
		if err := h.cm.DeleteAgent(req.AgentID); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "reset_password":
		if len(req.Password) < 6 {
			writeError(w, http.StatusBadRequest, "密码长度不能少于6位")
			return
		}
		agent := h.cm.GetAgent(req.AgentID)
		if agent == nil {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		bcryptHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "密码加密失败")
			return
		}
		h.cm.mu.Lock()
		agent.Password = string(bcryptHash)
		h.cm.auditLog = append(h.cm.auditLog, AuditEntry{
			Time: time.Now(), Action: "admin_reset_agent_password",
			Detail: fmt.Sprintf("agent_id=%s", req.AgentID),
		})
		h.cm.mu.Unlock()
		h.cm.save()
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}

	log.Printf("[ADMIN] Agent %s: %s", req.Action, req.AgentID)
	writeOK(w, nil)
}

func (h *AdminHandler) HandleAdminAgentCards(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	writeOK(w, map[string]interface{}{"cards": h.cm.AgentCards(agentID)})
}
