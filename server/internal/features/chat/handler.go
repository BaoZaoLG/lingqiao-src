package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/security/blacklist"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
)

type Handler struct {
	chat         *Service
	sessionsSvc  *sessions.Service
	blacklistSvc *blacklist.Service
}

func NewHandler(chat *Service, sessionsSvc *sessions.Service, blacklistSvc *blacklist.Service) *Handler {
	return &Handler{
		chat:         chat,
		sessionsSvc:  sessionsSvc,
		blacklistSvc: blacklistSvc,
	}
}

type chatRequest struct {
	SessionToken string `json:"session_token"`
	MachineID    string `json:"machine_id"`
	Card         string `json:"card"`
	Content      string `json:"content,omitempty"`
	ReplyToID    int64  `json:"reply_to_id,omitempty"`
	Nickname     string `json:"nickname,omitempty"`
	MessageID    int64  `json:"message_id,omitempty"`
	Reaction     string `json:"reaction,omitempty"`
	AfterID      int64  `json:"after_id,omitempty"`
}

func (h *Handler) readSignedRequest(r *http.Request) (*chatRequest, error) {
	body, err := httpapi.ReadBody(r)
	if err != nil {
		return nil, err
	}

	if err := auth.VerifyRequest(r, string(body)); err != nil {
		return nil, err
	}

	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	if h.blacklistSvc != nil {
		isBlacklisted, err := h.blacklistSvc.IsBlacklisted(r.Context(), req.MachineID)
		if err != nil {
			return nil, fmt.Errorf("failed to check blacklist status: %v", err)
		}
		if isBlacklisted {
			return nil, fmt.Errorf("machine is blacklisted")
		}
	}
	return &req, nil
}

func (h *Handler) HandleChatSend(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	domainSession, err := h.sessionsSvc.ValidateBound(r.Context(), req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	msg, err := h.chat.Add(&domainSession, req.Content, req.ReplyToID)
	if err != nil {
		if errors.Is(err, ErrMuted) {
			httpapi.WriteError(w, http.StatusForbidden, "你已被禁言，暂时无法发送消息")
			return
		}
		if errors.Is(err, ErrRateLimited) {
			httpapi.WriteError(w, http.StatusTooManyRequests, "发送过于频繁，请稍后再试")
			return
		}
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"message": msg})
}

func (h *Handler) HandleChatProfile(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	domainSession, err := h.sessionsSvc.ValidateBound(r.Context(), req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var profile ChatProfile
	if req.Nickname != "" {
		profile, err = h.chat.SetProfile(&domainSession, req.Nickname)
	} else {
		profile, err = h.chat.Profile(&domainSession)
	}
	if err != nil {
		if errors.Is(err, ErrMuted) {
			httpapi.WriteError(w, http.StatusForbidden, "你已被禁言，暂时无法修改昵称")
			return
		}
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"profile": profile})
}

func (h *Handler) HandleChatReact(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	domainSession, err := h.sessionsSvc.ValidateBound(r.Context(), req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	msg, err := h.chat.React(&domainSession, req.MessageID, req.Reaction)
	if err != nil {
		if errors.Is(err, ErrMuted) {
			httpapi.WriteError(w, http.StatusForbidden, "你已被禁言，暂时无法添加表情")
			return
		}
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"message": msg})
}

func (h *Handler) HandleChatPresence(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	domainSession, err := h.sessionsSvc.ValidateBound(r.Context(), req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	online, err := h.chat.TouchPresence(&domainSession)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"online": online})
}

func (h *Handler) HandleChatMessages(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	domainSession, err := h.sessionsSvc.ValidateBound(r.Context(), req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"messages": h.chat.List(req.AfterID, &domainSession)})
}

// HandleAdminChatMessages ...
func (h *Handler) HandleAdminChatMessages(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	if h.chat == nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "chat service unavailable")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"messages": h.chat.AdminList()})
}

// HandleChatDelete ...
func (h *Handler) HandleChatDelete(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	if h.chat == nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "chat service unavailable")
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID <= 0 {
		httpapi.WriteError(w, http.StatusBadRequest, "id is required")
		return
	}
	if !h.chat.Delete(req.ID) {
		httpapi.WriteError(w, http.StatusNotFound, "message not found")
		return
	}
	httpapi.WriteOK(w, nil)
}

// HandleChatSystem ...
func (h *Handler) HandleChatSystem(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	if h.chat == nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "chat service unavailable")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	msg, err := h.chat.AddSystem(req.Content)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"message": msg})
}

// HandleChatMute ...
func (h *Handler) HandleChatMute(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	if h.chat == nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "chat service unavailable")
		return
	}
	var req struct {
		AuthorID        string `json:"author_id"`
		DurationMinutes int    `json:"duration_minutes"`
		Reason          string `json:"reason"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	duration := time.Duration(req.DurationMinutes) * time.Minute
	mute, err := h.chat.Mute(req.AuthorID, duration, req.Reason)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"mute": mute})
}
