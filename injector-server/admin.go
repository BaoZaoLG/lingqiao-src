package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cardops "github.com/lingqiao/server/internal/cards"
	"golang.org/x/crypto/bcrypt"
)

// AdminHandler handles the web admin panel.
type AdminHandler struct {
	cm              *CardManager
	adminPassHash   string
	mu              sync.Mutex
	sessions        map[string]time.Time
	sessionTTL      time.Duration
	loginLimiter    *rateLimiter
	pwChangeLimiter *rateLimiter
}

func hashPassword(pw string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[WARN] bcrypt failed, falling back to sha256: %v", err)
		h := sha256.Sum256([]byte(pw))
		return hex.EncodeToString(h[:])
	}
	return string(hash)
}

// isBcryptHash returns true if the hash is a bcrypt hash (starts with "$2")
func isBcryptHash(h string) bool {
	return len(h) >= 4 && h[0] == '$' && h[1] == '2'
}

// verifyPassword checks password against a hash that may be bcrypt or legacy SHA-256.
// Returns true if password matches, and the new bcrypt hash if migration happened.
func verifyPassword(pw, storedHash string) (match bool, newHash string) {
	if isBcryptHash(storedHash) {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(pw)) == nil, ""
	}
	// Legacy SHA-256: compare, and if match, return bcrypt hash for migration
	h := sha256.Sum256([]byte(pw))
	legacyHash := hex.EncodeToString(h[:])
	if subtle.ConstantTimeCompare([]byte(legacyHash), []byte(storedHash)) != 1 {
		return false, ""
	}
	// Match — migrate to bcrypt
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return true, ""
	}
	return true, string(bcryptHash)
}

func NewAdminHandler(cm *CardManager) *AdminHandler {
	return NewAdminHandlerWithTTL(cm, currentSessionTTL())
}

func NewAdminHandlerWithTTL(cm *CardManager, sessionTTL time.Duration) *AdminHandler {
	if sessionTTL <= 0 {
		sessionTTL = 4 * time.Hour
	}
	h := &AdminHandler{
		cm:              cm,
		sessions:        make(map[string]time.Time),
		sessionTTL:      sessionTTL,
		loginLimiter:    newRateLimiter(15*time.Minute, 5),
		pwChangeLimiter: newRateLimiter(1*time.Hour, 5),
	}

	// Load persisted password hash
	if savedHash, err := os.ReadFile(dataPath("admin_password.hash")); err == nil {
		trimmed := strings.TrimSpace(string(savedHash))
		if isBcryptHash(trimmed) || len(trimmed) == 64 {
			h.adminPassHash = trimmed
			h.loadSessions()
			return h
		}
	}

	pass := os.Getenv("ADMIN_PASSWORD")
	if pass == "" {
		// Generate an initial password only when no persisted hash exists.
		b := make([]byte, 16)
		rand.Read(b)
		pass = hex.EncodeToString(b)
		log.Printf("[ADMIN] No persisted admin password or ADMIN_PASSWORD set. Generated random password: %s", pass)
		log.Println("[ADMIN] Set ADMIN_PASSWORD env var to use a fixed password.")
	}

	h.adminPassHash = hashPassword(pass)
	os.MkdirAll(dataDir(), 0755)
	os.WriteFile(dataPath("admin_password.hash"), []byte(h.adminPassHash), 0600)
	h.loadSessions()
	return h
}

func (h *AdminHandler) loadSessions() {
	data, err := os.ReadFile(dataPath("admin_sessions.json"))
	if err != nil {
		return
	}
	var saved map[string]time.Time
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}
	h.mu.Lock()
	now := time.Now()
	for token, expiry := range saved {
		if expiry.After(now) {
			h.sessions[token] = expiry
		}
	}
	h.mu.Unlock()
}

func (h *AdminHandler) saveSessions() {
	h.mu.Lock()
	data, _ := json.Marshal(h.sessions)
	h.mu.Unlock()
	os.WriteFile(dataPath("admin_sessions.json"), data, 0600)
}

func hashToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func (h *AdminHandler) createSession() string {
	b := make([]byte, 32)
	rand.Read(b)
	token := "admin_" + hex.EncodeToString(b)
	h.mu.Lock()
	h.sessions[hashToken(token)] = time.Now().Add(h.sessionTTL)
	h.mu.Unlock()
	h.saveSessions()
	return token
}

