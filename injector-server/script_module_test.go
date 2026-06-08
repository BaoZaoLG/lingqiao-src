package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScriptModuleAdminSaveAndClientDownload(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "data")))
	cm.sessions["session-1"] = &Session{
		Token:     "session-1",
		CardCode:  "ABCDEF-GHJKLM-NPQRST",
		MachineID: "machine-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	api := NewAPIHandler(cm)
	admin := &AdminHandler{cm: cm}

	content := "window.__LINGQIAO_SCRIPT_VERSION='2026.06.07.1';"
	saveReq := httptest.NewRequest(http.MethodPost, "/admin/api/script", bytes.NewBufferString(`{"version":"2026.06.07.1","content":`+mustJSONQuote(t, content)+`}`))
	saveRR := httptest.NewRecorder()
	admin.HandleScriptAdmin(saveRR, saveReq)
	if saveRR.Code != http.StatusOK {
		t.Fatalf("admin save status = %d, want 200; body=%s", saveRR.Code, saveRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/admin/api/script", nil)
	getRR := httptest.NewRecorder()
	admin.HandleScriptAdmin(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("admin get status = %d, want 200; body=%s", getRR.Code, getRR.Body.String())
	}
	var adminResp map[string]any
	if err := json.Unmarshal(getRR.Body.Bytes(), &adminResp); err != nil {
		t.Fatal(err)
	}
	if adminResp["version"] != "2026.06.07.1" {
		t.Fatalf("version = %v, want 2026.06.07.1", adminResp["version"])
	}
	sum := sha256.Sum256([]byte(content))
	wantSHA := hex.EncodeToString(sum[:])
	if adminResp["sha256"] != wantSHA {
		t.Fatalf("sha256 = %v, want %s", adminResp["sha256"], wantSHA)
	}

	clientReq := httptest.NewRequest(http.MethodGet, "/api/v1/script?token=session-1", nil)
	clientRR := httptest.NewRecorder()
	api.HandleScriptDownload(clientRR, clientReq)
	if clientRR.Code != http.StatusOK {
		t.Fatalf("client status = %d, want 200; body=%s", clientRR.Code, clientRR.Body.String())
	}
	var clientResp map[string]any
	if err := json.Unmarshal(clientRR.Body.Bytes(), &clientResp); err != nil {
		t.Fatal(err)
	}
	if clientResp["content"] != content {
		t.Fatalf("content = %q, want %q", clientResp["content"], content)
	}
	if clientResp["sha256"] != wantSHA {
		t.Fatalf("client sha256 = %v, want %s", clientResp["sha256"], wantSHA)
	}

	if _, err := os.Stat(filepath.Join("data", "scripts", "active.json")); err != nil {
		t.Fatalf("active script was not persisted: %v", err)
	}
}

func TestScriptDownloadRejectsMissingSession(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "data")))
	api := NewAPIHandler(cm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/script", nil)
	rr := httptest.NewRecorder()
	api.HandleScriptDownload(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestScriptRepositoryCanSelectActiveScript(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	first, err := SaveActiveScriptModule("2026.06.07.1", "window.first=true;")
	if err != nil {
		t.Fatal(err)
	}
	second, err := SaveScriptModuleDraft("2026.06.07.2", "window.second=true;", "new logic")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || second.ID == "" || first.ID == second.ID {
		t.Fatalf("invalid ids: first=%q second=%q", first.ID, second.ID)
	}

	list, activeID, err := ListScriptModules()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("script count = %d, want 2", len(list))
	}
	if activeID != first.ID {
		t.Fatalf("active id = %q, want %q", activeID, first.ID)
	}

	if err := SetActiveScriptModule(second.ID); err != nil {
		t.Fatal(err)
	}
	active, err := LoadActiveScriptModule()
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != second.ID || active.Content != "window.second=true;" {
		t.Fatalf("active = %#v, want second script", active)
	}
}

func TestScriptDraftDoesNotBecomeActiveUntilPublished(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	draft, err := SaveScriptModuleDraft("2026.06.07.1", "window.draft=true;", "draft only")
	if err != nil {
		t.Fatal(err)
	}
	if draft.Active {
		t.Fatalf("draft was marked active")
	}
	_, activeID, err := ListScriptModules()
	if err != nil {
		t.Fatal(err)
	}
	if activeID != "" {
		t.Fatalf("active id = %q, want empty for draft-only repository", activeID)
	}
	if _, err := LoadActiveScriptModule(); err == nil {
		t.Fatalf("LoadActiveScriptModule succeeded with only a draft script")
	}
}

func TestScriptRepositoryDoesNotDeleteActiveScript(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	active, err := SaveActiveScriptModule("2026.06.07.1", "window.active=true;")
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteScriptModule(active.ID); err == nil {
		t.Fatalf("DeleteScriptModule(active) succeeded, want error")
	}
}

func TestScriptAdminRecordsReleaseStyleAuditEvents(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "data")))
	admin := &AdminHandler{cm: cm}

	draftID := postScriptAdmin(t, admin, `{"action":"save","version":"2026.06.07.1","content":"window.draft=true;"}`)
	publishedID := postScriptAdmin(t, admin, `{"version":"2026.06.07.2","content":"window.live=true;"}`)

	activateReq := httptest.NewRequest(http.MethodPost, "/admin/api/script", bytes.NewBufferString(`{"action":"activate","id":"`+draftID+`"}`))
	activateRR := httptest.NewRecorder()
	admin.HandleScriptAdmin(activateRR, activateReq)
	if activateRR.Code != http.StatusOK {
		t.Fatalf("activate status = %d, want 200; body=%s", activateRR.Code, activateRR.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/admin/api/script", bytes.NewBufferString(`{"action":"delete","id":"`+publishedID+`"}`))
	deleteRR := httptest.NewRecorder()
	admin.HandleScriptAdmin(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200; body=%s", deleteRR.Code, deleteRR.Body.String())
	}

	actions := map[string]bool{}
	for _, entry := range cm.AuditLog() {
		actions[entry.Action] = true
	}
	for _, action := range []string{"script_saved", "script_published", "script_activated", "script_deleted"} {
		if !actions[action] {
			t.Fatalf("missing audit action %s in %#v", action, cm.AuditLog())
		}
	}
}

func postScriptAdmin(t *testing.T, admin *AdminHandler, body string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/script", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	admin.HandleScriptAdmin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("script admin status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID     string        `json:"id"`
		Script *ScriptModule `json:"script"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != "" {
		return resp.ID
	}
	if resp.Script != nil {
		return resp.Script.ID
	}
	t.Fatalf("script admin response did not include id: %s", rr.Body.String())
	return ""
}

func mustJSONQuote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
