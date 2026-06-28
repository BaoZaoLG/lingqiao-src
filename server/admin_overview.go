package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"
)

// ── Dashboard & Stats ────────────────────────────────────────────────────────

func (h *AdminHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	cards := h.cm.AllCards()
	sessions := h.cm.AllSessions()

	now := time.Now()
	var activeCards, expiredCards, activeSessions int
	for _, c := range cards {
		if c.Status == CardActive && now.Before(c.ExpiresAt) {
			activeCards++
		}
		if now.After(c.ExpiresAt) || c.Status == CardExpired {
			expiredCards++
		}
	}
	for _, s := range sessions {
		if s.ExpiresAt.After(now) {
			activeSessions++
		}
	}

	writeOK(w, map[string]interface{}{
		"total_cards":     len(cards),
		"active_cards":    activeCards,
		"expired_cards":   expiredCards,
		"active_sessions": activeSessions,
	})
}

func (h *AdminHandler) HandleServerStats(w http.ResponseWriter, r *http.Request) {
	sessions := h.cm.AllSessions()
	now := time.Now()
	activeCount := 0
	for _, s := range sessions {
		if s.ExpiresAt.After(now) {
			activeCount++
		}
	}
	writeOK(w, map[string]interface{}{
		"uptime_seconds":  int(time.Since(serverStart).Seconds()),
		"total_cards":     len(h.cm.AllCards()),
		"active_sessions": activeCount,
		"request_count":   requestCount.Load(),
	})
}

