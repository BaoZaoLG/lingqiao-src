package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
)

func (h *AdminHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	cardsList, err := h.cardsSvc.AllCards(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load cards: "+err.Error())
		return
	}

	now := time.Now()
	var activeCards, expiredCards int
	for _, c := range cardsList {
		if c.Status == cards.StatusActive && now.Before(c.ExpiresAt) {
			activeCards++
		}
		if now.After(c.ExpiresAt) || c.Status == cards.StatusExpired {
			expiredCards++
		}
	}

	activeSessions := 0
	if res, err := h.sessionsSvc.List(r.Context(), sessions.ListFilter{ActiveOnly: true}); err == nil {
		activeSessions = res.Total
	}

	httpapi.WriteOK(w, map[string]interface{}{
		"total_cards":     len(cardsList),
		"active_cards":    activeCards,
		"expired_cards":   expiredCards,
		"active_sessions": activeSessions,
	})
}

func (h *AdminHandler) HandleServerStats(w http.ResponseWriter, r *http.Request) {
	activeCount := 0
	if res, err := h.sessionsSvc.List(r.Context(), sessions.ListFilter{ActiveOnly: true}); err == nil {
		activeCount = res.Total
	}

	cardsList, err := h.cardsSvc.AllCards(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load cards: "+err.Error())
		return
	}

	httpapi.WriteOK(w, map[string]interface{}{
		"uptime_seconds":  int(time.Since(serverStart).Seconds()),
		"total_cards":     len(cardsList),
		"active_sessions": activeCount,
		"request_count":   requestCount.Load(),
	})
}

func (h *AdminHandler) HandleOpsOverview(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cardsList, err := h.cardsSvc.AllCards(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load cards: "+err.Error())
		return
	}
	auditLog, err := h.auditRecorder.Query(r.Context(), audit.Filter{}, 10000)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to query audit log: "+err.Error())
		return
	}

	var todayActivations int
	newMachines := make(map[string]struct{})
	recentAudit := make([]audit.Event, 0, 8)
	for _, entry := range auditLog {
		if entry.Time.After(dayStart) && entry.Action == "card_activated" {
			todayActivations++
			if entry.Machine != "" {
				newMachines[entry.Machine] = struct{}{}
			}
		}
	}
	for i := 0; i < len(auditLog) && i < 8; i++ {
		recentAudit = append(recentAudit, auditLog[i])
	}

	expiring := make([]cards.Card, 0)
	expiringBefore := now.Add(7 * 24 * time.Hour)
	for _, card := range cardsList {
		if card.Status == cards.StatusActive && card.ExpiresAt.After(now) && card.ExpiresAt.Before(expiringBefore) {
			expiring = append(expiring, card)
		}
	}
	sort.Slice(expiring, func(i, j int) bool { return expiring[i].ExpiresAt.Before(expiring[j].ExpiresAt) })
	if len(expiring) > 10 {
		expiring = expiring[:10]
	}

	activeSessions := 0
	if res, err := h.sessionsSvc.List(r.Context(), sessions.ListFilter{ActiveOnly: true}); err == nil {
		activeSessions = res.Total
	}

	riskyMachines := make([]cards.MachineInfo, 0)
	if h.db != nil {
		rows, err := h.db.QueryContext(r.Context(), `
			SELECT c.machine_id, COUNT(c.code) as card_count
			FROM cards c
			WHERE c.machine_id != ''
			GROUP BY c.machine_id
			HAVING card_count > 1
		`)
		if err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to query risky machines: "+err.Error())
			return
		}
		defer rows.Close()
		for rows.Next() {
			var m cards.MachineInfo
			if err := rows.Scan(&m.MachineID, &m.CardCount); err != nil {
				httpapi.WriteError(w, http.StatusInternalServerError, "failed to scan risky machine row: "+err.Error())
				return
			}
			riskyMachines = append(riskyMachines, m)
		}
		if err := rows.Err(); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "error iterating risky machine rows: "+err.Error())
			return
		}
	}

	blacklisted, err := h.blacklistSvc.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to query blacklist: "+err.Error())
		return
	}
	blacklistSet := make(map[string]bool)
	for _, b := range blacklisted {
		if b.Type != "machine" {
			continue
		}
		blacklistSet[b.Value] = true
		found := false
		for _, m := range riskyMachines {
			if m.MachineID == b.Value {
				found = true
				break
			}
		}
		if !found {
			riskyMachines = append(riskyMachines, cards.MachineInfo{MachineID: b.Value})
		}
	}
	for i := range riskyMachines {
		riskyMachines[i].IsBlacklisted = blacklistSet[riskyMachines[i].MachineID]
	}
	sort.Slice(riskyMachines, func(i, j int) bool {
		if riskyMachines[i].IsBlacklisted != riskyMachines[j].IsBlacklisted {
			return riskyMachines[i].IsBlacklisted
		}
		return riskyMachines[i].CardCount > riskyMachines[j].CardCount
	})
	if len(riskyMachines) > 10 {
		riskyMachines = riskyMachines[:10]
	}

	accs, err := h.agentsSvc.AllAgents(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load agents: "+err.Error())
		return
	}

	httpapi.WriteOK(w, map[string]interface{}{
		"today_activations":  todayActivations,
		"today_new_machines": len(newMachines),
		"active_sessions":    activeSessions,
		"expiring_cards":     expiring,
		"risky_machines":     riskyMachines,
		"agent_leaderboard":  buildAgentLeaderboard(cardsList, accs, now),
		"recent_audit":       recentAudit,
	})
}

