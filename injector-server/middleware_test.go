package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminCORSDenyCrossOriginWhenOriginNotConfigured(t *testing.T) {
	t.Setenv("ADMIN_ORIGIN", "")
	req := httptest.NewRequest(http.MethodGet, "/admin/api/dashboard", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()

	corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestAdminCORSAllowsConfiguredOrigin(t *testing.T) {
	t.Setenv("ADMIN_ORIGIN", "https://admin.example")
	req := httptest.NewRequest(http.MethodGet, "/admin/api/dashboard", nil)
	req.Header.Set("Origin", "https://admin.example")
	rr := httptest.NewRecorder()

	corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
}

func TestClientAPICORSRemainsPublic(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/activate", nil)
	req.Header.Set("Origin", "https://client.example")
	rr := httptest.NewRecorder()

	corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
}
