package admin

import (
	"context"
	"net/http"
	"sort"
	"sync/atomic"
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/security/blacklist"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
)

type Handler struct {
	svc             *Service
	cardsSvc        *cards.Service
	sessionsSvc     *sessions.Service
	sessionHandler  *sessions.Handler
	agentsSvc       *agents.Service
	inviteSvc       *agents.InviteService
	blacklistSvc    *blacklist.Service
	auditRecorder   *audit.Recorder
	serverStart     time.Time
	requestCount    *atomic.Int64
	loginLimiter    *httpapi.RateLimiter
	pwChangeLimiter *httpapi.RateLimiter
}

func NewHandler(svc *Service, cardsSvc *cards.Service, sessionsSvc *sessions.Service, agentsSvc *agents.Service, inviteSvc *agents.InviteService, blacklistSvc *blacklist.Service, auditRecorder *audit.Recorder, serverStart time.Time, requestCount *atomic.Int64) *Handler {
	return &Handler{
		svc:             svc,
		cardsSvc:        cardsSvc,
		sessionsSvc:     sessionsSvc,
		sessionHandler:  sessions.NewHandler(sessionsSvc, nil, nil, nil),
		agentsSvc:       agentsSvc,
		inviteSvc:       inviteSvc,
		blacklistSvc:    blacklistSvc,
		auditRecorder:   auditRecorder,
		serverStart:     serverStart,
		requestCount:    requestCount,
		loginLimiter:    httpapi.NewRateLimiter(15*time.Minute, 5),
		pwChangeLimiter: httpapi.NewRateLimiter(60*time.Minute, 10),
	}
}

// AdminAuth is a middleware that protects admin routes.
func (h *Handler) AdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/api/login" {
			next(w, r)
			return
		}

		token := h.extractToken(r, "admin_token")
		adminUser, ok := h.svc.CheckSession(r.Context(), token)
		if !ok {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// Save adminUser in request context for later
		ctx := context.WithValue(r.Context(), "adminUser", adminUser)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) extractToken(r *http.Request, cookieName string) string {
	if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	ip := httpapi.GetClientIP(r)
	if !h.loginLimiter.Allow(ip) {
		httpapi.WriteError(w, http.StatusTooManyRequests, "登录尝试过于频繁，请15分钟后再试")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	adminUser, err := h.svc.VerifyLogin(r.Context(), req.Username, req.Password)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	h.loginLimiter.Clear(ip)

	token, maxAge := h.svc.CreateSession(r.Context(), adminUser.ID, ip, r.UserAgent())
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	})

	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "token": token})
}

func (h *Handler) HandlePasswordChange(w http.ResponseWriter, r *http.Request) {
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

	// Get admin ID from session
	token := h.extractToken(r, "admin_token")
	adminUser, ok := h.svc.CheckSession(r.Context(), token)
	if !ok {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.svc.ChangePassword(r.Context(), adminUser.ID, req.OldPassword, req.NewPassword); err != nil {
		if err.Error() == "旧密码错误" {
			httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		} else {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	httpapi.WriteOK(w, map[string]interface{}{"message": "密码修改成功"})
}

func (h *Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	cardsList, err := h.cardsSvc.AllCards(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load cards: "+err.Error())
		return
	}

	now := time.Now()
	var activeCards, expiredCards int
	for _, c := range cardsList {
		if c.Status == cards.StatusActive && now.Before(c.ExpiresAt) {
			activeCards++
		}
		if now.After(c.ExpiresAt) || c.Status == cards.StatusExpired {
			expiredCards++
		}
	}

	activeSessions := 0
	if res, err := h.sessionsSvc.List(r.Context(), sessions.ListFilter{ActiveOnly: true}); err == nil {
		activeSessions = res.Total
	}

	httpapi.WriteOK(w, map[string]interface{}{
		"total_cards":     len(cardsList),
		"active_cards":    activeCards,
		"expired_cards":   expiredCards,
		"active_sessions": activeSessions,
	})
}

func (h *Handler) HandleServerStats(w http.ResponseWriter, r *http.Request) {
	activeCount := 0
	if res, err := h.sessionsSvc.List(r.Context(), sessions.ListFilter{ActiveOnly: true}); err == nil {
		activeCount = res.Total
	}

	cardsList, err := h.cardsSvc.AllCards(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load cards: "+err.Error())
		return
	}

	var reqCount int64
	if h.requestCount != nil {
		reqCount = h.requestCount.Load()
	}

	httpapi.WriteOK(w, map[string]interface{}{
		"uptime_seconds":  int(time.Since(h.serverStart).Seconds()),
		"total_cards":     len(cardsList),
		"active_sessions": activeCount,
		"request_count":   reqCount,
	})
}

func (h *Handler) HandleOpsOverview(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cardsList, err := h.cardsSvc.AllCards(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load cards: "+err.Error())
		return
	}
	auditLog, err := h.auditRecorder.Query(r.Context(), audit.Filter{}, 10000)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to query audit log: "+err.Error())
		return
	}

	var todayActivations int
	newMachines := make(map[string]struct{})
	recentAudit := make([]audit.Event, 0, 8)
	for _, entry := range auditLog {
		if entry.Time.After(dayStart) && entry.Action == "card_activated" {
			todayActivations++
			if entry.Machine != "" {
				newMachines[entry.Machine] = struct{}{}
			}
		}
	}
	for i := 0; i < len(auditLog) && i < 8; i++ {
		recentAudit = append(recentAudit, auditLog[i])
	}

	expiring := make([]cards.Card, 0)
	inThreeDays := now.Add(72 * time.Hour)
	for _, c := range cardsList {
		if c.Status == cards.StatusActive && c.ExpiresAt.After(now) && c.ExpiresAt.Before(inThreeDays) {
			expiring = append(expiring, c)
		}
	}
	sort.Slice(expiring, func(i, j int) bool {
		return expiring[i].ExpiresAt.Before(expiring[j].ExpiresAt)
	})
	if len(expiring) > 10 {
		expiring = expiring[:10]
	}
	agentCards := make(map[string]int)
	for _, c := range cardsList {
		if c.Status == cards.StatusUnused && c.AgentID != "" {
			agentCards[c.AgentID]++
		}
	}
	httpapi.WriteOK(w, map[string]interface{}{
		"today_activations":    todayActivations,
		"today_new_machines":   len(newMachines),
		"expiring_soon_cards":  expiring,
		"recent_activity":      recentAudit,
		"agent_card_inventory": agentCards,
	})
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/api/login", h.HandleLogin)
	mux.HandleFunc("/admin/api/password", h.AdminAuth(h.HandlePasswordChange))
	mux.HandleFunc("/admin/api/dashboard", h.AdminAuth(h.HandleDashboard))
	mux.HandleFunc("/admin/api/stats", h.AdminAuth(h.HandleServerStats))
	mux.HandleFunc("/admin/api/overview", h.AdminAuth(h.HandleOpsOverview))
}
