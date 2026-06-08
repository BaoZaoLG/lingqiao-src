package main

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleOpsOverviewReturnsOperationalSignals(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	active, _ := cm.GenerateCard(48*time.Hour, "active-card", 1, "agent-1")
	expiring, _ := cm.GenerateCard(24*time.Hour, "expiring-card", 1, "agent-1")
	multi, _ := cm.GenerateCard(72*time.Hour, "machine-risk", 1, "")
	_, _ = cm.ActivateCard(active.Code, "machine-a", "fp", "127.0.0.1", "2.0.0")
	_, _ = cm.ActivateCard(expiring.Code, "machine-b", "fp", "127.0.0.1", "2.0.0")
	_, _ = cm.ActivateCard(multi.Code, "machine-a", "fp", "127.0.0.1", "2.0.0")

	handler := &AdminHandler{cm: cm}
	req := httptest.NewRequest(http.MethodGet, "/admin/api/ops/overview", nil)
	rr := httptest.NewRecorder()

	handler.HandleOpsOverview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["today_activations"].(float64) < 3 {
		t.Fatalf("today_activations = %v, want >= 3", got["today_activations"])
	}
	if len(got["expiring_cards"].([]any)) == 0 {
		t.Fatal("expected expiring_cards to be populated")
	}
	if len(got["risky_machines"].([]any)) == 0 {
		t.Fatal("expected risky_machines to be populated")
	}
	if len(got["agent_leaderboard"].([]any)) == 0 {
		t.Fatal("expected agent_leaderboard to be populated")
	}
}

func TestHandleListCardsSupportsAdvancedFilters(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	agentCard, _ := cm.GenerateCard(24*time.Hour, "agent-owned", 2, "agent-1")
	plainCard, _ := cm.GenerateCard(240*time.Hour, "plain", 1, "")
	_, _ = cm.ActivateCard(agentCard.Code, "machine-a", "fp", "127.0.0.1", "2.0.0")

	handler := &AdminHandler{cm: cm}
	req := httptest.NewRequest(http.MethodGet, "/admin/api/cards?agent_id=agent-1&bound=true&max_sessions=2&expires_before="+time.Now().Add(48*time.Hour).Format(time.RFC3339), nil)
	rr := httptest.NewRecorder()

	handler.HandleListCards(rr, req)

	var got struct {
		Cards []Card `json:"cards"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Cards) != 1 || got.Cards[0].Code != agentCard.Code {
		t.Fatalf("filtered cards = %+v, want only %s (plain card %s excluded)", got.Cards, agentCard.Code, plainCard.Code)
	}
}

func TestHandleAuditLogSupportsKeywordAndTimeRange(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	cm.mu.Lock()
	cm.auditLog = []AuditEntry{
		{Time: time.Now().Add(-2 * time.Hour), Action: "card_generated", Card: "OLD", Detail: "legacy"},
		{Time: time.Now(), Action: "card_activated", Card: "NEW", Machine: "machine-a", Detail: "target keyword"},
	}
	cm.mu.Unlock()

	handler := &AdminHandler{cm: cm}
	from := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/admin/api/audit?q=keyword&from="+from, nil)
	rr := httptest.NewRecorder()

	handler.HandleAuditLog(rr, req)

	var got struct {
		Entries []AuditEntry `json:"entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Card != "NEW" {
		t.Fatalf("entries = %+v, want only NEW keyword entry", got.Entries)
	}
}

