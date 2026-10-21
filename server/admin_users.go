package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lingqiao/server/internal/features/admin"
	"github.com/lingqiao/server/internal/httpapi"
)

// HandleAdminUsersList returns a list of all admin users
func (h *AdminHandler) HandleAdminUsersList(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	admins, err := h.adminSvc.ListAdmins(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list admins")
		return
	}

	// Filter out password hashes
	for i := range admins {
		admins[i].PasswordHash = ""
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"admins": admins,
	})
}

// HandleAdminUserCreate creates a new admin user
func (h *AdminHandler) HandleAdminUserCreate(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Username string     `json:"username"`
		Password string     `json:"password"`
		Role     admin.Role `json:"role"`
		Disabled bool       `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" || req.Role == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "username, password, and role are required")
		return
	}

	newAdmin := &admin.AdminUser{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: req.Password,
		Role:         req.Role,
		Disabled:     req.Disabled,
		CreatedAt:    time.Now(),
	}

	if err := h.adminSvc.CreateAdmin(r.Context(), newAdmin); err != nil {
		if errors.Is(err, admin.ErrInvalidAdminRole) {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to create admin")
		return
	}

	newAdmin.PasswordHash = ""
	httpapi.WriteJSON(w, http.StatusOK, newAdmin)
}

// HandleAdminUserUpdate updates an existing admin user
func (h *AdminHandler) HandleAdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		ID             string     `json:"id"`
		ChangePassword bool       `json:"change_password"`
		NewPassword    string     `json:"new_password"`
		ChangeRole     bool       `json:"change_role"`
		Role           admin.Role `json:"role"`
		SetDisabled    bool       `json:"set_disabled"`
		Disabled       bool       `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	targetAdmin, err := h.adminSvc.GetAdmin(r.Context(), req.ID)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "Admin not found")
		return
	}

	if req.ChangePassword {
		if len(req.NewPassword) < admin.MinPasswordBytes || len(req.NewPassword) > admin.MaxPasswordBytes {
			httpapi.WriteError(w, http.StatusBadRequest, "password length must be between 6 and 72 bytes")
			return
		}
		passwordHash, err := h.adminSvc.HashPassword(req.NewPassword)
		if err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		targetAdmin.PasswordHash = passwordHash
	}

	if req.ChangeRole {
		targetAdmin.Role = req.Role
	}

	if req.SetDisabled {
		targetAdmin.Disabled = req.Disabled
	}

	if err := h.adminSvc.UpdateAdmin(r.Context(), targetAdmin); err != nil {
		if errors.Is(err, admin.ErrInvalidAdminRole) || errors.Is(err, admin.ErrLastActiveOwner) {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to update admin")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}
