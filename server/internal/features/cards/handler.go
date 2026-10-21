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

func writeCompatOK(w http.ResponseWriter, data map[string]interface{}) {
	res := map[string]interface{}{"status": "ok"}
	for k, v := range data {
		res[k] = v
	}
	httpapi.WriteJSON(w, http.StatusOK, res)
}

func writeCardMutationError(w http.ResponseWriter, err error, notFoundMsg, internalMsg string) {
	if err == nil {
		return
	}
	if errors.Is(err, ErrNotFound) || strings.Contains(strings.ToLower(err.Error()), "not found") {
		httpapi.WriteError(w, http.StatusNotFound, notFoundMsg)
		return
	}
	if errors.Is(err, ErrInvalidSessions) || errors.Is(err, ErrInvalidDuration) {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "extendhours") || strings.Contains(msg, "unknown action") || strings.Contains(msg, "invalid") || strings.Contains(msg, "unsupported") {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("[CARDS] %s: %v", internalMsg, err)
	httpapi.WriteError(w, http.StatusInternalServerError, internalMsg)
}

func effectiveStatus(card Card, now time.Time) Status {
	if !card.ExpiresAt.IsZero() && !now.Before(card.ExpiresAt) {
		return StatusExpired
	}
	return card.Status
}

func (h *Handler) HandleListCards(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	q := r.URL.Query()
	requestedStatus := Status(q.Get("status"))
	filter := ListFilter{
		Query:     q.Get("search"),
		MachineID: q.Get("machine"),
		AgentID:   q.Get("agent_id"),
	}
	if requestedStatus != StatusActive && requestedStatus != StatusExpired {
		filter.Status = requestedStatus
	}

	cards, err := h.repo.List(r.Context(), filter)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list cards")
		return
	}

	// Manual advanced filtering (could be moved to DB query)
	agentID := q.Get("agent_id")
	boundStr := q.Get("bound")
	expiresBefore := q.Get("expires_before")
	expiresAfter := q.Get("expires_after")
	maxSessStr := q.Get("max_sessions")

	var filtered []Card
	now := h.svc.now()
	for _, c := range cards {
		if agentID != "" && c.AgentID != agentID {
			continue
		}
		if (boundStr == "1" || boundStr == "true") && c.MachineID == "" {
			continue
		}
		if (boundStr == "0" || boundStr == "false") && c.MachineID != "" {
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
		c.Status = effectiveStatus(c, now)
		if requestedStatus != "" && c.Status != requestedStatus {
			continue
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	card, err := h.svc.Create(r.Context(), time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, "")
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to generate card")
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
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
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to batch generate cards")
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
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
	case "delete":
		err = h.svc.Delete(r.Context(), req.Code)
	default:
		httpapi.WriteError(w, http.StatusBadRequest, "unknown action")
		return
	}

	if err != nil {
		writeCardMutationError(w, err, "card not found", "failed to update card")
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
	case "delete":
		actionName = "card_deleted"
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	r.Body.Close()

	if err := json.Unmarshal(body, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
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
		writeCardMutationError(w, err, "card not found", "failed to update card details")
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Codes) == 0 {
		httpapi.WriteError(w, http.StatusBadRequest, "codes is required")
		return
	}

	result, err := h.svc.BulkUpdateCardsDetailed(r.Context(), req.Codes, req.Action, req.ExtendHours, req.DryRun)
	if err != nil {
		writeCardMutationError(w, err, "card not found", "failed to bulk update cards")
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
