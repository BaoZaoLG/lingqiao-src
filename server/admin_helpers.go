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

	"github.com/lingqiao/server/internal/audit"
	agentsvc "github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
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

func writeAuditCSV(w http.ResponseWriter, entries []audit.Event) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_export.csv")
	w.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM
	writer := csv.NewWriter(w)
	writer.UseCRLF = true
	_ = writer.Write([]string{"time", "action", "card", "machine", "agent", "ip", "detail"})
	for _, e := range entries {
		_ = writer.Write([]string{
			e.Time.Format("2006-01-02 15:04:05"),
			spreadsheetSafeCSVCell(e.Action),
			spreadsheetSafeCSVCell(e.Card),
			spreadsheetSafeCSVCell(e.Machine),
			spreadsheetSafeCSVCell(e.AgentID),
			spreadsheetSafeCSVCell(e.IP),
			spreadsheetSafeCSVCell(e.Detail),
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

func buildAgentLeaderboard(cardsList []cards.Card, accs []agentsvc.Account, now time.Time) []agentLeaderboardItem {
	names := make(map[string]string, len(accs))
	for _, agent := range accs {
		names[agent.ID] = agent.Username
	}
	byAgent := make(map[string]*agentLeaderboardItem)
	for _, card := range cardsList {
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
		if card.Status == cards.StatusActive && card.ExpiresAt.After(now) {
			item.ActiveCards++
		}
		if card.Status == cards.StatusExpired || card.ExpiresAt.Before(now) {
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

func filterAuditEntries(entries []audit.Event, q, fromStr, toStr string) []audit.Event {
	from, hasFrom := parseTimeQuery(fromStr)
	to, hasTo := parseTimeQuery(toStr)
	if hasTo {
		// If only date was provided (00:00), push to end of day
		if to.Hour() == 0 && to.Minute() == 0 && to.Second() == 0 {
			to = to.Add(24*time.Hour - time.Nanosecond)
		}
	}

	filtered := make([]audit.Event, 0)
	for _, e := range entries {
		if hasFrom && e.Time.Before(from) {
			continue
		}
		if hasTo && e.Time.After(to) {
			continue
		}
		if q != "" {
			qLower := strings.ToLower(q)
			if !strings.Contains(strings.ToLower(e.Action), qLower) &&
				!strings.Contains(strings.ToLower(e.Card), qLower) &&
				!strings.Contains(strings.ToLower(e.Machine), qLower) &&
				!strings.Contains(strings.ToLower(e.AgentID), qLower) &&
				!strings.Contains(strings.ToLower(e.IP), qLower) &&
				!strings.Contains(strings.ToLower(e.Detail), qLower) {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	return filtered
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
