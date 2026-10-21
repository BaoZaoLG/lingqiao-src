package authorizations

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/features/cards"
)

func TestHandlerListAuthorizations(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	repo := newMemoryCardRepo()
	repo.Save(context.Background(), cards.Card{Code: "AUTH-1", Status: cards.StatusActive, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), MaxSessions: 1})
	handler := NewHandler(NewService(repo, memorySessionLister{}, func() time.Time { return now }))

	req := httptest.NewRequest(http.MethodGet, "/admin/api/control/authorizations", nil)
	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v", resp["status"])
	}
	items := resp["authorizations"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("authorizations length = %d", len(items))
	}
}

func TestHandlerDetailAuthorization(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	repo := newMemoryCardRepo()
	repo.Save(context.Background(), cards.Card{Code: "AUTH-1", Status: cards.StatusActive, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), MaxSessions: 1})
	handler := NewHandler(NewService(repo, memorySessionLister{}, func() time.Time { return now }))

	req := httptest.NewRequest(http.MethodGet, "/admin/api/control/authorizations/AUTH-1", nil)
	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	item := resp["authorization"].(map[string]interface{})
	if item["code"] != "AUTH-1" {
		t.Fatalf("code = %v", item["code"])
	}
}

func TestHandlerDetailAuthorizationNotFound(t *testing.T) {
	handler := NewHandler(NewService(newMemoryCardRepo(), memorySessionLister{}, time.Now))

	req := httptest.NewRequest(http.MethodGet, "/admin/api/control/authorizations/missing", nil)
	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerPreviewAndExecuteAction(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	repo := newMemoryCardRepo()
	repo.Save(context.Background(), cards.Card{Code: "AUTH-1", Status: cards.StatusActive, MachineID: "M1", CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), MaxSessions: 1})
	handler := NewHandler(NewService(repo, memorySessionLister{}, func() time.Time { return now }))

	previewReq := httptest.NewRequest(http.MethodPost, "/admin/api/control/authorizations/AUTH-1/impact", bytes.NewBufferString(`{"action":"archive_delete"}`))
	previewW := httptest.NewRecorder()
	handler.Handle(previewW, previewReq)
	if previewW.Code != http.StatusOK {
		t.Fatalf("expected preview 200, got %d: %s", previewW.Code, previewW.Body.String())
	}

	actionReq := httptest.NewRequest(http.MethodPost, "/admin/api/control/authorizations/AUTH-1/actions", bytes.NewBufferString(`{"action":"archive_delete"}`))
	actionW := httptest.NewRecorder()
	handler.Handle(actionW, actionReq)
	if actionW.Code != http.StatusOK {
		t.Fatalf("expected action 200, got %d: %s", actionW.Code, actionW.Body.String())
	}
	card, err := repo.Get(context.Background(), "AUTH-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if card.Status != cards.StatusArchived {
		t.Fatalf("status = %q", card.Status)
	}
}


func TestAuthorizationHandlersRejectWrongMethod(t *testing.T) {
	h := NewHandler(NewService(newMemoryCardRepo(), memorySessionLister{}, time.Now))
	tests := []struct {
		name   string
		method string
		path   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{name: "list", method: http.MethodPost, path: "/admin/api/control/authorizations", call: h.HandleList},
		{name: "detail", method: http.MethodPost, path: "/admin/api/control/authorizations/AUTH-1", call: h.HandleDetail},
		{name: "impact", method: http.MethodGet, path: "/admin/api/control/authorizations/AUTH-1/impact", call: h.HandleImpact},
		{name: "action", method: http.MethodGet, path: "/admin/api/control/authorizations/AUTH-1/actions", call: h.HandleAction},
		{name: "bulk impact", method: http.MethodGet, path: "/admin/api/control/authorizations/bulk/impact", call: h.HandleBulkImpact},
		{name: "bulk action", method: http.MethodGet, path: "/admin/api/control/authorizations/bulk/actions", call: h.HandleBulkAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			tt.call(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
			}
		})
	}
}

func TestAuthorizationHandlersRejectInvalidJSONWithoutLeak(t *testing.T) {
	h := NewHandler(NewService(newMemoryCardRepo(), memorySessionLister{}, time.Now))
	tests := []struct {
		name string
		path string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "impact", path: "/admin/api/control/authorizations/AUTH-1/impact", call: h.HandleImpact},
		{name: "action", path: "/admin/api/control/authorizations/AUTH-1/actions", call: h.HandleAction},
		{name: "bulk impact", path: "/admin/api/control/authorizations/bulk/impact", call: h.HandleBulkImpact},
		{name: "bulk action", path: "/admin/api/control/authorizations/bulk/actions", call: h.HandleBulkAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString("{"))
			rec := httptest.NewRecorder()
			tt.call(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "unexpected") {
				t.Fatalf("leaked internal detail: %s", rec.Body.String())
			}
		})
	}
}
