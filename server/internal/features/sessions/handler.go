package sessions

import (
	"github.com/lingqiao/server/internal/httpapi"

	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// SignedRequestReader provides a way to read and verify signed client requests
type SignedRequestReader interface {
	ReadSignedRequest(r *http.Request) (ClientRequest, error)
}

// VersionChecker provides a way to check if a client version is allowed
type VersionChecker interface {
	CheckVersion(w http.ResponseWriter, req ClientRequest) (bool, map[string]interface{})
}

// ClientIPFunc extracts the remote IP from a request
type ClientIPFunc func(r *http.Request) string

// ClientRequest contains standard authenticated payload fields
type ClientRequest struct {
	Card          string
	MachineID     string
	Fingerprint   string
	ClientVersion string
	SessionToken  string
}

// Handler handles HTTP requests for sessions
type Handler struct {
	svc          *Service
	reader       SignedRequestReader
	versionCheck VersionChecker
	clientIP     ClientIPFunc
}

// NewHandler creates a new sessions HTTP handler
func NewHandler(svc *Service, reader SignedRequestReader, versionCheck VersionChecker, clientIP ClientIPFunc) *Handler {
	return &Handler{
		svc:          svc,
		reader:       reader,
		versionCheck: versionCheck,
		clientIP:     clientIP,
	}
}

// writeCompatOK writes a standard JSON 200 OK response with top-level "status": "ok"
func writeCompatOK(w http.ResponseWriter, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["status"] = "ok"
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeError writes a standard JSON error response
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    status,
		"message": message,
	})
}

// writeJSON writes a standard JSON response without code/message wrapping
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// requireMethod ensures the request uses the expected method
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

// readJSON is a helper to read JSON payloads
func readJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func (h *Handler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	req, err := h.reader.ReadSignedRequest(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if req.Card == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "card is required")
		return
	}

	if blocked, _ := h.versionCheck.CheckVersion(w, req); blocked {
		return
	}

	session, card, err := h.svc.Activate(r.Context(), ActivationInput{
		CardCode:      req.Card,
		MachineID:     req.MachineID,
		Fingerprint:   req.Fingerprint,
		ClientVersion: req.ClientVersion,
		RemoteAddr:    h.clientIP(r),
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrCardNotFound) || errors.Is(err, ErrCardDisabled) || errors.Is(err, ErrCardExpired) {
			status = http.StatusForbidden
		} else if errors.Is(err, ErrMaxSessionsReached) || errors.Is(err, ErrBoundToOtherMachine) || errors.Is(err, ErrCardNoLongerValid) || errors.Is(err, ErrCardBlacklisted) {
			status = http.StatusForbidden
		}
		httpapi.WriteError(w, status, err.Error())
		return
	}

	_, updateInfo := h.versionCheck.CheckVersion(w, req)
	resp := map[string]interface{}{
		"session_token":   session.Token,
		"expires_at":      session.ExpiresAt.Unix(),
		"card_expires_at": card.ExpiresAt.Unix(),
		"message":         "激活成功",
	}
	if updateInfo != nil {
		resp["update"] = updateInfo
	}
	writeCompatOK(w, resp)
}

func (h *Handler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	req, err := h.reader.ReadSignedRequest(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if blocked, _ := h.versionCheck.CheckVersion(w, req); blocked {
		return
	}

	session, card, err := h.svc.Heartbeat(r.Context(), HeartbeatInput{
		SessionToken:  req.SessionToken,
		MachineID:     req.MachineID,
		ClientVersion: req.ClientVersion,
		RemoteAddr:    h.clientIP(r),
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExpired) {
			status = http.StatusUnauthorized
		} else if errors.Is(err, ErrCardNotFound) || errors.Is(err, ErrCardDisabled) || errors.Is(err, ErrCardExpired) || errors.Is(err, ErrMachineMismatch) || errors.Is(err, ErrCardMismatch) || errors.Is(err, ErrCardNoLongerValid) {
			status = http.StatusForbidden
		}
		httpapi.WriteError(w, status, err.Error())
		return
	}

	_, updateInfo := h.versionCheck.CheckVersion(w, req)
	resp := map[string]interface{}{
		"expires_at":      session.ExpiresAt.Unix(),
		"card_expires_at": card.ExpiresAt.Unix(),
		"message":         "心跳正常",
	}
	if updateInfo != nil {
		resp["update"] = updateInfo
	}
	writeCompatOK(w, resp)
}

func (h *Handler) HandleDeactivate(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	req, err := h.reader.ReadSignedRequest(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if err := h.svc.Deactivate(r.Context(), DeactivateInput{
		SessionToken: req.SessionToken,
		MachineID:    req.MachineID,
		CardCode:     req.Card,
		Unbind:       false,
	}); err != nil {
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "会话不存在或已过期"})
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "已注销"})
}

func (h *Handler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	cardCode := r.URL.Query().Get("card")
	activeParam := r.URL.Query().Get("active")
	activeOnly := true // Default to active only
	if activeParam == "0" {
		activeOnly = false
	}
	machineID := r.URL.Query().Get("machine")

	// Helper to parse ints from query
	parseInt := func(val string, def int) int {
		var i int
		_, err := fmt.Sscanf(val, "%d", &i)
		if err != nil || i <= 0 {
			return def
		}
		return i
	}

	filter := ListFilter{
		CardCode:   cardCode,
		MachineID:  machineID,
		ActiveOnly: activeOnly,
		Page:       parseInt(r.URL.Query().Get("page"), 1),
		PerPage:    parseInt(r.URL.Query().Get("per_page"), 20),
	}

	result, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(result.Items))
	for _, s := range result.Items {
		items = append(items, map[string]interface{}{
			"token":          s.Token,
			"card":           s.CardCode, // Keep for backward compatibility
			"card_code":      s.CardCode,
			"machine_id":     s.MachineID,
			"fingerprint":    s.Fingerprint,
			"client_version": s.ClientVersion,
			"created_at":     s.CreatedAt.Format(time.RFC3339),
			"last_seen":      s.LastSeen.Format(time.RFC3339),
			"expires_at":     s.ExpiresAt.Format(time.RFC3339),
			"remote_addr":    s.RemoteAddr,
		})
	}

	writeCompatOK(w, map[string]interface{}{
		"sessions": items,
		"total":    result.Total,
		"page":     result.Page,
		"per_page": result.PerPage,
	})
}

func (h *Handler) HandleForceLogout(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var reqPayload struct {
		SessionToken string `json:"session_token"`
	}
	if err := httpapi.ReadJSON(r, &reqPayload); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	token := reqPayload.SessionToken
	if token == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "session_token is required")
		return
	}

	err := h.svc.Deactivate(r.Context(), DeactivateInput{
		SessionToken: token,
		IsAdminForce: true,
	})
	if err != nil {
		if err == ErrSessionNotFound {
			httpapi.WriteError(w, http.StatusNotFound, "session not found")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeCompatOK(w, map[string]interface{}{
		"message": "session terminated",
	})
}
