package main

import (
	"database/sql"
	"fmt"
	"github.com/lingqiao/server/internal/features/cards"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/httpapi"

	agentsvc "github.com/lingqiao/server/internal/features/agents"
)

// ── Session Management ───────────────────────────────────────────────────────

func (h *AdminHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if h.sessionHandler == nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "sessions service unavailable")
		return
	}
	h.sessionHandler.HandleListSessions(w, r)
}

func (h *AdminHandler) HandleForceLogout(w http.ResponseWriter, r *http.Request) {
	if h.sessionHandler == nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "sessions service unavailable")
		return
	}
	h.sessionHandler.HandleForceLogout(w, r)
}

// ── Blacklist ────────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleBlacklist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
			Type   string `json:"type"`
			Value  string `json:"value"`
			Reason string `json:"reason"`
		}
		if err := httpapi.ReadJSON(r, &req); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		switch req.Action {
		case "add":
			if err := h.blacklistSvc.Add(r.Context(), req.Type, req.Value, req.Reason); err != nil {
				log.Printf("[ADMIN] blacklist add failed: %v", err)
				httpapi.WriteError(w, http.StatusInternalServerError, "failed to add to blacklist")
				return
			}
			if err := h.auditRecorder.Append(audit.Event{Action: "blacklist_added", Detail: fmt.Sprintf("Type: %s, Value: %s, Reason: %s", req.Type, req.Value, req.Reason)}); err != nil {
				log.Printf("[ADMIN] blacklist add audit failed: %v", err)
				httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
				return
			}
			log.Printf("[ADMIN] Blacklisted %s: %s (reason: %s)", req.Type, req.Value, req.Reason)
		case "remove":
			if err := h.blacklistSvc.Remove(r.Context(), req.Value); err != nil {
				log.Printf("[ADMIN] blacklist remove failed: %v", err)
				httpapi.WriteError(w, http.StatusInternalServerError, "failed to remove from blacklist")
				return
			}
			if err := h.auditRecorder.Append(audit.Event{Action: "blacklist_removed", Detail: fmt.Sprintf("Value: %s", req.Value)}); err != nil {
				log.Printf("[ADMIN] blacklist remove audit failed: %v", err)
				httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
				return
			}
			log.Printf("[ADMIN] Removed from blacklist: %s", req.Value)
		default:
			httpapi.WriteError(w, http.StatusBadRequest, "unknown action")
			return
		}
		httpapi.WriteOK(w, nil)
		return
	case http.MethodGet:
		entries, err := h.blacklistSvc.List(r.Context())
		if err != nil {
			log.Printf("[ADMIN] blacklist list failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to list blacklist")
			return
		}
		httpapi.WriteOK(w, map[string]interface{}{"entries": entries})
		return
	default:
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
}

// ── Audit Log ────────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	filter := audit.Filter{
		Action:       q.Get("action"),
		ActorType:    q.Get("actor_type"),
		ActorID:      q.Get("actor_id"),
		ResourceType: q.Get("resource_type"),
		ResourceID:   q.Get("resource_id"),
		Card:         q.Get("card"),
		AgentID:      q.Get("agent_id"),
		Machine:      q.Get("machine"),
		IP:           q.Get("ip"),
		Query:        q.Get("q"),
	}
	if from := q.Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = t
		}
	}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = t
		}
	}

	entries, err := h.auditRecorder.Query(r.Context(), filter, 10000)
	if err != nil {
		log.Printf("[ADMIN] audit query failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to query audit log")
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Time.After(entries[j].Time)
	})

	if q.Get("export") == "csv" {
		writeAuditCSV(w, entries)
		return
	}

	page, perPage := parsePagination(r)
	start, end := paginate(len(entries), page, perPage)

	httpapi.WriteOK(w, map[string]interface{}{
		"entries":  entries[start:end],
		"total":    len(entries),
		"page":     page,
		"per_page": perPage,
	})
}

// ── Announcement ─────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleAnnouncementGet(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	a := GetAnnouncement()
	items, activeID, _ := ListAnnouncementRevisions()
	if a == nil {
		httpapi.WriteOK(w, map[string]interface{}{"announcement": nil, "announcements": items, "active_id": activeID})
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"announcement": a, "announcements": items, "active_id": activeID})
}

