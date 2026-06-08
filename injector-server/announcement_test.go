package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnnouncementDraftPublishAndRollbackWorkflow(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	cm := NewCardManager(NewJSONStorage(dataDir()))
	admin := &AdminHandler{cm: cm}

	draftID := postAnnouncementAdmin(t, admin, `{"action":"save","content":"draft announcement"}`)
	assertActiveAnnouncement(t, admin, "")

	firstID := postAnnouncementAdmin(t, admin, `{"action":"publish","id":"`+draftID+`"}`)
	assertActiveAnnouncement(t, admin, "draft announcement")

	secondID := postAnnouncementAdmin(t, admin, `{"content":"second announcement"}`)
	if secondID == firstID {
		t.Fatalf("second publish reused id %q", secondID)
	}
	assertActiveAnnouncement(t, admin, "second announcement")

	_ = postAnnouncementAdmin(t, admin, `{"action":"publish","id":"`+firstID+`"}`)
	assertActiveAnnouncement(t, admin, "draft announcement")

	resp := getAnnouncementAdmin(t, admin)
	if len(resp.Announcements) != 2 {
		t.Fatalf("announcement history count = %d, want 2", len(resp.Announcements))
	}
	if resp.ActiveID != firstID {
		t.Fatalf("active id = %q, want rollback target %q", resp.ActiveID, firstID)
	}
}

func TestAnnouncementDeleteRejectsActiveRevision(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	admin := &AdminHandler{cm: NewCardManager(NewJSONStorage(dataDir()))}

	id := postAnnouncementAdmin(t, admin, `{"content":"live announcement"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/announcement", bytes.NewBufferString(`{"action":"delete","id":"`+id+`"}`))
	rr := httptest.NewRecorder()
	admin.HandleAnnouncementSet(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("delete active status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

type announcementAdminResponse struct {
	Announcement  *Announcement  `json:"announcement"`
	Announcements []Announcement `json:"announcements"`
	ActiveID      string         `json:"active_id"`
	ID            string         `json:"id"`
}

func postAnnouncementAdmin(t *testing.T, admin *AdminHandler, body string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/announcement", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	admin.HandleAnnouncementSet(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("announcement post status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp announcementAdminResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != "" {
		return resp.ID
	}
	if resp.Announcement != nil && resp.Announcement.ID != "" {
		return resp.Announcement.ID
	}
	t.Fatalf("announcement response did not include id: %s", rr.Body.String())
	return ""
}

func getAnnouncementAdmin(t *testing.T, admin *AdminHandler) announcementAdminResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/announcement", nil)
	rr := httptest.NewRecorder()
	admin.HandleAnnouncementGet(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("announcement get status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp announcementAdminResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertActiveAnnouncement(t *testing.T, admin *AdminHandler, content string) {
	t.Helper()
	resp := getAnnouncementAdmin(t, admin)
	if content == "" {
		if resp.Announcement != nil {
			t.Fatalf("active announcement = %#v, want nil", resp.Announcement)
		}
		return
	}
	if resp.Announcement == nil || resp.Announcement.Content != content {
		t.Fatalf("active announcement = %#v, want content %q", resp.Announcement, content)
	}
}

func TestAnnouncementHistoryPersistsUnderDataDir(t *testing.T) {
	dataDir := t.TempDir()
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: dataDir, SessionTTL: 4 * time.Hour})
	defer restore()
	admin := &AdminHandler{cm: NewCardManager(NewJSONStorage(dataDir))}

	_ = postAnnouncementAdmin(t, admin, `{"content":"persisted announcement"}`)
	if _, err := os.Stat(filepath.Join(dataDir, "announcements", "index.json")); err != nil {
		t.Fatalf("announcement history was not persisted under DATA_DIR: %v", err)
	}
}