func (h *AdminHandler) checkSession(r *http.Request) bool {
	token := extractToken(r, "admin_token")
	if token == "" {
		return false
	}
	h.mu.Lock()
	expiry, exists := h.sessions[hashToken(token)]
	h.mu.Unlock()
	return exists && time.Now().Before(expiry)
}

func (h *AdminHandler) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/api/login" {
			next(w, r)
			return
		}
		if !h.checkSession(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

// ── Auth Handlers ────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	ip := getClientIP(r)
	if !h.loginLimiter.allow(ip) {
		writeError(w, http.StatusTooManyRequests, "登录尝试过于频繁，请15分钟后再试")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	match, newHash := verifyPassword(req.Password, h.adminPassHash)
	if !match {
		writeError(w, http.StatusUnauthorized, "wrong password")
		return
	}
	// Auto-migrate legacy SHA-256 hash to bcrypt on successful login
	if newHash != "" {
		h.adminPassHash = newHash
		os.MkdirAll(dataDir(), 0755)
		os.WriteFile(dataPath("admin_password.hash"), []byte(h.adminPassHash), 0600)
		log.Printf("[ADMIN] Password hash migrated from SHA-256 to bcrypt")
	}

	h.loginLimiter.clear(ip)

	token := h.createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.sessionTTL.Seconds()),
	})

	writeJSON(w, map[string]string{"status": "ok", "token": token})
}

func (h *AdminHandler) HandlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	ip := getClientIP(r)
	if !h.pwChangeLimiter.allow("pwchg_" + ip) {
		writeError(w, http.StatusTooManyRequests, "密码修改过于频繁，请稍后再试")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, "新密码长度不能少于6位")
		return
	}
	if len(req.NewPassword) > 128 {
		writeError(w, http.StatusBadRequest, "密码长度不能超过128位")
		return
	}
	match, _ := verifyPassword(req.OldPassword, h.adminPassHash)
	if !match {
		writeError(w, http.StatusUnauthorized, "旧密码错误")
		return
	}

	h.adminPassHash = hashPassword(req.NewPassword)
	os.MkdirAll(dataDir(), 0755)
	os.WriteFile(dataPath("admin_password.hash"), []byte(h.adminPassHash), 0600)
	log.Printf("[ADMIN] Admin password changed and persisted")
	writeOK(w, map[string]interface{}{"message": "密码修改成功"})
}

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

// ── Card Management ──────────────────────────────────────────────────────────

func (h *AdminHandler) HandleListCards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cards := h.cm.SearchCards(q.Get("search"), q.Get("status"), q.Get("machine"))
	cards = filterCardsAdvanced(cards, q.Get("agent_id"), q.Get("bound"), q.Get("expires_before"), q.Get("expires_after"), q.Get("max_sessions"))
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].CreatedAt.After(cards[j].CreatedAt)
	})
	writeOK(w, map[string]interface{}{"cards": cards})
}

func (h *AdminHandler) HandleGenerateCard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	card, err := h.cm.GenerateCard(time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[ADMIN] Generated card: %s", card.Code)
	writeOK(w, map[string]interface{}{"card": card})
}

func (h *AdminHandler) HandleBatchGenerateCards(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Count       int    `json:"count"`
		Duration    int    `json:"duration_hours"`
		MaxSessions int    `json:"max_sessions"`
		Note        string `json:"note"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	clampInt(&req.Count, 1, 500, 1)
	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)
	if len(req.Note) > 200 {
		req.Note = req.Note[:200]
	}

	cards, err := h.cm.BatchGenerateCards(req.Count, time.Duration(req.Duration)*time.Hour, req.Note, req.MaxSessions, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[ADMIN] Batch generated %d cards", len(cards))
	writeOK(w, map[string]interface{}{"cards": cards, "count": len(cards)})
}

func (h *AdminHandler) HandleUpdateCard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Code        string `json:"code"`
		Action      string `json:"action"`
		ExtendHours int    `json:"extend_hours"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	card := h.cm.GetCard(req.Code)
	if card == nil {
		writeError(w, http.StatusNotFound, "card not found")
		return
	}

	switch req.Action {
	case "disable", "enable", "expire":
		status := CardStatus(req.Action)
		if req.Action == "expire" {
			status = CardExpired
		}
		if err := h.cm.UpdateCardStatus(req.Code, status); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "extend":
		if req.ExtendHours <= 0 {
			req.ExtendHours = 720
		}
		if err := h.cm.ExtendCard(req.Code, time.Duration(req.ExtendHours)*time.Hour); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	case "unbind":
		if err := h.cm.UnbindCard(req.Code); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}

	log.Printf("[ADMIN] Card %s: %s", req.Action, req.Code)
	writeOK(w, nil)
}

