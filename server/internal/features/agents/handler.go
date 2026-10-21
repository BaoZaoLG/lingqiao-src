package agents

import (
	"github.com/lingqiao/server/internal/httpapi"

	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CardIssuer defines card generation capabilities required by agent handlers.
type CardIssuer interface {
	GenerateCard(duration time.Duration, note string, maxSessions int, agentID string) (CardSnapshot, error)
	BatchGenerateCards(count int, duration time.Duration, note string, maxSessions int, agentID string) ([]CardSnapshot, error)
}

// CardReader defines card querying capabilities required by agent handlers.
type CardReader interface {
	AllCards() []CardSnapshot
	AgentCards(agentID string) []CardSnapshot
	AgentStats(agentID string) (total, active, expired int)
}

type CardSnapshot struct {
	Code        string     `json:"code"`
	MachineID   string     `json:"machine_id,omitempty"`
	AgentID     string     `json:"agent_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Status      string     `json:"status"`
	Note        string     `json:"note,omitempty"`
	MaxSessions int        `json:"max_sessions"`
}

func clampInt(val *int, min, max, def int) {
	if *val == 0 {
		*val = def
	} else if *val < min {
		*val = min
	} else if *val > max {
		*val = max
	}
}

// AgentHandler handles the agent panel API, completely isolated from admin.
type AgentHandler struct {
	svc          *Service
	issuer       CardIssuer
	reader       CardReader
	inviteSvc    *InviteService
	mu           sync.Mutex
	sessions     map[string]time.Time
	sessionTTL   time.Duration
	fileMu       sync.Mutex // protects JSON file reads/writes for token map
	loginLimiter *httpapi.RateLimiter
	regLimiter   *httpapi.RateLimiter
	pwLimiter    *httpapi.RateLimiter
	dataDir      string
	auditSink    func(action, cardCode, agentID, detail string)
}

func (h *AgentHandler) SetAuditSink(sink func(action, cardCode, agentID, detail string)) {
	h.auditSink = sink
}

func NewAgentHandler(svc *Service, issuer CardIssuer, reader CardReader, inviteSvc *InviteService, sessionTTL time.Duration, dataDir string) *AgentHandler {
	if sessionTTL <= 0 {
		sessionTTL = 4 * time.Hour
	}
	h := &AgentHandler{
		svc:          svc,
		issuer:       issuer,
		reader:       reader,
		inviteSvc:    inviteSvc,
		sessions:     make(map[string]time.Time),
		sessionTTL:   sessionTTL,
		loginLimiter: httpapi.NewRateLimiter(15*time.Minute, 5),
		regLimiter:   httpapi.NewRateLimiter(1*time.Hour, 3),
		pwLimiter:    httpapi.NewRateLimiter(1*time.Hour, 3),
		dataDir:      dataDir,
	}
	h.loadSessions()
	return h
}

func (h *AgentHandler) dataPath(filename string) string {
	return filepath.Join(h.dataDir, filename)
}

func (h *AgentHandler) loadSessions() {
	data, err := os.ReadFile(h.dataPath("agent_sessions.json"))
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
	os.WriteFile(h.dataPath("agent_sessions.json"), data, 0600)
}

func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func (h *AgentHandler) createSession() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("[AGENT] Failed to generate session token: %v", err)
		return "", err
	}
	token := "agent_" + hex.EncodeToString(b)
	h.mu.Lock()
	h.sessions[hashToken(token)] = time.Now().Add(h.sessionTTL)
	h.mu.Unlock()
	h.saveSessions()
	return token, nil
}

func extractToken(r *http.Request, cookieName string) string {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		return c.Value
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
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
	if agentID == "" {
		return "", false
	}
	agent, err := h.svc.GetAgent(r.Context(), agentID)
	if err != nil || agent == nil || agent.Disabled {
		return "", false
	}
	return agentID, true
}

func (h *AgentHandler) getAgentIDForToken(token string) string {
	h.fileMu.Lock()
	defer h.fileMu.Unlock()
	data, err := os.ReadFile(h.dataPath("agent_token_map.json"))
	if err != nil {
		return ""
	}
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m[hashToken(token)]
}

func (h *AgentHandler) mapTokenToAgent(token, agentID string) {
	h.fileMu.Lock()
	defer h.fileMu.Unlock()
	data, _ := os.ReadFile(h.dataPath("agent_token_map.json"))
	m := make(map[string]string)
	json.Unmarshal(data, &m)
	m[hashToken(token)] = agentID
	d, _ := json.Marshal(m)
	os.WriteFile(h.dataPath("agent_token_map.json"), d, 0600)
}

func (h *AgentHandler) agentAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID, ok := h.checkSession(r)
		if !ok {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		r.Header.Set("X-Agent-ID", agentID)
		next(w, r)
	}
}

func (h *AgentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/login", h.HandleLogin)
	mux.HandleFunc("/api/register", h.HandleRegister)
	mux.HandleFunc("/api/dashboard", h.agentAuth(h.HandleDashboard))
	mux.HandleFunc("/api/cards", h.agentAuth(h.HandleListCards))
	mux.HandleFunc("/api/card/generate", h.agentAuth(h.HandleGenerateCard))
	mux.HandleFunc("/api/card/batch-generate", h.agentAuth(h.HandleBatchGenerate))
	mux.HandleFunc("/api/info", h.agentAuth(h.HandleAgentInfo))
	mux.HandleFunc("/api/password", h.agentAuth(h.HandlePasswordChange))
}

func (h *AgentHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	ip := httpapi.GetClientIP(r)
	if !h.regLimiter.Allow("reg_" + ip) {
		httpapi.WriteError(w, http.StatusTooManyRequests, "注册过于频繁，请稍后再试")
		return
	}

	var req struct {
		InviteCode string `json:"invite_code"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 || len(req.Username) > 20 {
		httpapi.WriteError(w, http.StatusBadRequest, "用户名长度必须在3-20位之间")
		return
	}

	if len(req.Password) < 6 || len(req.Password) > 72 {
		httpapi.WriteError(w, http.StatusBadRequest, "密码长度必须在6-72位之间")
		return
	}

	for _, c := range req.Username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			httpapi.WriteError(w, http.StatusBadRequest, "用户名只能包含字母、数字和下划线")
			return
		}
	}

	// 1. Call the transactional RegisterAgent method
	agent, err := h.svc.RegisterAgent(r.Context(), req.Username, req.Password, "", req.InviteCode, h.inviteSvc)
	if err != nil {
		if errors.Is(err, ErrUsernameExists) {
			httpapi.WriteError(w, http.StatusConflict, "用户名已存在")
		} else if errors.Is(err, ErrPasswordTooLong) {
			httpapi.WriteError(w, http.StatusBadRequest, "密码长度不能超过72位")
		} else if strings.Contains(err.Error(), "邀请码") {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			log.Printf("[AGENT] register failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "注册失败，请稍后重试")
		}
		return
	}

	log.Printf("[AGENT] Registered: %s (%s) via invite %s", agent.Username, agent.ID, req.InviteCode)
	httpapi.WriteOK(w, map[string]interface{}{
		"agent_id": agent.ID,
		"username": agent.Username,
	})
}

