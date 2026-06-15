package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ── Session Management ───────────────────────────────────────────────────────

func (h *AdminHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.cm.AllSessions()
	now := time.Now()
	active := make([]*Session, 0)
	for _, s := range sessions {
		if s.ExpiresAt.After(now) {
			active = append(active, s)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].LastSeen.After(active[j].LastSeen)
	})

	page, perPage := parsePagination(r)
	start, end := paginate(len(active), page, perPage)

	writeOK(w, map[string]interface{}{
		"sessions": active[start:end],
		"total":    len(active),
		"page":     page,
		"per_page": perPage,
	})
}

func (h *AdminHandler) HandleForceLogout(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		SessionToken string `json:"session_token"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.cm.DeactivateSession(req.SessionToken, false); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeOK(w, nil)
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
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		switch req.Action {
		case "add":
			h.cm.AddBlacklist(req.Type, req.Value, req.Reason)
			log.Printf("[ADMIN] Blacklisted %s: %s (reason: %s)", req.Type, req.Value, req.Reason)
		case "remove":
			h.cm.RemoveBlacklist(req.Value)
			log.Printf("[ADMIN] Removed from blacklist: %s", req.Value)
		default:
			writeError(w, http.StatusBadRequest, "unknown action")
			return
		}
		writeOK(w, nil)
		return
	}

	writeOK(w, map[string]interface{}{"entries": h.cm.AllBlacklist()})
}

// ── Audit Log ────────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	entries := h.cm.AuditLog()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Time.After(entries[j].Time)
	})

	q := r.URL.Query()
	if action := q.Get("action"); action != "" {
		filtered := make([]AuditEntry, 0)
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

	writeOK(w, map[string]interface{}{
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
		writeOK(w, map[string]interface{}{"announcement": nil, "announcements": items, "active_id": activeID})
		return
	}
	writeOK(w, map[string]interface{}{"announcement": a, "announcements": items, "active_id": activeID})
}

func (h *AdminHandler) HandleAnnouncementSet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Content       string `json:"content"`
		LatestVersion string `json:"latest_version"`
		MinVersion    string `json:"min_version"`
		ForceUpdate   bool   `json:"force_update"`
		Action        string `json:"action"`
		ID            string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch req.Action {
	case "save":
		a, err := SaveAnnouncementRevision(req.Content, req.LatestVersion, req.MinVersion, req.ForceUpdate, false)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.recordAnnouncementAudit("announcement_saved", a)
		writeOK(w, map[string]interface{}{"announcement": a, "id": a.ID})
	case "publish":
		a, err := PublishAnnouncementRevision(req.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.recordAnnouncementAudit("announcement_published", a)
		writeOK(w, map[string]interface{}{"announcement": a, "id": a.ID})
	case "delete":
		if err := DeleteAnnouncementRevision(req.ID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.recordAnnouncementAudit("announcement_deleted", &Announcement{ID: req.ID})
		writeOK(w, nil)
	default:
		a, err := SaveAnnouncementRevision(req.Content, req.LatestVersion, req.MinVersion, req.ForceUpdate, true)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.recordAnnouncementAudit("announcement_published", a)
		log.Printf("[ADMIN] Announcement updated: content=%q version=%s", req.Content, req.LatestVersion)
		writeOK(w, map[string]interface{}{"announcement": a, "id": a.ID})
	}
}

func (h *AdminHandler) recordAnnouncementAudit(action string, a *Announcement) {
	if h == nil || h.cm == nil || a == nil {
		return
	}
	h.cm.RecordAudit(AuditEntry{
		Action: action,
		Detail: fmt.Sprintf("id=%s status=%s size=%d", a.ID, a.Status, len([]byte(a.Content))),
	})
}

// ── Machines ─────────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleMachines(w http.ResponseWriter, r *http.Request) {
	machines := h.cm.GetUniqueMachines()
	sort.Slice(machines, func(i, j int) bool {
		return machines[i].CardCount > machines[j].CardCount
	})
	writeOK(w, map[string]interface{}{"machines": machines})
}

func (h *AdminHandler) HandleMachineCards(w http.ResponseWriter, r *http.Request) {
	machineID := r.URL.Query().Get("id")
	if machineID == "" {
		writeError(w, http.StatusBadRequest, "machine id is required")
		return
	}
	writeOK(w, map[string]interface{}{"cards": h.cm.GetCardsByMachine(machineID)})
}

// ── Invite Codes ─────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleInviteList(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]interface{}{"invites": LoadInviteCodes()})
}

func (h *AdminHandler) HandleInviteCreate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Count   int `json:"count"`
		MaxUses int `json:"max_uses"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	clampInt(&req.Count, 1, 50, 1)
	clampInt(&req.MaxUses, 1, 1000, 1)

	created := make([]InviteCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		created = append(created, CreateInviteCode(req.MaxUses, "admin"))
	}
	h.recordInviteAudit("invite_created", created, fmt.Sprintf("count=%d max_uses=%d", len(created), req.MaxUses))
	log.Printf("[ADMIN] Created %d invite codes (max_uses=%d)", len(created), req.MaxUses)
	writeOK(w, map[string]interface{}{"invites": created, "count": len(created)})
}

func (h *AdminHandler) HandleInviteDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !DeleteInviteCode(req.Code) {
		writeError(w, http.StatusNotFound, "邀请码不存在")
		return
	}
	h.recordInviteAudit("invite_deleted", []InviteCode{{Code: req.Code}}, "")
	log.Printf("[ADMIN] Deleted invite code: %s", req.Code)
	writeOK(w, nil)
}

func (h *AdminHandler) recordInviteAudit(action string, invites []InviteCode, extra string) {
	if h == nil || h.cm == nil {
		return
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
	h.cm.RecordAudit(AuditEntry{Action: action, Detail: detail})
}