func (h *AdminHandler) HandleUpdateCardDetails(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Code        string `json:"code"`
		Note        string `json:"note"`
		MaxSessions int    `json:"max_sessions"`
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := jsonUnmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	// Detect which fields were explicitly set
	var notePtr *string
	var maxSessPtr *int
	bodyStr := string(body)
	if strings.Contains(bodyStr, "\"note\"") {
		notePtr = &req.Note
	}
	if strings.Contains(bodyStr, "\"max_sessions\"") {
		maxSessPtr = &req.MaxSessions
	}

	if err := h.cm.UpdateCardDetails(req.Code, notePtr, maxSessPtr); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	log.Printf("[ADMIN] Updated card details: %s", req.Code)
	writeOK(w, nil)
}

func (h *AdminHandler) HandleBulkCards(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Codes       []string `json:"codes"`
		Action      string   `json:"action"`
		ExtendHours int      `json:"extend_hours"`
		DryRun      bool     `json:"dry_run"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Codes) == 0 {
		writeError(w, http.StatusBadRequest, "codes is required")
		return
	}
	if req.Action == "" {
		writeError(w, http.StatusBadRequest, "action is required")
		return
	}

	var (
		result cardops.BulkResult
		err    error
	)
	if req.DryRun {
		result, err = h.cm.PreviewBulkUpdateCardsDetailed(req.Codes, req.Action, req.ExtendHours)
	} else {
		result, err = h.cm.BulkUpdateCardsDetailed(req.Codes, req.Action, req.ExtendHours)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.DryRun {
		log.Printf("[ADMIN] Bulk %s dry-run: %d cards", req.Action, result.Updated)
	} else {
		log.Printf("[ADMIN] Bulk %s: %d cards", req.Action, result.Updated)
	}
	writeOK(w, map[string]interface{}{"affected": result.Updated, "dry_run": req.DryRun, "result": result})
}

func (h *AdminHandler) HandleExportCards(w http.ResponseWriter, r *http.Request) {
	cards := h.cm.AllCards()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=cards_export.csv")
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	writer.UseCRLF = true
	_ = writer.Write([]string{"code", "status", "note", "max_sessions", "machine_id", "created_at", "activated_at", "expires_at"})
	for _, c := range cards {
		activatedAt := ""
		if c.ActivatedAt != nil {
			activatedAt = c.ActivatedAt.Format("2006-01-02 15:04:05")
		}
		_ = writer.Write([]string{
			spreadsheetSafeCSVCell(c.Code),
			spreadsheetSafeCSVCell(string(c.Status)),
			spreadsheetSafeCSVCell(c.Note),
			strconv.Itoa(c.MaxSessions),
			spreadsheetSafeCSVCell(c.MachineID),
			c.CreatedAt.Format("2006-01-02 15:04:05"),
			activatedAt,
			c.ExpiresAt.Format("2006-01-02 15:04:05"),
		})
	}
	writer.Flush()
}

func (h *AdminHandler) HandleImportCards(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		CSV         string `json:"csv"`
		Duration    int    `json:"duration"`
		MaxSessions int    `json:"max_sessions"`
		DryRun      bool   `json:"dry_run"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.CSV == "" {
		writeError(w, http.StatusBadRequest, "CSV data is required")
		return
	}
	clampInt(&req.Duration, 1, 17520, 720)
	clampInt(&req.MaxSessions, 1, 100, 1)

	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(req.CSV)))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid CSV: "+err.Error())
		return
	}

	report := h.importCardsFromRecords(records, time.Duration(req.Duration)*time.Hour, req.MaxSessions, req.DryRun)
	if !req.DryRun && report.Imported > 0 {
		h.cm.RecordAudit(AuditEntry{
			Action: "cards_import_completed",
			Detail: fmt.Sprintf("imported=%d duplicates=%d invalid=%d skipped=%d", report.Imported, report.Duplicates, report.Invalid, report.Skipped),
		})
	}
	if req.DryRun {
		log.Printf("[ADMIN] Previewed card CSV import: valid=%d duplicates=%d invalid=%d", report.ValidRows, report.Duplicates, report.Invalid)
	} else {
		log.Printf("[ADMIN] Imported %d cards from CSV", report.Imported)
	}
	writeOK(w, map[string]interface{}{"imported": report.Imported, "dry_run": req.DryRun, "report": report})
}