func (h *AgentHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	ip := httpapi.GetClientIP(r)
	if !h.loginLimiter.Allow("login_" + ip) {
		httpapi.WriteError(w, http.StatusTooManyRequests, "登录尝试过于频繁，请15分钟后再试")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	agent, migrated, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, ErrAgentDisabled) {
			httpapi.WriteError(w, http.StatusForbidden, "账号已被禁用，请联系管理员")
		} else {
			httpapi.WriteError(w, http.StatusUnauthorized, "用户名或密码错误")
		}
		return
	}

	if migrated {
		log.Printf("[AGENT] Agent %s password hash migrated to bcrypt", agent.Username)
	}

	h.loginLimiter.Clear("login_" + ip)

	token, err := h.createSession()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "登录失败，请重试")
		return
	}
	h.mapTokenToAgent(token, agent.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     "agent_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.sessionTTL.Seconds()),
	})

	log.Printf("[AGENT] Login: %s (%s)", agent.Username, agent.ID)
	httpapi.WriteOK(w, map[string]interface{}{
		"token":    token,
		"agent_id": agent.ID,
		"username": agent.Username,
	})
}

func (h *AgentHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	agentID := r.Header.Get("X-Agent-ID")
	total, active, expired := h.reader.AgentStats(agentID)
	httpapi.WriteOK(w, map[string]interface{}{
		"total_cards":   total,
		"active_cards":  active,
		"expired_cards": expired,
	})
}