func (h *AdminHandler) HandleAnnouncementSet(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Content string `json:"content"`
		Action  string `json:"action"`
		ID      string `json:"id"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Action {
	case "save":
		a, err := SaveAnnouncementRevision(req.Content, false)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "failed to save announcement")
			return
		}
		if err := h.recordAnnouncementAudit("announcement_saved", a); err != nil {
			log.Printf("[ADMIN] announcement audit failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
			return
		}
		httpapi.WriteOK(w, map[string]interface{}{"announcement": a, "id": a.ID})
	case "publish":
		a, err := PublishAnnouncementRevision(req.ID)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "failed to publish announcement")
			return
		}
		if err := h.recordAnnouncementAudit("announcement_published", a); err != nil {
			log.Printf("[ADMIN] announcement audit failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
			return
		}
		httpapi.WriteOK(w, map[string]interface{}{"announcement": a, "id": a.ID})
	case "delete":
		if err := DeleteAnnouncementRevision(req.ID); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "failed to delete announcement")
			return
		}
		if err := h.recordAnnouncementAudit("announcement_deleted", &Announcement{ID: req.ID}); err != nil {
			log.Printf("[ADMIN] announcement audit failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
			return
		}
		httpapi.WriteOK(w, nil)
	default:
		a, err := SaveAnnouncementRevision(req.Content, true)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "failed to save announcement")
			return
		}
		if err := h.recordAnnouncementAudit("announcement_published", a); err != nil {
			log.Printf("[ADMIN] announcement audit failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
			return
		}
		httpapi.WriteOK(w, map[string]interface{}{"announcement": a, "id": a.ID})
	}
}

func (h *AdminHandler) recordAnnouncementAudit(action string, a *Announcement) error {
	if h == nil || h.auditRecorder == nil || a == nil {
		return nil
	}
	return h.auditRecorder.Append(audit.Event{
		Action: action,
		Detail: fmt.Sprintf("id=%s status=%s size=%d", a.ID, a.Status, len([]byte(a.Content))),
	})
}

// ── Machines ─────────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleAdminMachines(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	if h.db == nil {
		httpapi.WriteOK(w, map[string]interface{}{"machines": []cards.MachineInfo{}})
		return
	}

	query := `
		SELECT 
			c.machine_id, 
			COUNT(c.code) as card_count, 
			MAX(s.last_seen) as last_seen
		FROM cards c
		LEFT JOIN sessions s ON c.machine_id = s.machine_id
		WHERE c.machine_id != ''
		GROUP BY c.machine_id
	`
	rows, err := h.db.QueryContext(r.Context(), query)
	if err != nil {
		log.Printf("[ADMIN] list machines failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list machines")
		return
	}
	defer rows.Close()

	var machines []cards.MachineInfo
	for rows.Next() {
		var m cards.MachineInfo
		var lastSeen sql.NullTime
		if err := rows.Scan(&m.MachineID, &m.CardCount, &lastSeen); err != nil {
			log.Printf("[ADMIN] scan machine row failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to list machines")
			return
		}
		if lastSeen.Valid {
			m.LastSeen = &lastSeen.Time
		}
		machines = append(machines, m)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[ADMIN] iterate machines failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list machines")
		return
	}

	// Annotate blacklisted machines
	if h.blacklistSvc != nil {
		blacklisted, err := h.blacklistSvc.List(r.Context())
		if err != nil {
			log.Printf("[ADMIN] annotate blacklist failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to list machines")
			return
		}
		blacklistSet := make(map[string]bool)
		for _, b := range blacklisted {
			if b.Type == "machine" {
				blacklistSet[b.Value] = true
			}
		}
		for i := range machines {
			if blacklistSet[machines[i].MachineID] {
				machines[i].IsBlacklisted = true
			}
		}
	}

	httpapi.WriteOK(w, map[string]interface{}{"machines": machines})
}

func (h *AdminHandler) HandleMachineCards(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	machineID := r.URL.Query().Get("id")
	if machineID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "machine id is required")
		return
	}
	cardsList, err := h.cardsSvc.GetCardsByMachine(r.Context(), machineID)
	if err != nil {
		log.Printf("[ADMIN] get machine cards failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to get machine cards")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"cards": cardsList})
}

// ── Invite Codes ─────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleInviteList(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	invites, err := h.inviteSvc.List()
	if err != nil {
		log.Printf("[ADMIN] list invites failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load invites")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"invites": invites})
}

func (h *AdminHandler) HandleInviteCreate(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Count   int `json:"count"`
		MaxUses int `json:"max_uses"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	clampInt(&req.Count, 1, 50, 1)
	clampInt(&req.MaxUses, 1, 1000, 1)

	created := make([]agentsvc.InviteCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		invite, err := h.inviteSvc.Create(req.MaxUses, "admin")
		if err != nil {
			log.Printf("[ADMIN] create invite failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to create invite")
			return
		}
		created = append(created, invite)
	}
	if err := h.recordInviteAudit("invite_created", created, fmt.Sprintf("count=%d max_uses=%d", len(created), req.MaxUses)); err != nil {
		log.Printf("[ADMIN] invite audit failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
		return
	}
	log.Printf("[ADMIN] Created %d invite codes (max_uses=%d)", len(created), req.MaxUses)
	httpapi.WriteOK(w, map[string]interface{}{"invites": created, "count": len(created)})
}

func (h *AdminHandler) HandleInviteDelete(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deleted, err := h.inviteSvc.Delete(req.Code)
	if err != nil {
		log.Printf("[ADMIN] delete invite failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to delete invite")
		return
	}
	if !deleted {
		httpapi.WriteError(w, http.StatusNotFound, "邀请码不存在")
		return
	}
	if err := h.recordInviteAudit("invite_deleted", []agentsvc.InviteCode{{Code: req.Code}}, ""); err != nil {
		log.Printf("[ADMIN] invite audit failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
		return
	}
	log.Printf("[ADMIN] Deleted invite code: %s", req.Code)
	httpapi.WriteOK(w, nil)
}

func (h *AdminHandler) recordInviteAudit(action string, invites []agentsvc.InviteCode, extra string) error {
	if h == nil || h.auditRecorder == nil {
		return nil
	}
	codes := make([]string, 0, len(invites))
	for _, invite := range invites {
		if invite.Code != "" {
			codes = append(codes, invite.Code)
		}
	}
	detail := "codes=" + strings.Join(codes, ",")
	if extra != "" {
		detail += " " + extra
	}
	return h.auditRecorder.Append(audit.Event{Action: action, Detail: detail})
}
