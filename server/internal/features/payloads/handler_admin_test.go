package payloads

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminPayloadHandlersRejectWrongMethod(t *testing.T) {
	h := &Handler{}
	tests := []struct {
		name   string
		method string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{name: "list", method: http.MethodPost, call: h.HandleAdminList},
		{name: "manage", method: http.MethodGet, call: h.HandleAdminManage},
		{name: "dll upload", method: http.MethodGet, call: h.HandleAdminDLLUpload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			rec := httptest.NewRecorder()
			tt.call(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status=%d want %d body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
			}
		})
	}
}

func TestAdminPayloadManageRejectsInvalidJSONWithoutLeak(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/payloads/manage", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	h.HandleAdminManage(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "unexpected") {
		t.Fatalf("leaked: %s", rec.Body.String())
	}
}
