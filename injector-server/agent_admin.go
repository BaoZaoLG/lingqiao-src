package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (h *AdminHandler) HandleAdminListAgents(w http.ResponseWriter, r *http.Request) {
	summaries := buildAdminAgentSummaries(h.cm.AllAgents(), h.cm.AllCards(), time.Now())
	if r.URL.Query().Get("export") == "csv" {
		writeAgentMetricsCSV(w, summaries)
		return
	}
	writeOK(w, map[string]interface{}{"agents": summaries})
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

func buildAdminAgentSummaries(agents []*Agent, cards []*Card, now time.Time) []adminAgentSummary {
	byID := make(map[string]*adminAgentSummary, len(agents))
	machinesByAgent := make(map[string]map[string]struct{}, len(agents))
	result := make([]adminAgentSummary, 0, len(agents))
	for _, agent := range agents {
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
		if card.Status == CardActive && card.ExpiresAt.After(now) {
			summary.ActiveCards++
		}
		if card.Status == CardExpired || card.ExpiresAt.Before(now) {
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

func writeAgentMetricsCSV(w http.ResponseWriter, agents []adminAgentSummary) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=agents_export.csv")
	writer := csv.NewWriter(w)
	writer.UseCRLF = true
	_ = writer.Write([]string{"agent_id", "username", "disabled", "total_cards", "active_cards", "expired_cards", "bound_machines", "last_card_created_at", "created_at"})
	for _, agent := range agents {
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

func (h *AdminHandler) HandleAdminUpdateAgent(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		AgentID  string `json:"agent_id"`
		Action   string `json:"action"`
		Password string `json:"password,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch req.Action {
	case "disable":
		if err := h.cm.UpdateAgentStatus(req.AgentID, true); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "enable":
		if err := h.cm.UpdateAgentStatus(req.AgentID, false); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "delete":
		if err := h.cm.DeleteAgent(req.AgentID); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "reset_password":
		if len(req.Password) < 6 {
			writeError(w, http.StatusBadRequest, "密码长度不能少于6位")
			return
		}
		if h.cm.GetAgent(req.AgentID) == nil {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		bcryptHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "密码加密失败")
			return
		}
		if err := h.cm.UpdateAgentPassword(req.AgentID, string(bcryptHash)); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}

	log.Printf("[ADMIN] Agent %s: %s", req.Action, req.AgentID)
	writeOK(w, nil)
}

func (h *AdminHandler) HandleAdminAgentCards(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	writeOK(w, map[string]interface{}{"cards": h.cm.AgentCards(agentID)})
}
