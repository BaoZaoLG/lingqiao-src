package chat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientChatHandlersRejectWrongMethod(t *testing.T) {
	h := NewHandler(nil, nil, nil)
	tests := []struct {
		name    string
		path    string
		handler http.HandlerFunc
	}{
		{name: "profile", path: "/api/v1/chat/profile", handler: h.HandleChatProfile},
		{name: "messages", path: "/api/v1/chat/messages", handler: h.HandleChatMessages},
		{name: "presence", path: "/api/v1/chat/presence", handler: h.HandleChatPresence},
		{name: "react", path: "/api/v1/chat/react", handler: h.HandleChatReact},
		{name: "send", path: "/api/v1/chat/send", handler: h.HandleChatSend},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
			}
		})
	}
}

func TestAdminChatHandlersRejectWrongMethod(t *testing.T) {
	h := &Handler{}
	tests := []struct {
		name   string
		method string
		path   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{name: "admin messages", method: http.MethodPost, path: "/admin/api/chat/messages", call: h.HandleAdminChatMessages},
		{name: "delete", method: http.MethodGet, path: "/admin/api/chat/delete", call: h.HandleChatDelete},
		{name: "system", method: http.MethodGet, path: "/admin/api/chat/system", call: h.HandleChatSystem},
		{name: "mute", method: http.MethodGet, path: "/admin/api/chat/mute", call: h.HandleChatMute},
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

func TestAdminChatHandlersRejectInvalidJSONWithoutLeak(t *testing.T) {
	svc := NewService(nil, time.Now)
	h := &Handler{chat: svc}
	tests := []struct {
		name string
		path string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "delete", path: "/admin/api/chat/delete", call: h.HandleChatDelete},
		{name: "system", path: "/admin/api/chat/system", call: h.HandleChatSystem},
		{name: "mute", path: "/admin/api/chat/mute", call: h.HandleChatMute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader("{"))
			rec := httptest.NewRecorder()
			tt.call(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "unexpected") {
				t.Fatalf("leaked internal detail: %s", rec.Body.String())
			}
		})
	}
}
