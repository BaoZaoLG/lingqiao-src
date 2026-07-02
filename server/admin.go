package main

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/features/admin"
	"github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/security/blacklist"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
)

// AdminHandler handles the web admin panel.
type AdminHandler struct {
	cardsSvc       *cards.Service
	sessionsSvc    *sessions.Service
	sessionHandler *sessions.Handler
	agentsSvc      *agents.Service
	inviteSvc      *agents.InviteService
	blacklistSvc   *blacklist.Service
	auditRecorder  *audit.Recorder

	adminSvc        *admin.Service
	db              *sql.DB
	loginLimiter    *httpapi.RateLimiter
	pwChangeLimiter *httpapi.RateLimiter
}

func NewAdminHandler(cardsSvc *cards.Service, sessionsSvc *sessions.Service, agentsSvc *agents.Service, inviteSvc *agents.InviteService, blacklistSvc *blacklist.Service, auditRecorder *audit.Recorder, db *sql.DB) *AdminHandler {
	return NewAdminHandlerWithTTL(cardsSvc, sessionsSvc, agentsSvc, inviteSvc, blacklistSvc, auditRecorder, db, currentSessionTTL())
}

func NewAdminHandlerWithTTL(cardsSvc *cards.Service, sessionsSvc *sessions.Service, agentsSvc *agents.Service, inviteSvc *agents.InviteService, blacklistSvc *blacklist.Service, auditRecorder *audit.Recorder, db *sql.DB, sessionTTL time.Duration) *AdminHandler {
	repo, _ := admin.NewRepositorySQLite(db)
	adminSvc := admin.NewService(repo, sessionTTL, dataDir())

	return &AdminHandler{
		cardsSvc:       cardsSvc,
		sessionsSvc:    sessionsSvc,
		sessionHandler: sessions.NewHandler(sessionsSvc, nil, nil, nil),
		agentsSvc:      agentsSvc,
		inviteSvc:      inviteSvc,
		blacklistSvc:   blacklistSvc,
		auditRecorder:  auditRecorder,

		adminSvc:        adminSvc,
		db:              db,
		loginLimiter:    httpapi.NewRateLimiter(15*time.Minute, 5),
		pwChangeLimiter: httpapi.NewRateLimiter(60*time.Minute, 10),
	}
}

func (h *AdminHandler) checkSession(r *http.Request) (*admin.AdminUser, bool) {
	token := extractToken(r, "admin_token")
	if token == "" {
		return nil, false
	}
	return h.adminSvc.CheckSession(r.Context(), token)
}

func (h *AdminHandler) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/api/login" {
			next(w, r)
			return
		}
		adminUser, ok := h.checkSession(r)
		if !ok {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), "adminUser", adminUser)
		next(w, r.WithContext(ctx))
	}
}

func (h *AdminHandler) RequirePermission(perm string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminUser, ok := r.Context().Value("adminUser").(*admin.AdminUser)
		if !ok || !admin.HasPermission(adminUser.Role, perm) {
			httpapi.WriteError(w, http.StatusForbidden, "forbidden: requires permission "+perm)
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
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Username == "" {
		req.Username = admin.DefaultOwnerUsername
	}

	adminUser, err := h.adminSvc.VerifyLogin(r.Context(), req.Username, req.Password)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	h.loginLimiter.Clear(ip)

	token, maxAge := h.adminSvc.CreateSession(r.Context(), adminUser.ID, ip, r.UserAgent())
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
	adminUser, ok := h.checkSession(r)
	if !ok {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.adminSvc.ChangePassword(r.Context(), adminUser.ID, req.OldPassword, req.NewPassword); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httpapi.WriteOK(w, map[string]interface{}{"message": "密码修改成功"})
}
