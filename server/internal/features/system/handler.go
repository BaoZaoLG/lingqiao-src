package system

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/features/security/blacklist"
	"github.com/lingqiao/server/internal/httpapi"
)

type Handler struct {
	announcementSvc *AnnouncementService
	db              *sql.DB
	blacklistSvc    *blacklist.Service
	auditRecorder   *audit.Recorder
}

func NewHandler(announcementSvc *AnnouncementService, db *sql.DB, blacklistSvc *blacklist.Service, auditRecorder *audit.Recorder) *Handler {
	return &Handler{
		announcementSvc: announcementSvc,
		db:              db,
		blacklistSvc:    blacklistSvc,
		auditRecorder:   auditRecorder,
	}
}

func (h *Handler) HandleAnnouncementGet(w http.ResponseWriter, r *http.Request) {
	a := h.announcementSvc.Get()
	items, activeID, _ := h.announcementSvc.ListRevisions()
	if a == nil {
		httpapi.WriteOK(w, map[string]interface{}{"announcement": nil, "announcements": items, "active_id": activeID})
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"announcement": a, "announcements": items, "active_id": activeID})
}

func (h *Handler) HandleAnnouncementSet(w http.ResponseWriter, r *http.Request) {
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
		a, err := h.announcementSvc.SaveRevision(req.Content, false)
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
		a, err := h.announcementSvc.PublishRevision(req.ID)
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
		if err := h.announcementSvc.DeleteRevision(req.ID); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.recordAnnouncementAudit("announcement_deleted", &Announcement{ID: req.ID}); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "Action succeeded but audit log failed to save: "+err.Error())
			return
		}
		httpapi.WriteOK(w, nil)
	default:
		a, err := h.announcementSvc.SaveRevision(req.Content, true)
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

func (h *Handler) recordAnnouncementAudit(action string, a *Announcement) error {
	if h.auditRecorder == nil || a == nil {
		return nil
	}
	return h.auditRecorder.Append(audit.Event{
		Action: action,
		Detail: fmt.Sprintf("id=%s status=%s size=%d", a.ID, a.Status, len([]byte(a.Content))),
	})
}

// MachineInfo is returned to the admin frontend
type MachineInfo struct {
	MachineID     string     `json:"machine_id"`
	CardCount     int        `json:"card_count"`
	LastSeen      *time.Time `json:"last_seen,omitempty"`
	IsBlacklisted bool       `json:"is_blacklisted"`
}

func (h *Handler) HandleAdminMachines(w http.ResponseWriter, r *http.Request) {
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

// RegisterRoutes is called by the main IoC container. Note: Auth middleware should be applied by the caller.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/admin/api/announcement", authMiddleware(h.HandleAnnouncementGet))
	mux.HandleFunc("/admin/api/announcement/set", authMiddleware(h.HandleAnnouncementSet))
	mux.HandleFunc("/admin/api/machines", authMiddleware(h.HandleAdminMachines))
}
