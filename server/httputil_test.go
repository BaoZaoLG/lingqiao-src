package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWriteProblemUsesRFC9457Shape(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/activate", nil)
	rec := httptest.NewRecorder()

	writeProblem(rec, req, http.StatusForbidden, "CARD_EXPIRED", "Card expired", "卡密已过期")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var body problemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body.Type != "https://lingqiao.local/problems/card-expired" {
		t.Fatalf("Type = %q", body.Type)
	}
	if body.Title != "Card expired" {
		t.Fatalf("Title = %q", body.Title)
	}
	if body.Status != http.StatusForbidden {
		t.Fatalf("Status = %d", body.Status)
	}
	if body.Detail != "卡密已过期" {
		t.Fatalf("Detail = %q", body.Detail)
	}
	if body.Instance != "/api/v1/activate" {
		t.Fatalf("Instance = %q", body.Instance)
	}
	if body.Code != "CARD_EXPIRED" {
		t.Fatalf("Code = %q", body.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(1*time.Minute, 3)
	key := "127.0.0.1"

	// First 3 requests should be allowed
	if !rl.allow(key) {
		t.Fatal("1st request should be allowed")
	}
	if !rl.allow(key) {
		t.Fatal("2nd request should be allowed")
	}
	if !rl.allow(key) {
		t.Fatal("3rd request should be allowed")
	}

	// 4th request should be blocked
	if rl.allow(key) {
		t.Fatal("4th request should be blocked")
	}

	// Wait for window to pass? Can't really wait 1 min in test easily without mocking time.
	// But we can test the `clear` function
	rl.clear(key)
	if !rl.allow(key) {
		t.Fatal("request after clear should be allowed")
	}
}
