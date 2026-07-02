package cards

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lingqiao/server/internal/httpapi"
)

type AuditSink func(action string, card string, detail string)

type Handler struct {
	svc       *Service
	repo      Repository
	auditSink AuditSink
}

func NewHandler(svc *Service, repo Repository) *Handler {
	return &Handler{svc: svc, repo: repo}
}

func (h *Handler) SetAuditSink(sink AuditSink) {
	h.auditSink = sink
}

func (h *Handler) recordAudit(action string, card string, detail string) {
	if h.auditSink != nil {
		h.auditSink(action, card, detail)
	}
}

func clampInt(val *int, min, max, def int) {
	if *val < min {
		*val = def
	} else if *val > max {
		*val = max
	}
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return false
	}
	return true
}

func readJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeCompatOK(w http.ResponseWriter, data map[string]interface{}) {
	res := map[string]interface{}{"status": "ok"}
	for k, v := range data {
		res[k] = v
	}
	httpapi.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) HandleListCards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := ListFilter{
		Query:     q.Get("search"),
		Status:    Status(q.Get("status")),
		MachineID: q.Get("machine"),
		AgentID:   q.Get("agent_id"),
	}

	cards, err := h.repo.List(r.Context(), filter)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Manual advanced filtering (could be moved to DB query)
	agentID := q.Get("agent_id")
	boundStr := q.Get("bound")
	expiresBefore := q.Get("expires_before")
	expiresAfter := q.Get("expires_after")
	maxSessStr := q.Get("max_sessions")

	var filtered []Card
	for _, c := range cards {
		if agentID != "" && c.AgentID != agentID {
			continue
		}
		if boundStr == "1" && c.MachineID == "" {
			continue
		}
		if boundStr == "0" && c.MachineID != "" {
			continue
		}
		if expiresBefore != "" {
			if t, err := time.Parse(time.RFC3339, expiresBefore); err == nil {
				if c.ExpiresAt.After(t) {
					continue
				}
			}
		}
		if expiresAfter != "" {
			if t, err := time.Parse(time.RFC3339, expiresAfter); err == nil {
				if c.ExpiresAt.Before(t) {
					continue
				}
			}
		}
		if maxSessStr != "" {
			if m, err := strconv.Atoi(maxSessStr); err == nil {
				if c.MaxSessions != m {
					continue
				}
			}
		}
		filtered = append(filtered, c)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	writeCompatOK(w, map[string]interface{}{"cards": filtered})
}

func (h *Handler) HandleGenerateCard(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	card, err := h.svc.Create(r.Context(), time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, "")
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[ADMIN] Generated card: %s", card.Code)
	h.recordAudit("card_generated", card.Code, fmt.Sprintf("Duration: %dh, MaxSessions: %d", req.Duration, req.MaxSessions))
	writeCompatOK(w, map[string]interface{}{"card": card})
}

func (h *Handler) HandleBatchGenerateCards(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Count       int    `json:"count"`
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	clampInt(&req.Count, 1, 500, 1)
	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	cards, err := h.svc.BatchCreate(r.Context(), req.Count, time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, "")
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[ADMIN] Batch generated %d cards", len(cards))
	h.recordAudit("cards_batch_generated", "", fmt.Sprintf("Count: %d, Duration: %dh, MaxSessions: %d", req.Count, req.Duration, req.MaxSessions))
	writeCompatOK(w, map[string]interface{}{"cards": cards, "count": len(cards)})
}

func (h *Handler) HandleUpdateCard(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Code        string `json:"code"`
		Action      string `json:"action"`
		ExtendHours int    `json:"extend_hours"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Code == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "code is required")
		return
	}

	var err error
	switch req.Action {
	case "disable", "enable", "expire":
		status := StatusActive
		switch req.Action {
		case "disable":
			status = StatusDisabled
		case "expire":
			status = StatusExpired
		}
		err = h.svc.UpdateStatus(r.Context(), req.Code, status)
	case "extend":
		if req.ExtendHours <= 0 {
			req.ExtendHours = 720
		}
		err = h.svc.Extend(r.Context(), req.Code, req.ExtendHours)
	case "unbind":
		err = h.svc.Unbind(r.Context(), req.Code)
	default:
		httpapi.WriteError(w, http.StatusBadRequest, "unknown action")
		return
	}

	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	log.Printf("[ADMIN] Card %s: %s", req.Action, req.Code)

	actionName := req.Action
	switch req.Action {
	case "disable", "enable", "expire":
		actionName = "card_status_changed"
	case "extend":
		actionName = "card_extended"
	case "unbind":
		actionName = "card_unbound"
	}
	h.recordAudit(actionName, req.Code, fmt.Sprintf("Action: %s", req.Action))
	writeCompatOK(w, nil)
}

func (h *Handler) HandleUpdateCardDetails(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Code        string `json:"code"`
		Note        string `json:"note"`
		MaxSessions int    `json:"max_sessions"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	r.Body.Close()

	if err := json.Unmarshal(body, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Code == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "code is required")
		return
	}

	var notePtr *string
	var maxSessPtr *int
	bodyStr := string(body)
	if strings.Contains(bodyStr, "\"note\"") {
		notePtr = &req.Note
	}
	if strings.Contains(bodyStr, "\"max_sessions\"") {
		maxSessPtr = &req.MaxSessions
	}

	if err := h.svc.UpdateDetails(r.Context(), req.Code, notePtr, maxSessPtr); err != nil {
		if errors.Is(err, ErrInvalidSessions) {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			httpapi.WriteError(w, http.StatusNotFound, err.Error())
		}
		return
	}
	log.Printf("[ADMIN] Updated card details: %s", req.Code)
	h.recordAudit("card_status_changed", req.Code, "Updated Details")
	writeCompatOK(w, nil)
}

func (h *Handler) HandleBulkCards(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Codes       []string `json:"codes"`
		Action      string   `json:"action"`
		ExtendHours int      `json:"extend_hours"`
		DryRun      bool     `json:"dry_run"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Codes) == 0 {
		httpapi.WriteError(w, http.StatusBadRequest, "codes is required")
		return
	}

	result, err := h.svc.BulkUpdateCardsDetailed(r.Context(), req.Codes, req.Action, req.ExtendHours, req.DryRun)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.DryRun {
		log.Printf("[ADMIN] Bulk %s dry-run: %d cards", req.Action, result.Updated)
	} else {
		log.Printf("[ADMIN] Bulk %s: %d cards", req.Action, result.Updated)
		h.recordAudit("cards_bulk_updated", "", fmt.Sprintf("Action: %s, Affected: %d", req.Action, result.Updated))
	}

	writeCompatOK(w, map[string]interface{}{
		"affected": result.Updated,
		"dry_run":  req.DryRun,
		"result":   result,
	})
}
