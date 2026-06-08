package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleBulkCardsReturnsDetailedResult(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	card, _ := cm.GenerateCard(24*time.Hour, "bulk-handler", 1, "")
	handler := &AdminHandler{cm: cm}

	body := bytes.NewBufferString(`{"codes":["` + card.Code + `","missing"],"action":"disable"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/cards/bulk", body)
	rr := httptest.NewRecorder()

	handler.HandleBulkCards(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["affected"].(float64) != 1 {
		t.Fatalf("affected = %v, want 1", got["affected"])
	}
	result := got["result"].(map[string]any)
	if result["updated"].(float64) != 1 {
		t.Fatalf("result.updated = %v, want 1", result["updated"])
	}
	if result["skipped"].(float64) != 1 {
		t.Fatalf("result.skipped = %v, want 1", result["skipped"])
	}
}

func TestHandleBulkCardsDryRunReportsImpactWithoutMutating(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	card, _ := cm.GenerateCard(24*time.Hour, "bulk-dry-run", 1, "")
	handler := &AdminHandler{cm: cm}

	body := bytes.NewBufferString(`{"codes":["` + card.Code + `","missing"],"action":"disable","dry_run":true}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/cards/bulk", body)
	rr := httptest.NewRecorder()

	handler.HandleBulkCards(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Affected float64        `json:"affected"`
		DryRun   bool           `json:"dry_run"`
		Result   map[string]any `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !got.DryRun {
		t.Fatalf("dry_run = false, want true")
	}
	if got.Affected != 1 || got.Result["updated"].(float64) != 1 || got.Result["skipped"].(float64) != 1 {
		t.Fatalf("unexpected dry-run result: %+v", got)
	}
	if refreshed := cm.GetCard(card.Code); refreshed == nil || refreshed.Status != CardActive {
		t.Fatalf("dry-run mutated card status: %+v", refreshed)
	}
	for _, entry := range cm.AuditLog() {
		if entry.Action == "bulk_disable" {
			t.Fatalf("dry-run wrote bulk audit entry: %+v", entry)
		}
	}
}

func TestHandleExportCardsWritesCSVAndNeutralizesFormulas(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	card, _ := cm.GenerateCard(24*time.Hour, "=cmd|'/C calc'!A0", 1, "")
	cm.mu.Lock()
	card.MachineID = "machine,one"
	cm.mu.Unlock()
	handler := &AdminHandler{cm: cm}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/cards/export", nil)
	rr := httptest.NewRecorder()

	handler.HandleExportCards(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := strings.TrimPrefix(rr.Body.String(), "\ufeff")
	rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatalf("export is not valid CSV: %v\n%s", err, body)
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want header + 1 card: %#v", len(rows), rows)
	}
	if rows[1][2] != "'=cmd|'/C calc'!A0" {
		t.Fatalf("note = %q, want formula-neutralized note", rows[1][2])
	}
	if rows[1][4] != "machine,one" {
		t.Fatalf("machine_id = %q, want CSV field with comma preserved", rows[1][4])
	}
}

func TestHandleImportCardsSupportsQuotedCSVAndExplicitCodes(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	handler := &AdminHandler{cm: cm}
	payload, err := json.Marshal(map[string]any{
		"csv":          "code,note\nABCDEF-GHJKMN-PQRSTV,\"team, alpha\"\n",
		"duration":     24,
		"max_sessions": 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/cards/import", bytes.NewReader(payload))
	rr := httptest.NewRecorder()

	handler.HandleImportCards(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["imported"].(float64) != 1 {
		t.Fatalf("imported = %v, want 1", got["imported"])
	}
	card := cm.GetCard("ABCDEF-GHJKMN-PQRSTV")
	if card == nil {
		t.Fatal("imported card was not stored")
	}
	if card.Note != "team, alpha" {
		t.Fatalf("Note = %q, want quoted CSV note", card.Note)
	}
	if card.MaxSessions != 2 {
		t.Fatalf("MaxSessions = %d, want 2", card.MaxSessions)
	}
}

func TestHandleImportCardsDryRunReturnsValidationReport(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	existing, _ := cm.GenerateCardWithCode("ABCDEF-GHJKMN-PQRSTV", 24*time.Hour, "existing", 1, "")
	handler := &AdminHandler{cm: cm}
	payload, err := json.Marshal(map[string]any{
		"csv":          "code,note\n" + existing.Code + ",duplicate\ninvalid-code,bad\nZZZZZZ-ZZZZZZ-ZZZZZZ,valid\n",
		"duration":     24,
		"max_sessions": 2,
		"dry_run":      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/cards/import", bytes.NewReader(payload))
	rr := httptest.NewRecorder()

	handler.HandleImportCards(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Imported int  `json:"imported"`
		DryRun   bool `json:"dry_run"`
		Report   struct {
			TotalRows  int `json:"total_rows"`
			ValidRows  int `json:"valid_rows"`
			Imported   int `json:"imported"`
			Duplicates int `json:"duplicates"`
			Invalid    int `json:"invalid"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !got.DryRun || got.Imported != 0 || got.Report.Imported != 0 {
		t.Fatalf("dry-run import should not commit rows: %+v", got)
	}
	if got.Report.TotalRows != 3 || got.Report.ValidRows != 1 || got.Report.Duplicates != 1 || got.Report.Invalid != 1 {
		t.Fatalf("unexpected import report: %+v", got.Report)
	}
	if card := cm.GetCard("ZZZZZZ-ZZZZZZ-ZZZZZZ"); card != nil {
		t.Fatalf("dry-run imported card unexpectedly: %+v", card)
	}
}