func (h *AgentHandler) HandleListCards(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	agentID := r.Header.Get("X-Agent-ID")
	httpapi.WriteOK(w, map[string]interface{}{"cards": h.reader.AgentCards(agentID)})
}

func (h *AgentHandler) HandleAgentInfo(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	agentID := r.Header.Get("X-Agent-ID")
	agent, err := h.svc.GetAgent(r.Context(), agentID)
	if err != nil || agent == nil {
		httpapi.WriteError(w, http.StatusNotFound, "agent not found")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{
		"agent": map[string]interface{}{
			"id":         agent.ID,
			"username":   agent.Username,
			"prefix":     agent.Prefix,
			"created_at": agent.CreatedAt,
		},
	})
}

func (h *AgentHandler) HandleGenerateCard(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	agentID := r.Header.Get("X-Agent-ID")
	var req struct {
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clampInt(&req.Duration, 1, 8760, 720)
	clampInt(&req.MaxSessions, 1, 5, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	card, err := h.issuer.GenerateCard(time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, agentID)
	if err != nil {
		log.Printf("[AGENT] generate card failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to generate card")
		return
	}

	if h.auditSink != nil {
		h.auditSink("agent_card_generated", card.Code, agentID, fmt.Sprintf("duration=%d hours", req.Duration))
	}

	log.Printf("[AGENT] Generated card: %s by agent %s", card.Code, agentID)
	httpapi.WriteOK(w, map[string]interface{}{"card": card})
}

func (h *AgentHandler) HandleBatchGenerate(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	agentID := r.Header.Get("X-Agent-ID")
	var req struct {
		Count       int    `json:"count"`
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clampInt(&req.Count, 1, 50, 1)
	clampInt(&req.Duration, 1, 8760, 720)
	clampInt(&req.MaxSessions, 1, 5, 1)

	cards, err := h.issuer.BatchGenerateCards(req.Count, time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, agentID)
	if err != nil {
		log.Printf("[AGENT] batch generate failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to batch generate cards")
		return
	}

	if h.auditSink != nil {
		h.auditSink("agent_batch_generated", "", agentID, fmt.Sprintf("count=%d duration=%d hours", len(cards), req.Duration))
	}

	log.Printf("[AGENT] Batch generated %d cards by agent %s", len(cards), agentID)
	httpapi.WriteOK(w, map[string]interface{}{"cards": cards, "count": len(cards)})
}

func (h *AgentHandler) HandlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	ip := httpapi.GetClientIP(r)
	if !h.pwLimiter.Allow("pwchg_" + ip) {
		httpapi.WriteError(w, http.StatusTooManyRequests, "密码修改过于频繁，请稍后再试")
		return
	}

	agentID := r.Header.Get("X-Agent-ID")

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.NewPassword) < 6 || len(req.NewPassword) > 72 {
		httpapi.WriteError(w, http.StatusBadRequest, "密码长度必须在6-72位之间")
		return
	}

	err := h.svc.ChangePassword(r.Context(), agentID, req.OldPassword, req.NewPassword)
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			httpapi.WriteError(w, http.StatusUnauthorized, "旧密码错误")
		} else {
			log.Printf("[AGENT] password change failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "密码修改失败，请稍后重试")
		}
		return
	}

	log.Printf("[AGENT] Password changed: %s", agentID)
	httpapi.WriteOK(w, map[string]interface{}{"message": "密码修改成功"})
}