type cardImportItem struct {
	Row     int    `json:"row"`
	Code    string `json:"code,omitempty"`
	Note    string `json:"note,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type cardImportReport struct {
	TotalRows  int              `json:"total_rows"`
	ValidRows  int              `json:"valid_rows"`
	Imported   int              `json:"imported"`
	Duplicates int              `json:"duplicates"`
	Invalid    int              `json:"invalid"`
	Skipped    int              `json:"skipped"`
	Items      []cardImportItem `json:"items"`
}

func (h *AdminHandler) importCardsFromRecords(records [][]string, duration time.Duration, maxSessions int, dryRun bool) cardImportReport {
	report := cardImportReport{Items: make([]cardImportItem, 0, len(records))}
	seen := make(map[string]struct{}, len(records))
	for idx, record := range records {
		rowNumber := idx + 1
		if len(record) == 0 {
			continue
		}
		code := strings.TrimSpace(record[0])
		lower := strings.ToLower(code)
		if lower == "code" || strings.Contains(lower, "卡密") {
			continue
		}

		report.TotalRows++
		if code == "" {
			report.Skipped++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Status: "skipped", Message: "empty code"})
			continue
		}

		note := "imported"
		if len(record) > 1 && strings.TrimSpace(record[1]) != "" {
			note = strings.TrimSpace(record[1])
		}
		normalized, err := validateImportCardCode(code)
		if err != nil {
			report.Invalid++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Code: code, Note: note, Status: "invalid", Message: err.Error()})
			continue
		}
		formatted := FormatCardCode(normalized)
		if _, exists := seen[normalized]; exists || h.cm.GetCard(formatted) != nil {
			report.Duplicates++
			report.Items = append(report.Items, cardImportItem{Row: rowNumber, Code: formatted, Note: note, Status: "duplicate", Message: "card already exists"})
			continue
		}
		seen[normalized] = struct{}{}
		report.ValidRows++
		item := cardImportItem{Row: rowNumber, Code: formatted, Note: note, Status: "valid"}
		if !dryRun {
			if _, err := h.cm.GenerateCardWithCode(formatted, duration, note, maxSessions, ""); err != nil {
				report.Invalid++
				report.ValidRows--
				item.Status = "invalid"
				item.Message = err.Error()
			} else {
				report.Imported++
				item.Status = "imported"
			}
		}
		report.Items = append(report.Items, item)
	}
	return report
}

func validateImportCardCode(code string) (string, error) {
	normalized := normalizeCardCode(code)
	if len(normalized) != 18 {
		return "", fmt.Errorf("card code must contain 18 base32 characters")
	}
	for i := 0; i < len(normalized); i++ {
		if crockfordDecode[normalized[i]] < 0 {
			return "", fmt.Errorf("card code contains invalid character %q", normalized[i])
		}
	}
	return normalized, nil
}

func spreadsheetSafeCSVCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

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

func writeAuditCSV(w http.ResponseWriter, entries []AuditEntry) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_export.csv")
	writer := csv.NewWriter(w)
	writer.UseCRLF = true
	_ = writer.Write([]string{"time", "action", "card", "machine", "agent_id", "detail", "addr"})
	for _, entry := range entries {
		_ = writer.Write([]string{
			entry.Time.Format(time.RFC3339),
			spreadsheetSafeCSVCell(entry.Action),
			spreadsheetSafeCSVCell(entry.Card),
			spreadsheetSafeCSVCell(entry.Machine),
			spreadsheetSafeCSVCell(entry.AgentID),
			spreadsheetSafeCSVCell(entry.Detail),
			spreadsheetSafeCSVCell(entry.Addr),
		})
	}
	writer.Flush()
}

type agentLeaderboardItem struct {
	AgentID      string `json:"agent_id"`
	Username     string `json:"username"`
	TotalCards   int    `json:"total_cards"`
	ActiveCards  int    `json:"active_cards"`
	ExpiredCards int    `json:"expired_cards"`
}

func buildAgentLeaderboard(cards []*Card, agents []*Agent, now time.Time) []agentLeaderboardItem {
	names := make(map[string]string, len(agents))
	for _, agent := range agents {
		names[agent.ID] = agent.Username
	}
	byAgent := make(map[string]*agentLeaderboardItem)
	for _, card := range cards {
		if card.AgentID == "" {
			continue
		}
		item := byAgent[card.AgentID]
		if item == nil {
			item = &agentLeaderboardItem{AgentID: card.AgentID, Username: names[card.AgentID]}
			if item.Username == "" {
				item.Username = card.AgentID
			}
			byAgent[card.AgentID] = item
		}
		item.TotalCards++
		if card.Status == CardActive && card.ExpiresAt.After(now) {
			item.ActiveCards++
		}
		if card.Status == CardExpired || card.ExpiresAt.Before(now) {
			item.ExpiredCards++
		}
	}
	result := make([]agentLeaderboardItem, 0, len(byAgent))
	for _, item := range byAgent {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].TotalCards > result[j].TotalCards })
	if len(result) > 10 {
		result = result[:10]
	}
	return result
}

func filterCardsAdvanced(cards []*Card, agentID, bound, expiresBefore, expiresAfter, maxSessions string) []*Card {
	before, hasBefore := parseTimeQuery(expiresBefore)
	after, hasAfter := parseTimeQuery(expiresAfter)
	maxSess, hasMaxSess := parseIntQuery(maxSessions)
	result := make([]*Card, 0, len(cards))
	for _, card := range cards {
		if agentID != "" && card.AgentID != agentID {
			continue
		}
		if bound == "true" && card.MachineID == "" {
			continue
		}
		if bound == "false" && card.MachineID != "" {
			continue
		}
		if hasBefore && card.ExpiresAt.After(before) {
			continue
		}
		if hasAfter && card.ExpiresAt.Before(after) {
			continue
		}
		if hasMaxSess && card.MaxSessions != maxSess {
			continue
		}
		result = append(result, card)
	}
	return result
}

func filterAuditEntries(entries []AuditEntry, query, from, to string) []AuditEntry {
	fromTime, hasFrom := parseTimeQuery(from)
	toTime, hasTo := parseTimeQuery(to)
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]AuditEntry, 0, len(entries))
	for _, entry := range entries {
		if hasFrom && entry.Time.Before(fromTime) {
			continue
		}
		if hasTo && entry.Time.After(toTime) {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				entry.Action, entry.Card, entry.Machine, entry.AgentID, entry.Detail, entry.Addr,
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		result = append(result, entry)
	}
	return result
}

func parseTimeQuery(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func parseIntQuery(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
		return 0, false
	}
	return n, true
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

// ── Shared Helpers ───────────────────────────────────────────────────────────

// extractToken gets auth token from cookie or Authorization header.
func extractToken(r *http.Request, cookieName string) string {
	if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// clampInt constrains val to [min, max], using defaultVal if val <= 0.
func clampInt(val *int, min, max, defaultVal int) {
	if *val <= 0 {
		*val = defaultVal
	}
	if *val < min {
		*val = min
	}
	if *val > max {
		*val = max
	}
}

// parsePagination extracts page and per_page from query params.
func parsePagination(r *http.Request) (page, perPage int) {
	page = 1
	perPage = 50
	if p := r.URL.Query().Get("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		fmt.Sscanf(pp, "%d", &perPage)
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	return
}

// paginate returns start/end indices for a slice of given length.
func paginate(length, page, perPage int) (start, end int) {
	start = (page - 1) * perPage
	end = start + perPage
	if start > length {
		start = length
	}
	if end > length {
		end = length
	}
	return
}
