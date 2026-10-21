package authorizations

import (
	"encoding/json"
	"log"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/httpapi"
)

const routePrefix = "/admin/api/control/authorizations"

type Handler struct {
	svc       *Service
	auditSink func(action string, code string, detail string)
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) SetAuditSink(sink func(action string, code string, detail string)) {
	h.auditSink = sink
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == routePrefix {
		h.HandleList(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, routePrefix+"/") {
		if strings.HasSuffix(r.URL.Path, "/impact") {
			h.HandleImpact(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/actions") {
			h.HandleAction(w, r)
			return
		}
		h.HandleDetail(w, r)
		return
	}
	httpapi.WriteError(w, http.StatusNotFound, "authorization route not found")
}

func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	maxSessions, _ := strconv.Atoi(q.Get("max_sessions"))
	items, err := h.svc.List(r.Context(), ListFilter{
		Query:           q.Get("search"),
		EffectiveStatus: EffectiveStatus(q.Get("effective_status")),
		AgentID:         q.Get("agent_id"),
		MachineID:       q.Get("machine_id"),
		Bound:           q.Get("bound"),
		ExpiresAfter:    parseOptionalTime(q.Get("expires_after")),
		ExpiresBefore:   parseOptionalTime(q.Get("expires_before")),
		MaxSessions:     maxSessions,
	})
	if err != nil {
		log.Printf("[AUTHORIZATIONS] list failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list authorizations")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"authorizations": items,
	})
}

func parseOptionalTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (h *Handler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	code := authorizationCodeFromPath(r.URL.Path)
	if code == "" || strings.Contains(code, "/") {
		httpapi.WriteError(w, http.StatusNotFound, "authorization not found")
		return
	}
	item, err := h.svc.Detail(r.Context(), code)
	if err != nil {
		if errors.Is(err, cards.ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "authorization not found")
			return
		}
		log.Printf("[AUTHORIZATIONS] detail failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load authorization")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"authorization": item,
	})
}

func (h *Handler) HandleImpact(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Action      Action `json:"action"`
		ExtendHours int    `json:"extend_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	code := authorizationCodeFromPath(strings.TrimSuffix(r.URL.Path, "/impact"))
	preview, err := h.svc.PreviewAction(r.Context(), []string{code}, req.Action, req.ExtendHours)
	if err != nil {
		if errors.Is(err, cards.ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "authorization not found")
			return
		}
		log.Printf("[AUTHORIZATIONS] impact preview failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to preview authorization action")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "preview": preview})
}

func (h *Handler) HandleAction(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Action      Action `json:"action"`
		ExtendHours int    `json:"extend_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	code := authorizationCodeFromPath(strings.TrimSuffix(r.URL.Path, "/actions"))
	result, err := h.svc.ExecuteAction(r.Context(), []string{code}, req.Action, req.ExtendHours)
	if err != nil {
		if errors.Is(err, cards.ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "authorization not found")
			return
		}
		log.Printf("[AUTHORIZATIONS] action failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to execute authorization action")
		return
	}
	if h.auditSink != nil && result.Updated > 0 {
		h.auditSink(auditAction(req.Action), code, "Action: "+string(req.Action))
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "result": result})
}

func (h *Handler) HandleBulkImpact(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Codes       []string `json:"codes"`
		Action      Action   `json:"action"`
		ExtendHours int      `json:"extend_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	preview, err := h.svc.PreviewAction(r.Context(), req.Codes, req.Action, req.ExtendHours)
	if err != nil {
		log.Printf("[AUTHORIZATIONS] bulk impact failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to preview bulk authorization action")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "preview": preview})
}

func (h *Handler) HandleBulkAction(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Codes       []string `json:"codes"`
		Action      Action   `json:"action"`
		ExtendHours int      `json:"extend_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.svc.ExecuteAction(r.Context(), req.Codes, req.Action, req.ExtendHours)
	if err != nil {
		log.Printf("[AUTHORIZATIONS] bulk action failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to execute bulk authorization action")
		return
	}
	if h.auditSink != nil && result.Updated > 0 {
		h.auditSink("authorization_bulk_updated", "", "Action: "+string(req.Action))
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "result": result})
}

func authorizationCodeFromPath(path string) string {
	return strings.Trim(strings.TrimPrefix(path, routePrefix+"/"), "/")
}

func auditAction(action Action) string {
	switch action {
	case ActionExtend:
		return "authorization_extended"
	case ActionUnbindMachine:
		return "authorization_unbound"
	case ActionFreeze:
		return "authorization_frozen"
	case ActionRestore:
		return "authorization_restored"
	case ActionArchiveDelete:
		return "authorization_archived"
	default:
		return "authorization_updated"
	}
}
