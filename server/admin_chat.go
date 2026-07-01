package main

import (
	"github.com/lingqiao/server/internal/httpapi"

	"net/http"
	"time"
)

// HandleChatMessages ...
func (h *AdminHandler) HandleChatMessages(w http.ResponseWriter, r *http.Request) {
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
func (h *AdminHandler) HandleChatDelete(w http.ResponseWriter, r *http.Request) {
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
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
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
func (h *AdminHandler) HandleChatSystem(w http.ResponseWriter, r *http.Request) {
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
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	msg, err := h.chat.AddSystem(req.Content)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"message": msg})
}

// HandleChatMute ...
func (h *AdminHandler) HandleChatMute(w http.ResponseWriter, r *http.Request) {
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
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	duration := time.Duration(req.DurationMinutes) * time.Minute
	mute, err := h.chat.Mute(req.AuthorID, duration, req.Reason)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"mute": mute})
}
