package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(pw string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[WARN] bcrypt failed, falling back to sha256: %v", err)
		h := sha256.Sum256([]byte(pw))
		return hex.EncodeToString(h[:])
	}
	return string(hash)
}

func isBcryptHash(h string) bool {
	return len(h) >= 4 && h[0] == '$' && h[1] == '2'
}

func verifyPassword(pw, storedHash string) (match bool, newHash string) {
	if isBcryptHash(storedHash) {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(pw)) == nil, ""
	}
	h := sha256.Sum256([]byte(pw))
	legacyHash := hex.EncodeToString(h[:])
	if subtle.ConstantTimeCompare([]byte(legacyHash), []byte(storedHash)) != 1 {
		return false, ""
	}
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return true, ""
	}
	return true, string(bcryptHash)
}

func hashToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func extractToken(r *http.Request, cookieName string) string {
	if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

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
