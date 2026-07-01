package main

import (
	"github.com/lingqiao/server/internal/httpapi"

	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/sessions"
)

// AdminHandler handles the web admin panel.
type AdminHandler struct {
	cm              CardManager
	cardsSvc        *cards.Service
	sessionsSvc     *sessions.Service
	sessionHandler  *sessions.Handler
	agentsSvc       *agents.Service
	inviteSvc       *agents.InviteService
	chat            *ChatStore
	adminPassHash   string
	mu              sync.Mutex
	sessions        map[string]time.Time
	sessionTTL      time.Duration
	loginLimiter    *httpapi.RateLimiter
	pwChangeLimiter *httpapi.RateLimiter
}

func NewAdminHandler(cm CardManager, cardsSvc *cards.Service, sessionsSvc *sessions.Service, agentsSvc *agents.Service, inviteSvc *agents.InviteService) *AdminHandler {
	return NewAdminHandlerWithTTL(cm, cardsSvc, sessionsSvc, agentsSvc, inviteSvc, currentSessionTTL())
}

func NewAdminHandlerWithTTL(cm CardManager, cardsSvc *cards.Service, sessionsSvc *sessions.Service, agentsSvc *agents.Service, inviteSvc *agents.InviteService, sessionTTL time.Duration) *AdminHandler {
	if sessionTTL <= 0 {
		sessionTTL = 4 * time.Hour
	}
	h := &AdminHandler{
		cm:              cm,
		cardsSvc:        cardsSvc,
		sessionsSvc:     sessionsSvc,
		sessionHandler:  sessions.NewHandler(sessionsSvc, nil, nil, nil),
		agentsSvc:       agentsSvc,
		inviteSvc:       inviteSvc,
		chat:            NewChatStore(cm.GetStorage(), time.Now),
		sessions:        make(map[string]time.Time),
		sessionTTL:      sessionTTL,
		loginLimiter:    httpapi.NewRateLimiter(15*time.Minute, 5),
		pwChangeLimiter: httpapi.NewRateLimiter(60*time.Minute, 10),
	}

	// Load persisted password hash
	if savedHash, err := os.ReadFile(dataPath("admin_password.hash")); err == nil {
		trimmed := strings.TrimSpace(string(savedHash))
		if isBcryptHash(trimmed) || len(trimmed) == 64 {
			h.adminPassHash = trimmed
			h.loadSessions()
			return h
		}
	}

	pass := os.Getenv("ADMIN_PASSWORD")
	if pass == "" {
		// Generate an initial password only when no persisted hash exists.
		b := make([]byte, 16)
		rand.Read(b)
		pass = hex.EncodeToString(b)
		log.Printf("[ADMIN] No persisted admin password or ADMIN_PASSWORD set. Generated random password: %s****", pass[:4])
		log.Println("[ADMIN] Set ADMIN_PASSWORD env var to use a fixed password.")
	}

	h.adminPassHash = hashPassword(pass)
	os.MkdirAll(dataDir(), 0755)
	os.WriteFile(dataPath("admin_password.hash"), []byte(h.adminPassHash), 0600)
	h.loadSessions()
	return h
}

func (h *AdminHandler) loadSessions() {
	data, err := os.ReadFile(dataPath("admin_sessions.json"))
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

func (h *AdminHandler) saveSessions() {
	h.mu.Lock()
	data, _ := json.Marshal(h.sessions)
	h.mu.Unlock()
	os.WriteFile(dataPath("admin_sessions.json"), data, 0600)
}

func (h *AdminHandler) createSession() string {
	b := make([]byte, 32)
	rand.Read(b)
	token := "admin_" + hex.EncodeToString(b)
	h.mu.Lock()
	h.sessions[hashToken(token)] = time.Now().Add(h.sessionTTL)
	h.mu.Unlock()
	h.saveSessions()
	return token
}

func (h *AdminHandler) checkSession(r *http.Request) bool {
	token := extractToken(r, "admin_token")
	if token == "" {
		return false
	}
	h.mu.Lock()
	expiry, exists := h.sessions[hashToken(token)]
	h.mu.Unlock()
	return exists && time.Now().Before(expiry)
}

func (h *AdminHandler) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/api/login" {
			next(w, r)
			return
		}
		if !h.checkSession(r) {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

// ── Auth Handlers ────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	ip := httpapi.GetClientIP(r)
	if !h.loginLimiter.Allow(ip) {
		httpapi.WriteError(w, http.StatusTooManyRequests, "登录尝试过于频繁，请15分钟后再试")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	match, newHash := verifyPassword(req.Password, h.adminPassHash)
	if !match {
		httpapi.WriteError(w, http.StatusUnauthorized, "wrong password")
		return
	}
	// Auto-migrate legacy SHA-256 hash to bcrypt on successful login
	if newHash != "" {
		h.adminPassHash = newHash
		os.MkdirAll(dataDir(), 0755)
		os.WriteFile(dataPath("admin_password.hash"), []byte(h.adminPassHash), 0600)
		log.Printf("[ADMIN] Password hash migrated from SHA-256 to bcrypt")
	}

	h.loginLimiter.Clear(ip)

	token := h.createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.sessionTTL.Seconds()),
	})

	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "token": token})
}

func (h *AdminHandler) HandlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	ip := httpapi.GetClientIP(r)
	if !h.pwChangeLimiter.Allow("pwchg_" + ip) {
		httpapi.WriteError(w, http.StatusTooManyRequests, "密码修改过于频繁，请稍后再试")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.NewPassword) < 6 {
		httpapi.WriteError(w, http.StatusBadRequest, "新密码长度不能少于6位")
		return
	}
	if len(req.NewPassword) > 128 {
		httpapi.WriteError(w, http.StatusBadRequest, "密码长度不能超过128位")
		return
	}
	match, _ := verifyPassword(req.OldPassword, h.adminPassHash)
	if !match {
		httpapi.WriteError(w, http.StatusUnauthorized, "旧密码错误")
		return
	}

	h.adminPassHash = hashPassword(req.NewPassword)
	os.MkdirAll(dataDir(), 0755)
	os.WriteFile(dataPath("admin_password.hash"), []byte(h.adminPassHash), 0600)
	log.Printf("[ADMIN] Admin password changed and persisted")
	httpapi.WriteOK(w, map[string]interface{}{"message": "密码修改成功"})
}
