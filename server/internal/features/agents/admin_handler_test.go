package agents

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminAgentHandlersRejectWrongMethod(t *testing.T) {
	h := &AdminHandler{}
	tests := []struct {
		name   string
		method string
		path   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{name: "list", method: http.MethodPost, path: "/admin/api/agents", call: h.HandleAdminListAgents},
		{name: "update", method: http.MethodGet, path: "/admin/api/agent/update", call: h.HandleAdminUpdateAgent},
		{name: "cards", method: http.MethodPost, path: "/admin/api/agent/cards", call: h.HandleAdminAgentCards},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			tt.call(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status=%d want %d body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
			}
		})
	}
}

func TestAdminUpdateAgentRejectsInvalidJSONWithoutLeak(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/agent/update", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	h.HandleAdminUpdateAgent(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "unexpected") {
		t.Fatalf("leaked: %s", rec.Body.String())
	}
}
