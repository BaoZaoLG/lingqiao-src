package agents

import (
	"github.com/lingqiao/server/internal/httpapi"

	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"
)

type AdminHandler struct {
	svc *Service
	cm  CardReader
}

func NewAdminHandler(svc *Service, cm CardReader) *AdminHandler {
	return &AdminHandler{svc: svc, cm: cm}
}

func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux, authP func(string, http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/admin/api/agents", authP("agents.read", h.HandleAdminListAgents))
	mux.HandleFunc("/admin/api/agent/update", authP("agents.write", h.HandleAdminUpdateAgent))
	mux.HandleFunc("/admin/api/agent/cards", authP("agents.read", h.HandleAdminAgentCards))
}

// HandleAdminListAgents ...
func (h *AdminHandler) HandleAdminListAgents(w http.ResponseWriter, r *http.Request) {
	allAgents, err := h.svc.AllAgents(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	summaries := buildAdminAgentSummaries(allAgents, h.cm.AllCards(), time.Now())
	if r.URL.Query().Get("export") == "csv" {
		writeAgentMetricsCSV(w, summaries)
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"agents": summaries})
}

type adminAgentSummary struct {
	ID                string     `json:"id"`
	Username          string     `json:"username"`
	Prefix            string     `json:"prefix,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	Disabled          bool       `json:"disabled"`
	TotalCards        int        `json:"total_cards"`
	ActiveCards       int        `json:"active_cards"`
	ExpiredCards      int        `json:"expired_cards"`
	BoundMachines     int        `json:"bound_machines"`
	LastCardCreatedAt *time.Time `json:"last_card_created_at,omitempty"`
}

func buildAdminAgentSummaries(accs []Account, cards []CardSnapshot, now time.Time) []adminAgentSummary {
	byID := make(map[string]*adminAgentSummary, len(accs))
	machinesByAgent := make(map[string]map[string]struct{}, len(accs))
	result := make([]adminAgentSummary, 0, len(accs))
	for _, agent := range accs {
		summary := adminAgentSummary{
			ID:        agent.ID,
			Username:  agent.Username,
			Prefix:    agent.Prefix,
			CreatedAt: agent.CreatedAt,
			Disabled:  agent.Disabled,
		}
		result = append(result, summary)
		byID[agent.ID] = &result[len(result)-1]
		machinesByAgent[agent.ID] = map[string]struct{}{}
	}

	for _, card := range cards {
		summary := byID[card.AgentID]
		if summary == nil {
			continue
		}
		summary.TotalCards++
		if card.Status == "active" && card.ExpiresAt.After(now) {
			summary.ActiveCards++
		}
		if card.Status == "expired" || card.ExpiresAt.Before(now) {
			summary.ExpiredCards++
		}
		if card.MachineID != "" {
			machinesByAgent[card.AgentID][card.MachineID] = struct{}{}
		}
		if summary.LastCardCreatedAt == nil || card.CreatedAt.After(*summary.LastCardCreatedAt) {
			t := card.CreatedAt
			summary.LastCardCreatedAt = &t
		}
	}

	for i := range result {
		result[i].BoundMachines = len(machinesByAgent[result[i].ID])
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalCards != result[j].TotalCards {
			return result[i].TotalCards > result[j].TotalCards
		}
		return result[i].Username < result[j].Username
	})
	return result
}

func writeAgentMetricsCSV(w http.ResponseWriter, summaries []adminAgentSummary) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=agents_export.csv")
	writer := csv.NewWriter(w)
	writer.UseCRLF = true
	_ = writer.Write([]string{"agent_id", "username", "disabled", "total_cards", "active_cards", "expired_cards", "bound_machines", "last_card_created_at", "created_at"})
	for _, agent := range summaries {
		lastCardCreatedAt := ""
		if agent.LastCardCreatedAt != nil {
			lastCardCreatedAt = agent.LastCardCreatedAt.Format(time.RFC3339)
		}
		_ = writer.Write([]string{
			spreadsheetSafeCSVCell(agent.ID),
			spreadsheetSafeCSVCell(agent.Username),
			fmt.Sprintf("%t", agent.Disabled),
			fmt.Sprintf("%d", agent.TotalCards),
			fmt.Sprintf("%d", agent.ActiveCards),
			fmt.Sprintf("%d", agent.ExpiredCards),
			fmt.Sprintf("%d", agent.BoundMachines),
			lastCardCreatedAt,
			agent.CreatedAt.Format(time.RFC3339),
		})
	}
	writer.Flush()
}

func spreadsheetSafeCSVCell(val string) string {
	if len(val) > 0 && (val[0] == '=' || val[0] == '+' || val[0] == '-' || val[0] == '@') {
		return "'" + val
	}
	return val
}

// HandleAdminUpdateAgent ...
func (h *AdminHandler) HandleAdminUpdateAgent(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		AgentID                   string `json:"agent_id"`
		Action                    string `json:"action"`
		Password                  string `json:"password,omitempty"`
		MaxSessionDurationMinutes int    `json:"max_session_duration_minutes,omitempty"`
		MaxDailySessions          int    `json:"max_daily_sessions,omitempty"`
		MaxConcurrentSessions     int    `json:"max_concurrent_sessions,omitempty"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	switch req.Action {
	case "disable":
		if err := h.svc.SetAgentStatus(ctx, req.AgentID, true); err != nil {
			httpapi.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
	case "enable":
		if err := h.svc.SetAgentStatus(ctx, req.AgentID, false); err != nil {
			httpapi.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
	case "delete":
		if err := h.svc.DeleteAgent(ctx, req.AgentID); err != nil {
			httpapi.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
	case "update_limits":
		if err := h.svc.SetAgentLimits(ctx, req.AgentID, req.MaxSessionDurationMinutes, req.MaxDailySessions, req.MaxConcurrentSessions); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "reset_password":
		if len(req.Password) < 6 || len(req.Password) > 72 {
			httpapi.WriteError(w, http.StatusBadRequest, "密码长度必须在6-72位之间")
			return
		}
		// In an admin reset, we don't know the old password.
		// Service.ChangePassword requires oldPassword.
		// We'll need a way for admin to force reset password. We can add ForceResetPassword to Service.
		if err := h.svc.ForceResetPassword(ctx, req.AgentID, req.Password); err != nil {
			httpapi.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
	default:
		httpapi.WriteError(w, http.StatusBadRequest, "unknown action")
		return
	}

	log.Printf("[ADMIN] Agent %s: %s", req.Action, req.AgentID)
	httpapi.WriteOK(w, nil)
}

// HandleAdminAgentCards ...
func (h *AdminHandler) HandleAdminAgentCards(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"cards": h.cm.AgentCards(agentID)})
}
