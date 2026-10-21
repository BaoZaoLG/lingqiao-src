package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/audit"
	agentsvc "github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
)

func TestClampIntAndPagination(t *testing.T) {
	v := 0
	clampInt(&v, 1, 10, 5)
	if v != 5 {
		t.Fatalf("default = %d", v)
	}
	v = 100
	clampInt(&v, 1, 10, 5)
	if v != 10 {
		t.Fatalf("max = %d", v)
	}
	v = -3
	clampInt(&v, 1, 10, 5)
	if v != 5 {
		t.Fatalf("non-positive uses default = %d", v)
	}

	req := httptest.NewRequest(http.MethodGet, "/x?page=2&per_page=20", nil)
	page, per := parsePagination(req)
	if page != 2 || per != 20 {
		t.Fatalf("page/per = %d/%d", page, per)
	}
	req = httptest.NewRequest(http.MethodGet, "/x?page=0&per_page=9999", nil)
	page, per = parsePagination(req)
	if page != 1 || per != 200 {
		t.Fatalf("clamped page/per = %d/%d", page, per)
	}
	start, end := paginate(25, 2, 10)
	if start != 10 || end != 20 {
		t.Fatalf("paginate = %d,%d", start, end)
	}
	start, end = paginate(5, 3, 10)
	if start != 5 || end != 5 {
		t.Fatalf("paginate past end = %d,%d", start, end)
	}
}

func TestSpreadsheetSafeCSVCell(t *testing.T) {
	if spreadsheetSafeCSVCell("=cmd") != "'=cmd" {
		t.Fatal("formula prefix")
	}
	if spreadsheetSafeCSVCell("ok") != "ok" {
		t.Fatal("plain")
	}
	if spreadsheetSafeCSVCell("  ") != "" {
		t.Fatal("trim empty")
	}
}

func TestParseTimeAndIntQuery(t *testing.T) {
	if _, ok := parseTimeQuery(""); ok {
		t.Fatal("empty time")
	}
	if _, ok := parseTimeQuery("2026-07-17"); !ok {
		t.Fatal("date")
	}
	if _, ok := parseTimeQuery("2026-07-17T12:00:00Z"); !ok {
		t.Fatal("rfc3339")
	}
	if _, ok := parseTimeQuery("nope"); ok {
		t.Fatal("invalid")
	}
	if n, ok := parseIntQuery("42"); !ok || n != 42 {
		t.Fatalf("int = %d %v", n, ok)
	}
	if _, ok := parseIntQuery("x"); ok {
		t.Fatal("bad int")
	}
}

func TestFilterAuditEntriesAndLeaderboard(t *testing.T) {
	now := time.Date(2026, 7, 17, 15, 0, 0, 0, time.UTC)
	entries := []audit.Event{
		{Time: now.Add(-2 * time.Hour), Action: "login", Card: "C1", Detail: "ok"},
		{Time: now.Add(-48 * time.Hour), Action: "logout", Card: "C2", Detail: "bye"},
	}
	filtered := filterAuditEntries(entries, "login", "2026-07-17", "2026-07-17")
	if len(filtered) != 1 || filtered[0].Action != "login" {
		t.Fatalf("filter = %+v", filtered)
	}

	board := buildAgentLeaderboard([]cards.Card{
		{Code: "1", AgentID: "a1", Status: cards.StatusActive, ExpiresAt: now.Add(time.Hour)},
		{Code: "2", AgentID: "a1", Status: cards.StatusExpired, ExpiresAt: now.Add(-time.Hour)},
		{Code: "3", AgentID: "a2", Status: cards.StatusActive, ExpiresAt: now.Add(time.Hour)},
	}, []agentsvc.Account{{ID: "a1", Username: "alice"}}, now)
	if len(board) != 2 {
		t.Fatalf("board len = %d", len(board))
	}
	if board[0].TotalCards < board[1].TotalCards {
		t.Fatal("expected sorted by total desc")
	}
	_ = url.Values{}
}

func TestHasPermission(t *testing.T) {
	if !HasPermission(RoleOwner, "admins.write") {
		t.Fatal("owner should have admins.write")
	}
	if HasPermission(RoleSupport, "cards.write") {
		t.Fatal("support should not write cards")
	}
	if HasPermission(Role("nope"), "cards.read") {
		t.Fatal("unknown role")
	}
	if !IsValidRole(RoleOperator) || IsValidRole(Role("x")) {
		t.Fatal("IsValidRole")
	}
}
