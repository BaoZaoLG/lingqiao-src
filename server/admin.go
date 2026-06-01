package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AdminHandler handles the web admin panel.
type AdminHandler struct {
	cm              *CardManager
	adminPassHash   string
	mu              sync.Mutex
	sessions        map[string]time.Time
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
	pass := os.Getenv("ADMIN_PASSWORD")
	if pass == "" {
		// Generate a random password on first run, print to log
		b := make([]byte, 16)
		rand.Read(b)
		pass = hex.EncodeToString(b)
		log.Printf("[ADMIN] No ADMIN_PASSWORD set. Generated random password: %s", pass)
		log.Println("[ADMIN] Set ADMIN_PASSWORD env var to use a fixed password.")
	}

	h := &AdminHandler{
		cm:              cm,
		sessions:        make(map[string]time.Time),
		loginLimiter:    newRateLimiter(15*time.Minute, 5),
		pwChangeLimiter: newRateLimiter(1*time.Hour, 5),
	}

	// Load persisted password hash
	if savedHash, err := os.ReadFile("data/admin_password.hash"); err == nil {
		trimmed := strings.TrimSpace(string(savedHash))
		if len(trimmed) == 64 {
			h.adminPassHash = trimmed
			h.loadSessions()
			return h
		}
	}

	h.adminPassHash = hashPassword(pass)
	os.MkdirAll("data", 0755)
	os.WriteFile("data/admin_password.hash", []byte(h.adminPassHash), 0600)
	h.loadSessions()
	return h
}

func (h *AdminHandler) loadSessions() {
	data, err := os.ReadFile("data/admin_sessions.json")
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
	os.WriteFile("data/admin_sessions.json", data, 0600)
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
	h.sessions[hashToken(token)] = time.Now().Add(4 * time.Hour)
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
		os.MkdirAll("data", 0755)
		os.WriteFile("data/admin_password.hash", []byte(h.adminPassHash), 0600)
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
		MaxAge:   14400,
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
	os.MkdirAll("data", 0755)
	os.WriteFile("data/admin_password.hash", []byte(h.adminPassHash), 0600)
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

// ── Card Management ──────────────────────────────────────────────────────────

func (h *AdminHandler) HandleListCards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cards := h.cm.SearchCards(q.Get("search"), q.Get("status"), q.Get("machine"))
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
		cm := h.cm
		cm.mu.Lock()
		if c, ok := cm.cards[normalizeCardCode(req.Code)]; ok {
			c.MachineID = ""
			if c.ActivatedAt != nil {
				originalDuration := c.ExpiresAt.Sub(*c.ActivatedAt)
				c.ExpiresAt = c.CreatedAt.Add(originalDuration)
			}
			c.ActivatedAt = nil
		}
		cm.auditLog = append(cm.auditLog, AuditEntry{
			Time: time.Now(), Action: "card_unbound", Card: normalizeCardCode(req.Code),
		})
		cm.mu.Unlock()
		cm.save()
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

	affected, err := h.cm.BulkUpdateCards(req.Codes, req.Action, req.ExtendHours)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("[ADMIN] Bulk %s: %d cards", req.Action, affected)
	writeOK(w, map[string]interface{}{"affected": affected})
}

func (h *AdminHandler) HandleExportCards(w http.ResponseWriter, r *http.Request) {
	cards := h.cm.AllCards()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=cards_export.csv")
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	w.Write([]byte("code,status,note,max_sessions,machine_id,created_at,activated_at,expires_at\r\n"))
	for _, c := range cards {
		activatedAt := ""
		if c.ActivatedAt != nil {
			activatedAt = c.ActivatedAt.Format("2006-01-02 15:04:05")
		}
		w.Write([]byte(fmt.Sprintf("%s,%s,\"%s\",%d,\"%s\",%s,%s,%s\r\n",
			c.Code, c.Status, c.Note, c.MaxSessions, c.MachineID,
			c.CreatedAt.Format("2006-01-02 15:04:05"),
			activatedAt,
			c.ExpiresAt.Format("2006-01-02 15:04:05"),
		)))
	}
}

func (h *AdminHandler) HandleImportCards(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		CSV         string `json:"csv"`
		Duration    int    `json:"duration"`
		MaxSessions int    `json:"max_sessions"`
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

	lines := strings.Split(strings.TrimSpace(req.CSV), "\n")
	imported := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "code") || strings.Contains(lower, "卡密") {
			continue
		}
		code := strings.TrimSpace(strings.Split(line, ",")[0])
		if code == "" {
			continue
		}
		if _, err := h.cm.GenerateCard(time.Duration(req.Duration)*time.Hour, "imported", req.MaxSessions, ""); err == nil {
			imported++
		}
	}
	log.Printf("[ADMIN] Imported %d cards from CSV", imported)
	writeOK(w, map[string]interface{}{"imported": imported})
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

	if action := r.URL.Query().Get("action"); action != "" {
		filtered := make([]AuditEntry, 0)
		for _, e := range entries {
			if e.Action == action {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
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
	if a == nil {
		writeOK(w, map[string]interface{}{"announcement": nil})
		return
	}
	writeOK(w, map[string]interface{}{"announcement": a})
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
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	SetAnnouncement(req.Content, req.LatestVersion, req.MinVersion, req.ForceUpdate)
	log.Printf("[ADMIN] Announcement updated: content=%q version=%s", req.Content, req.LatestVersion)
	writeOK(w, nil)
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

	codes := LoadInviteCodes()
	filtered := make([]InviteCode, 0, len(codes))
	found := false
	for _, c := range codes {
		if c.Code == req.Code {
			found = true
			continue
		}
		filtered = append(filtered, c)
	}
	if !found {
		writeError(w, http.StatusNotFound, "邀请码不存在")
		return
	}
	SaveInviteCodes(filtered)
	log.Printf("[ADMIN] Deleted invite code: %s", req.Code)
	writeOK(w, nil)
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