type moduleOverviewItem struct {
	Key       string                 `json:"key"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Summary   string                 `json:"summary"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	UpdatedAt *time.Time             `json:"updated_at,omitempty"`
}

func (h *AdminHandler) HandleModulesOverview(w http.ResponseWriter, r *http.Request) {
	cardsList, cardErr := h.cardsSvc.AllCards(r.Context())
	activeSessions := 0
	if res, err := h.sessionsSvc.List(r.Context(), sessions.ListFilter{ActiveOnly: true}); err == nil {
		activeSessions = res.Total
	}
	cardStatus := "ready"
	cardSummary := fmt.Sprintf("%d total cards", len(cardsList))
	if cardErr != nil {
		cardStatus = "error"
		cardSummary = cardErr.Error()
	}

	agentsList, agentErr := h.agentsSvc.AllAgents(r.Context())
	agentStatus := "ready"
	agentSummary := fmt.Sprintf("%d agents", len(agentsList))
	if agentErr != nil {
		agentStatus = "error"
		agentSummary = agentErr.Error()
	}

	auditEvents, auditErr := h.auditRecorder.Query(r.Context(), audit.Filter{}, 10000)
	auditStatus := "ready"
	auditSummary := fmt.Sprintf("%d retained events", len(auditEvents))
	if auditErr != nil {
		auditStatus = "error"
		auditSummary = auditErr.Error()
	}

	httpapi.WriteOK(w, map[string]interface{}{
		"generated_at": time.Now().UTC(),
		"modules": []moduleOverviewItem{
			{
				Key:     "cards",
				Name:    "卡密授权",
				Status:  cardStatus,
				Summary: cardSummary,
				Metrics: map[string]interface{}{"total_cards": len(cardsList), "active_sessions": activeSessions},
			},
			{
				Key:     "agents",
				Name:    "代理渠道",
				Status:  agentStatus,
				Summary: agentSummary,
				Metrics: map[string]interface{}{"total_agents": len(agentsList)},
			},
			{
				Key:     "audit",
				Name:    "审计日志",
				Status:  auditStatus,
				Summary: auditSummary,
				Metrics: map[string]interface{}{"total_events": len(auditEvents)},
			},
		},
	})
}