func (h *AdminHandler) HandleOpsOverview(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cards := h.cm.AllCards()
	sessions := h.cm.AllSessions()
	audit := h.cm.AuditLog()

	var todayActivations int
	newMachines := make(map[string]struct{})
	recentAudit := make([]AuditEntry, 0, 8)
	for _, entry := range audit {
		if entry.Time.After(dayStart) && entry.Action == "card_activated" {
			todayActivations++
			if entry.Machine != "" {
				newMachines[entry.Machine] = struct{}{}
			}
		}
	}
	sort.Slice(audit, func(i, j int) bool { return audit[i].Time.After(audit[j].Time) })
	for i := 0; i < len(audit) && i < 8; i++ {
		recentAudit = append(recentAudit, audit[i])
	}

	expiring := make([]*Card, 0)
	expiringBefore := now.Add(7 * 24 * time.Hour)
	for _, card := range cards {
		if card.Status == CardActive && card.ExpiresAt.After(now) && card.ExpiresAt.Before(expiringBefore) {
			expiring = append(expiring, card)
		}
	}
	sort.Slice(expiring, func(i, j int) bool { return expiring[i].ExpiresAt.Before(expiring[j].ExpiresAt) })
	if len(expiring) > 10 {
		expiring = expiring[:10]
	}

	activeSessions := 0
	for _, session := range sessions {
		if session.ExpiresAt.After(now) {
			activeSessions++
		}
	}

	riskyMachines := make([]MachineInfo, 0)
	for _, machine := range h.cm.GetUniqueMachines() {
		if machine.IsBlacklisted || machine.CardCount > 1 {
			riskyMachines = append(riskyMachines, machine)
		}
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

	agentStats := buildAgentLeaderboard(cards, h.cm.AllAgents(), now)
	writeOK(w, map[string]interface{}{
		"today_activations":  todayActivations,
		"today_new_machines": len(newMachines),
		"active_sessions":    activeSessions,
		"expiring_cards":     expiring,
		"risky_machines":     riskyMachines,
		"agent_leaderboard":  agentStats,
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
	modules := []moduleOverviewItem{
		h.cardsModuleOverview(),
		h.agentsModuleOverview(),
		invitesModuleOverview(),
		releasesModuleOverview(r),
		scriptsModuleOverview(),
		h.payloadsModuleOverview(),
		announcementModuleOverview(),
		h.auditModuleOverview(),
	}
	writeOK(w, map[string]interface{}{
		"generated_at": time.Now().UTC(),
		"modules":      modules,
	})
}

func (h *AdminHandler) cardsModuleOverview() moduleOverviewItem {
	now := time.Now()
	cards := h.cm.AllCards()
	sessions := h.cm.AllSessions()
	activeCards := 0
	expiredCards := 0
	activeSessions := 0
	for _, card := range cards {
		if card.Status == CardActive && card.ExpiresAt.After(now) {
			activeCards++
		}
		if card.Status == CardExpired || card.ExpiresAt.Before(now) {
			expiredCards++
		}
	}
	for _, session := range sessions {
		if session.ExpiresAt.After(now) {
			activeSessions++
		}
	}
	status := "warning"
	if len(cards) > 0 {
		status = "ready"
	}
	return moduleOverviewItem{
		Key:     "cards",
		Name:    "卡密授权",
		Status:  status,
		Summary: fmt.Sprintf("%d total, %d active", len(cards), activeCards),
		Metrics: map[string]interface{}{
			"total_cards":     len(cards),
			"active_cards":    activeCards,
			"expired_cards":   expiredCards,
			"active_sessions": activeSessions,
		},
	}
}

func (h *AdminHandler) agentsModuleOverview() moduleOverviewItem {
	agents := buildAdminAgentSummaries(h.cm.AllAgents(), h.cm.AllCards(), time.Now())
	disabled := 0
	totalCards := 0
	for _, agent := range agents {
		if agent.Disabled {
			disabled++
		}
		totalCards += agent.TotalCards
	}
	status := "warning"
	if len(agents) > 0 {
		status = "ready"
	}
	return moduleOverviewItem{
		Key:     "agents",
		Name:    "代理渠道",
		Status:  status,
		Summary: fmt.Sprintf("%d agents, %d cards", len(agents), totalCards),
		Metrics: map[string]interface{}{
			"total_agents":    len(agents),
			"disabled_agents": disabled,
			"agent_cards":     totalCards,
		},
	}
}

func invitesModuleOverview() moduleOverviewItem {
	invites := LoadInviteCodes()
	available := 0
	for _, invite := range invites {
		if invite.MaxUses == 0 || invite.UseCount < invite.MaxUses {
			available++
		}
	}
	status := "warning"
	if available > 0 {
		status = "ready"
	}
	return moduleOverviewItem{
		Key:     "invites",
		Name:    "邀请码",
		Status:  status,
		Summary: fmt.Sprintf("%d available / %d total", available, len(invites)),
		Metrics: map[string]interface{}{
			"total_invites":     len(invites),
			"available_invites": available,
		},
	}
}

func releasesModuleOverview(r *http.Request) moduleOverviewItem {
	item := moduleOverviewItem{
		Key:     "releases",
		Name:    "客户端发布",
		Status:  "warning",
		Summary: "release service unavailable",
		Metrics: map[string]interface{}{},
	}
	svc := currentReleaseService()
	if svc == nil {
		return item
	}
	releases, err := svc.store.ListReleases(r.Context())
	if err != nil {
		item.Summary = err.Error()
		return item
	}
	counts := map[string]int{}
	var latestPublished *time.Time
	latestVersion := ""
	for _, release := range releases {
		status := string(release.Status)
		counts[status]++
		if status == "published" {
			publishedAt := release.CreatedAt
			if release.PublishedAt != nil {
				publishedAt = *release.PublishedAt
			}
			if latestPublished == nil || publishedAt.After(*latestPublished) {
				t := publishedAt
				latestPublished = &t
				latestVersion = release.Version
			}
		}
	}
	if counts["published"] > 0 {
		item.Status = "ready"
		item.Summary = fmt.Sprintf("v%s published", latestVersion)
		item.UpdatedAt = latestPublished
	} else {
		item.Summary = "no published release"
	}
	item.Metrics = map[string]interface{}{
		"total_releases":       len(releases),
		"published_releases":   counts["published"],
		"draft_releases":       counts["draft"],
		"paused_releases":      counts["paused"],
		"rolled_back_releases": counts["rolled_back"],
	}
	return item
}

func scriptsModuleOverview() moduleOverviewItem {
	scripts, activeID, err := ListScriptModules()
	item := moduleOverviewItem{
		Key:     "scripts",
		Name:    "脚本模块",
		Status:  "warning",
		Summary: "no active script",
		Metrics: map[string]interface{}{"total_scripts": 0},
	}
	if err != nil {
		item.Summary = err.Error()
		return item
	}
	item.Metrics["total_scripts"] = len(scripts)
	item.Metrics["active_id"] = activeID
	for _, script := range scripts {
		if script.ID == activeID {
			item.Status = "ready"
			item.Summary = "active v" + script.Version
			t := script.UpdatedAt
			item.UpdatedAt = &t
			return item
		}
	}
	return item
}

func (h *AdminHandler) payloadsModuleOverview() moduleOverviewItem {
	storage := h.cm.storage
	if storage == nil {
		storage = NewJSONStorage(dataDir())
	}
	store := NewPayloadStore(storage)
	payloads := store.List()
	active := store.Active()
	item := moduleOverviewItem{
		Key:     "payloads",
		Name:    "载荷模块",
		Status:  "warning",
		Summary: "no active payload",
		Metrics: map[string]interface{}{"total_payloads": len(payloads)},
	}
	if active != nil {
		item.Status = "ready"
		item.Summary = "active " + active.PayloadID
		item.Metrics["active_payload_id"] = active.PayloadID
		item.Metrics["active_payload_size"] = active.TotalSize
		t := active.CreatedAt
		item.UpdatedAt = &t
	}
	return item
}

func announcementModuleOverview() moduleOverviewItem {
	announcement := GetAnnouncement()
	item := moduleOverviewItem{
		Key:     "announcement",
		Name:    "公告",
		Status:  "warning",
		Summary: "no active announcement",
		Metrics: map[string]interface{}{},
	}
	revisions, activeID, _ := ListAnnouncementRevisions()
	item.Metrics["total_announcements"] = len(revisions)
	item.Metrics["active_id"] = activeID
	if announcement != nil {
		item.Status = "ready"
		item.Summary = fmt.Sprintf("active announcement, %d bytes", len([]byte(announcement.Content)))
		t := announcement.UpdatedAt
		item.UpdatedAt = &t
	}
	return item
}

func (h *AdminHandler) auditModuleOverview() moduleOverviewItem {
	audit := h.cm.AuditLog()
	item := moduleOverviewItem{
		Key:     "audit",
		Name:    "审计日志",
		Status:  "warning",
		Summary: "no audit events",
		Metrics: map[string]interface{}{"total_events": len(audit)},
	}
	if len(audit) > 0 {
		item.Status = "ready"
		item.Summary = fmt.Sprintf("%d retained events", len(audit))
		latest := audit[0].Time
		for _, entry := range audit[1:] {
			if entry.Time.After(latest) {
				latest = entry.Time
			}
		}
		item.UpdatedAt = &latest
	}
	return item
}
