package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/httpapi"

	agentsvc "github.com/lingqiao/server/internal/features/agents"
)

// ── Session Management ───────────────────────────────────────────────────────

func (h *AdminHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if h.sessionHandler != nil {
		h.sessionHandler.HandleListSessions(w, r)
	}
}

func (h *AdminHandler) HandleForceLogout(w http.ResponseWriter, r *http.Request) {
	if h.sessionHandler != nil {
		h.sessionHandler.HandleForceLogout(w, r)
	}
}

// ── Blacklist ────────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleBlacklist(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Action string `json:"action"`
			Type   string `json:"type"`
			Value  string `json:"value"`
			Reason string `json:"reason"`
		}
		if err := httpapi.ReadJSON(r, &req); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		switch req.Action {
		case "add":
			if err := h.blacklistSvc.Add(r.Context(), req.Type, req.Value, req.Reason); err != nil {
				httpapi.WriteError(w, http.StatusInternalServerError, "failed to add to blacklist: "+err.Error())
				return
			}
			if err := h.auditRecorder.Append(audit.Event{Action: "blacklist_added", Detail: fmt.Sprintf("Type: %s, Value: %s, Reason: %s", req.Type, req.Value, req.Reason)}); err != nil {
				httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
				return
			}
			log.Printf("[ADMIN] Blacklisted %s: %s (reason: %s)", req.Type, req.Value, req.Reason)
		case "remove":
			if err := h.blacklistSvc.Remove(r.Context(), req.Value); err != nil {
				httpapi.WriteError(w, http.StatusInternalServerError, "failed to remove from blacklist: "+err.Error())
				return
			}
			if err := h.auditRecorder.Append(audit.Event{Action: "blacklist_removed", Detail: fmt.Sprintf("Value: %s", req.Value)}); err != nil {
				httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
				return
			}
			log.Printf("[ADMIN] Removed from blacklist: %s", req.Value)
		default:
			httpapi.WriteError(w, http.StatusBadRequest, "unknown action")
			return
		}
		httpapi.WriteOK(w, nil)
		return
	}

	entries, err := h.blacklistSvc.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list blacklist: "+err.Error())
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"entries": entries})
}

// ── Audit Log ────────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	entries, err := h.auditRecorder.Query(r.Context(), 10000)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to query audit log: "+err.Error())
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Time.After(entries[j].Time)
	})

	q := r.URL.Query()
	if action := q.Get("action"); action != "" {
		filtered := make([]audit.Event, 0)
		for _, e := range entries {
			if e.Action == action {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	entries = filterAuditEntries(entries, q.Get("q"), q.Get("from"), q.Get("to"))

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
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch req.Action {
	case "save":
		a, err := SaveAnnouncementRevision(req.Content, false)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.recordAnnouncementAudit("announcement_saved", a); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
			return
		}
		httpapi.WriteOK(w, map[string]interface{}{"announcement": a, "id": a.ID})
	case "publish":
		a, err := PublishAnnouncementRevision(req.ID)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.recordAnnouncementAudit("announcement_published", a); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
			return
		}
		httpapi.WriteOK(w, map[string]interface{}{"announcement": a, "id": a.ID})
	case "delete":
		if err := DeleteAnnouncementRevision(req.ID); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.recordAnnouncementAudit("announcement_deleted", &Announcement{ID: req.ID}); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
			return
		}
		httpapi.WriteOK(w, nil)
	default:
		a, err := SaveAnnouncementRevision(req.Content, true)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.recordAnnouncementAudit("announcement_published", a); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
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
	if h.db == nil {
		httpapi.WriteOK(w, map[string]interface{}{"machines": []MachineInfo{}})
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
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var machines []MachineInfo
	for rows.Next() {
		var m MachineInfo
		var lastSeen sql.NullTime
		if err := rows.Scan(&m.MachineID, &m.CardCount, &lastSeen); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to scan machine row: "+err.Error())
			return
		}
		if lastSeen.Valid {
			m.LastSeen = &lastSeen.Time
		}
		machines = append(machines, m)
	}
	if err := rows.Err(); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "error iterating machine rows: "+err.Error())
		return
	}

	// Annotate blacklisted machines
	if h.blacklistSvc != nil {
		blacklisted, err := h.blacklistSvc.List(r.Context())
		if err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to query blacklist: "+err.Error())
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
	machineID := r.URL.Query().Get("id")
	if machineID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "machine id is required")
		return
	}
	cardsList, err := h.cardsSvc.GetCardsByMachine(r.Context(), machineID)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to get machine cards")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"cards": cardsList})
}

// ── Invite Codes ─────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleInviteList(w http.ResponseWriter, r *http.Request) {
	invites, err := h.inviteSvc.List()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load invites: "+err.Error())
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
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	clampInt(&req.Count, 1, 50, 1)
	clampInt(&req.MaxUses, 1, 1000, 1)

	created := make([]agentsvc.InviteCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		invite, err := h.inviteSvc.Create(req.MaxUses, "admin")
		if err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to create invite: "+err.Error())
			return
		}
		created = append(created, invite)
	}
	if err := h.recordInviteAudit("invite_created", created, fmt.Sprintf("count=%d max_uses=%d", len(created), req.MaxUses)); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
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
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	deleted, err := h.inviteSvc.Delete(req.Code)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to delete invite: "+err.Error())
		return
	}
	if !deleted {
		httpapi.WriteError(w, http.StatusNotFound, "邀请码不存在")
		return
	}
	if err := h.recordInviteAudit("invite_deleted", []agentsvc.InviteCode{{Code: req.Code}}, ""); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
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
