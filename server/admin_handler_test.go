package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/httpapi"
)

func TestAdminHandlersRejectWrongMethodBeforeDependencies(t *testing.T) {
	h := &AdminHandler{}
	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{name: "login", method: http.MethodGet, path: "/admin/api/login", handler: h.HandleLogin},
		{name: "password", method: http.MethodGet, path: "/admin/api/password", handler: h.HandlePasswordChange},
		{name: "dashboard", method: http.MethodPost, path: "/admin/api/dashboard", handler: h.HandleDashboard},
		{name: "stats", method: http.MethodPost, path: "/admin/api/stats", handler: h.HandleServerStats},
		{name: "ops overview", method: http.MethodPost, path: "/admin/api/ops/overview", handler: h.HandleOpsOverview},
		{name: "modules overview", method: http.MethodPost, path: "/admin/api/modules/overview", handler: h.HandleModulesOverview},
		{name: "admin users list", method: http.MethodPut, path: "/admin/api/admin-users", handler: h.HandleAdminUsersList},
		{name: "admin user create", method: http.MethodGet, path: "/admin/api/admin-users", handler: h.HandleAdminUserCreate},
		{name: "admin user update", method: http.MethodGet, path: "/admin/api/admin-user/update", handler: h.HandleAdminUserUpdate},
		{name: "export cards", method: http.MethodPost, path: "/admin/api/cards/export", handler: h.HandleExportCards},
		{name: "import cards", method: http.MethodGet, path: "/admin/api/cards/import", handler: h.HandleImportCards},
		{name: "blacklist get", method: http.MethodPut, path: "/admin/api/blacklist", handler: h.HandleBlacklist},
		{name: "audit log", method: http.MethodPost, path: "/admin/api/audit", handler: h.HandleAuditLog},
		{name: "announcement get", method: http.MethodPost, path: "/admin/api/announcement", handler: h.HandleAnnouncementGet},
		{name: "announcement set", method: http.MethodGet, path: "/admin/api/announcement", handler: h.HandleAnnouncementSet},
		{name: "machines", method: http.MethodPost, path: "/admin/api/machines", handler: h.HandleAdminMachines},
		{name: "machine cards", method: http.MethodPost, path: "/admin/api/machine/cards", handler: h.HandleMachineCards},
		{name: "invite list", method: http.MethodPost, path: "/admin/api/invites", handler: h.HandleInviteList},

	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
			}
		})
	}
}

func TestAdminPasswordChangeRejectsPasswordLongerThanServiceLimit(t *testing.T) {
	h := &AdminHandler{
		pwChangeLimiter: httpapi.NewRateLimiter(time.Minute, 10),
	}
	body := `{"old_password":"old-password","new_password":"` + strings.Repeat("a", 73) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/api/password", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandlePasswordChange(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminUserCreateRejectsInvalidJSON(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/admin-users", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	h.HandleAdminUserCreate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "unexpected EOF") || strings.Contains(rec.Body.String(), "sql:") {
		t.Fatalf("response leaked internal parse/storage detail: %s", rec.Body.String())
	}
}

func TestAdminUserUpdateRejectsInvalidJSON(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/admin-user/update", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	h.HandleAdminUserUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdminImportCardsRejectsInvalidJSONWithoutLeak(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/cards/import", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	h.HandleImportCards(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "unexpected") {
		t.Fatalf("leaked internal detail: %s", rec.Body.String())
	}
}

func TestAdminBlacklistRejectsInvalidJSONWithoutLeak(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/blacklist", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	h.HandleBlacklist(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "unexpected") {
		t.Fatalf("leaked internal detail: %s", rec.Body.String())
	}
}

func TestAdminAnnouncementRejectsInvalidJSONWithoutLeak(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/announcement", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	h.HandleAnnouncementSet(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "unexpected") {
		t.Fatalf("leaked internal detail: %s", rec.Body.String())
	}
}