func TestHandleAuditLogExportsFilteredCSV(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	cm.RecordAudit(AuditEntry{Time: time.Now(), Action: "announcement_published", Detail: "id=ann-1"})
	cm.RecordAudit(AuditEntry{Time: time.Now(), Action: "payload_activated", Detail: "id=payload-1"})
	handler := &AdminHandler{cm: cm}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/audit?export=csv&action=payload_activated", nil)
	rr := httptest.NewRecorder()
	handler.HandleAuditLog(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Fatalf("content type = %q, want text/csv; charset=utf-8", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "time,action,card,machine,agent_id,detail,addr") {
		t.Fatalf("csv header missing: %q", body)
	}
	if !strings.Contains(body, "payload_activated") || strings.Contains(body, "announcement_published") {
		t.Fatalf("csv filter not applied: %q", body)
	}
}

func TestInviteCreateAndDeleteAreAudited(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	cm := NewCardManager(NewJSONStorage(dataDir()))
	handler := &AdminHandler{cm: cm}

	createReq := httptest.NewRequest(http.MethodPost, "/admin/api/invite/create", strings.NewReader(`{"count":1,"max_uses":3}`))
	createRR := httptest.NewRecorder()
	handler.HandleInviteCreate(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body=%s", createRR.Code, createRR.Body.String())
	}
	var created struct {
		Invites []InviteCode `json:"invites"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Invites) != 1 {
		t.Fatalf("created invites = %#v, want one", created.Invites)
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/admin/api/invite/delete", strings.NewReader(`{"code":"`+created.Invites[0].Code+`"}`))
	deleteRR := httptest.NewRecorder()
	handler.HandleInviteDelete(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200; body=%s", deleteRR.Code, deleteRR.Body.String())
	}

	actions := map[string]bool{}
	for _, entry := range cm.AuditLog() {
		actions[entry.Action] = true
	}
	for _, action := range []string{"invite_created", "invite_deleted"} {
		if !actions[action] {
			t.Fatalf("missing audit action %s in %#v", action, cm.AuditLog())
		}
	}
}

func TestHandleAdminListAgentsReturnsMetricsWithoutPassword(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	agent, err := cm.CreateAgent("alpha", hashPassword("pass123"), "")
	if err != nil {
		t.Fatal(err)
	}
	active, _ := cm.GenerateCard(48*time.Hour, "agent-active", 1, agent.ID)
	expired, _ := cm.GenerateCard(48*time.Hour, "agent-expired", 1, agent.ID)
	_, _ = cm.ActivateCard(active.Code, "machine-alpha", "fp", "127.0.0.1", "2.0.0")
	_ = cm.UpdateCardStatus(expired.Code, CardExpired)

	handler := &AdminHandler{cm: cm}
	req := httptest.NewRequest(http.MethodGet, "/admin/api/agents", nil)
	rr := httptest.NewRecorder()

	handler.HandleAdminListAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Agents) != 1 {
		t.Fatalf("agents = %#v, want one", got.Agents)
	}
	summary := got.Agents[0]
	if _, exposed := summary["password"]; exposed {
		t.Fatalf("agent summary exposes password hash: %#v", summary)
	}
	if summary["total_cards"].(float64) != 2 || summary["active_cards"].(float64) != 1 || summary["expired_cards"].(float64) != 1 {
		t.Fatalf("unexpected agent metrics: %#v", summary)
	}
	if summary["bound_machines"].(float64) != 1 {
		t.Fatalf("bound_machines = %v, want 1", summary["bound_machines"])
	}
	if summary["last_card_created_at"] == nil {
		t.Fatalf("last_card_created_at missing: %#v", summary)
	}
}

func TestHandleAdminListAgentsExportsMetricsCSV(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	agent, err := cm.CreateAgent("beta", hashPassword("pass123"), "")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = cm.GenerateCard(48*time.Hour, "agent-csv", 1, agent.ID)

	handler := &AdminHandler{cm: cm}
	req := httptest.NewRequest(http.MethodGet, "/admin/api/agents?export=csv", nil)
	rr := httptest.NewRecorder()

	handler.HandleAdminListAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Fatalf("content type = %q, want text/csv; charset=utf-8", ct)
	}
	rows, err := csv.NewReader(strings.NewReader(rr.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v\n%s", err, rr.Body.String())
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2: %#v", len(rows), rows)
	}
	wantHeader := []string{"agent_id", "username", "disabled", "total_cards", "active_cards", "expired_cards", "bound_machines", "last_card_created_at", "created_at"}
	if strings.Join(rows[0], ",") != strings.Join(wantHeader, ",") {
		t.Fatalf("header = %#v, want %#v", rows[0], wantHeader)
	}
	if rows[1][1] != "beta" || strings.Contains(rr.Body.String(), agent.Password) {
		t.Fatalf("CSV row exposes wrong data: %#v", rows[1])
	}
}

func TestHandleModulesOverviewReturnsReadinessForCommercialModules(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	cm := NewCardManager(NewJSONStorage(dataDir()))
	agent, err := cm.CreateAgent("ops", hashPassword("pass123"), "")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = cm.GenerateCard(48*time.Hour, "ops-card", 1, agent.ID)
	if _, err := SaveActiveScriptModule("2026.06.08.1", "console.log('ready')"); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveAnnouncementRevision("service ready", "", "", false, true); err != nil {
		t.Fatal(err)
	}
	payloadStore := NewPayloadStore(NewJSONStorage(dataDir()))
	payloadStore.Add(&PayloadInfo{PayloadID: "payload-1", ExeHash: strings.Repeat("a", 64), ChunkCount: 1, ChunkSize: 1024, TotalSize: 1024, CreatedAt: time.Now()})
	if err := payloadStore.SetActive("payload-1"); err != nil {
		t.Fatal(err)
	}

	handler := &AdminHandler{cm: cm}
	req := httptest.NewRequest(http.MethodGet, "/admin/api/modules/overview", nil)
	rr := httptest.NewRecorder()

	handler.HandleModulesOverview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Modules []moduleOverviewItem `json:"modules"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	byKey := map[string]moduleOverviewItem{}
	for _, item := range got.Modules {
		byKey[item.Key] = item
	}
	for _, key := range []string{"cards", "agents", "invites", "releases", "scripts", "payloads", "announcement", "audit"} {
		if _, ok := byKey[key]; !ok {
			t.Fatalf("module %s missing from overview: %#v", key, got.Modules)
		}
	}
	if byKey["cards"].Status != "ready" || byKey["cards"].Metrics["total_cards"].(float64) != 1 {
		t.Fatalf("cards readiness = %#v", byKey["cards"])
	}
	if byKey["scripts"].Status != "ready" || byKey["payloads"].Status != "ready" || byKey["announcement"].Status != "ready" {
		t.Fatalf("expected script/payload/announcement ready: %#v", byKey)
	}
	if byKey["releases"].Status != "warning" {
		t.Fatalf("empty releases should be warning, got %#v", byKey["releases"])
	}
}
