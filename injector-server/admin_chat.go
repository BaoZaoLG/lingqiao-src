package main

import (
	"net/http"
	"time"
)

func (h *AdminHandler) HandleChatMessages(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if h.chat == nil {
		writeError(w, http.StatusInternalServerError, "chat service unavailable")
		return
	}
	writeOK(w, map[string]interface{}{"messages": h.chat.AdminList()})
}

func (h *AdminHandler) HandleChatDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if h.chat == nil {
		writeError(w, http.StatusInternalServerError, "chat service unavailable")
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ID <= 0 {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if !h.chat.Delete(req.ID) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	writeOK(w, nil)
}

func (h *AdminHandler) HandleChatSystem(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if h.chat == nil {
		writeError(w, http.StatusInternalServerError, "chat service unavailable")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	msg, err := h.chat.AddSystem(req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"message": msg})
}

func (h *AdminHandler) HandleChatMute(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if h.chat == nil {
		writeError(w, http.StatusInternalServerError, "chat service unavailable")
		return
	}
	var req struct {
		AuthorID        string `json:"author_id"`
		DurationMinutes int    `json:"duration_minutes"`
		Reason          string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	duration := time.Duration(req.DurationMinutes) * time.Minute
	mute, err := h.chat.Mute(req.AuthorID, duration, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"mute": mute})
}
