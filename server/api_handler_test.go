package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIHandlerAnnouncementRejectsNonGET(t *testing.T) {
	h := &APIHandler{}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/announcement", nil)
	rec := httptest.NewRecorder()

	h.HandleAnnouncement(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
	}
}

func TestAPIHandlerDllDownloadRejectsNonGET(t *testing.T) {
	h := &APIHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dll", nil)
	rec := httptest.NewRecorder()

	h.HandleDllDownload(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
	}
}

func TestHealthHandlerRejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
	}
}
