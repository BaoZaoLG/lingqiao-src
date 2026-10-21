package cards

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setupHandlerTest(t *testing.T) (*Handler, *memoryRepo) {
	repo := newMemoryRepo()
	service := NewService(repo, func() (string, error) { return "TEST-CARD-123", nil }, time.Now)
	handler := NewHandler(service, repo)
	handler.SetAuditSink(func(action, card, detail string) {})
	return handler, repo
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return resp
}

func TestHandlerGenerateCard(t *testing.T) {
	h, _ := setupHandlerTest(t)
	body := []byte(`{"duration_hours": 24, "note": "test", "max_sessions": 2}`)
	req := httptest.NewRequest(http.MethodPost, "/api/card/generate", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.HandleGenerateCard(w, req)

	resp := parseResponse(t, w)
	if resp["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", resp["status"])
	}
	if resp["card"] == nil {
		t.Errorf("Expected 'card' in response")
	} else {
		cardMap := resp["card"].(map[string]interface{})
		if cardMap["code"] != "TEST-CARD-123" {
			t.Errorf("Expected code TEST-CARD-123, got %v", cardMap["code"])
		}
	}
}

func TestHandlerBatchGenerateCards(t *testing.T) {
	h, _ := setupHandlerTest(t)
	body := []byte(`{"duration_hours": 24, "count": 5, "max_sessions": 2}`)
	req := httptest.NewRequest(http.MethodPost, "/api/card/batch-generate", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.HandleBatchGenerateCards(w, req)

	resp := parseResponse(t, w)
	if resp["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", resp["status"])
	}
	if resp["count"].(float64) != 5 {
		t.Errorf("Expected count 5, got %v", resp["count"])
	}
	cards := resp["cards"].([]interface{})
	if len(cards) != 5 {
		t.Errorf("Expected 5 cards, got %v", len(cards))
	}
}

func TestHandlerUpdateCard_NotFound(t *testing.T) {
	h, _ := setupHandlerTest(t)
	body := []byte(`{"code": "NONEXISTENT", "action": "disable"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/card/update", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.HandleUpdateCard(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", w.Code)
	}
}

func TestHandlerListCards_NormalizesExpiredByTime(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepo()
	service := NewService(repo, nil, func() time.Time { return now })
	h := NewHandler(service, repo)
	repo.Save(context.Background(), Card{Code: "ACTIVE-FUTURE", Status: StatusActive, ExpiresAt: now.Add(time.Hour)})
	repo.Save(context.Background(), Card{Code: "ACTIVE-PAST", Status: StatusActive, ExpiresAt: now.Add(-time.Hour)})

	req := httptest.NewRequest(http.MethodGet, "/api/cards", nil)
	w := httptest.NewRecorder()
	h.HandleListCards(w, req)

	resp := parseResponse(t, w)
	cards := resp["cards"].([]interface{})
	statusByCode := map[string]string{}
	for _, item := range cards {
		card := item.(map[string]interface{})
		statusByCode[card["code"].(string)] = card["status"].(string)
	}
	if statusByCode["ACTIVE-PAST"] != string(StatusExpired) {
		t.Fatalf("expected ACTIVE-PAST to be returned as expired, got %q", statusByCode["ACTIVE-PAST"])
	}

	reqActive := httptest.NewRequest(http.MethodGet, "/api/cards?status=active", nil)
	wActive := httptest.NewRecorder()
	h.HandleListCards(wActive, reqActive)
	activeResp := parseResponse(t, wActive)
	activeCards := activeResp["cards"].([]interface{})
	if len(activeCards) != 1 || activeCards[0].(map[string]interface{})["code"] != "ACTIVE-FUTURE" {
		t.Fatalf("active filter should only return unexpired active cards, got %#v", activeCards)
	}

	reqExpired := httptest.NewRequest(http.MethodGet, "/api/cards?status=expired", nil)
	wExpired := httptest.NewRecorder()
	h.HandleListCards(wExpired, reqExpired)
	expiredResp := parseResponse(t, wExpired)
	expiredCards := expiredResp["cards"].([]interface{})
	if len(expiredCards) != 1 || expiredCards[0].(map[string]interface{})["code"] != "ACTIVE-PAST" {
		t.Fatalf("expired filter should include active cards past expires_at, got %#v", expiredCards)
	}
}

func TestHandlerUpdateCard_Delete(t *testing.T) {
	h, repo := setupHandlerTest(t)
	repo.Save(context.Background(), Card{Code: "DELETE-ME", Status: StatusActive, ExpiresAt: time.Now().Add(time.Hour)})

	body := []byte(`{"code": "DELETE-ME", "action": "delete"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/card/update", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	h.HandleUpdateCard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := repo.Get(context.Background(), "DELETE-ME"); err == nil {
		t.Fatal("card was not deleted")
	}
}

func TestHandlerBulkUpdateCardsDetailed_DryRun(t *testing.T) {
	h, repo := setupHandlerTest(t)
	repo.Save(context.Background(), Card{Code: "C1", Status: StatusActive})

	body := []byte(`{"codes": ["C1", "C2"], "action": "disable", "dry_run": true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/cards/bulk", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.HandleBulkCards(w, req)

	resp := parseResponse(t, w)
	if resp["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", resp["status"])
	}
	result := resp["result"].(map[string]interface{})
	if result["updated"].(float64) != 1 {
		t.Errorf("Expected result.updated=1, got %v", result["updated"])
	}
	if result["skipped"].(float64) != 1 { // C2 not found
		t.Errorf("Expected result.skipped=1, got %v", result["skipped"])
	}
}

func TestHandlerBulkUpdateCardsDetailed_Delete(t *testing.T) {
	h, repo := setupHandlerTest(t)
	repo.Save(context.Background(), Card{Code: "C1", Status: StatusActive})
	repo.Save(context.Background(), Card{Code: "C2", Status: StatusActive})

	body := []byte(`{"codes": ["C1", "C2"], "action": "delete"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/cards/bulk", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	h.HandleBulkCards(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	result := resp["result"].(map[string]interface{})
	if result["updated"].(float64) != 2 {
		t.Fatalf("expected 2 deleted items, got %#v", result)
	}
	if _, err := repo.Get(context.Background(), "C1"); err == nil {
		t.Fatal("C1 was not deleted")
	}
	if _, err := repo.Get(context.Background(), "C2"); err == nil {
		t.Fatal("C2 was not deleted")
	}
}

func TestHandlerBulkUpdateCardsDetailed_EmptyCodes(t *testing.T) {
	h, _ := setupHandlerTest(t)
	body := []byte(`{"codes": [], "action": "disable"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/cards/bulk", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.HandleBulkCards(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	if resp["detail"] != "codes is required" {
		t.Errorf("Expected detail 'codes is required', got %v", resp)
	}
}

func TestHandlerUpdateCardDetails_MaxSessionsBoundary(t *testing.T) {
	h, repo := setupHandlerTest(t)
	repo.Save(context.Background(), Card{Code: "C1", Status: StatusActive})

	// Test max_sessions = 0 (should fail)
	body0 := []byte(`{"code": "C1", "max_sessions": 0}`)
	req0 := httptest.NewRequest(http.MethodPost, "/api/card/update-details", bytes.NewBuffer(body0))
	w0 := httptest.NewRecorder()
	h.HandleUpdateCardDetails(w0, req0)
	if w0.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for max_sessions=0, got %d", w0.Code)
	}

	// Test max_sessions = -1 (should fail)
	bodyNeg := []byte(`{"code": "C1", "max_sessions": -1}`)
	reqNeg := httptest.NewRequest(http.MethodPost, "/api/card/update-details", bytes.NewBuffer(bodyNeg))
	wNeg := httptest.NewRecorder()
	h.HandleUpdateCardDetails(wNeg, reqNeg)
	if wNeg.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for max_sessions=-1, got %d", wNeg.Code)
	}

	// Test max_sessions = 1 (should succeed)
	body1 := []byte(`{"code": "C1", "max_sessions": 1}`)
	req1 := httptest.NewRequest(http.MethodPost, "/api/card/update-details", bytes.NewBuffer(body1))
	w1 := httptest.NewRecorder()
	h.HandleUpdateCardDetails(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for max_sessions=1, got %d", w1.Code)
	}
}

func TestCardHandlersRejectWrongMethod(t *testing.T) {
	h := NewHandler(nil, nil)
	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{name: "list cards", method: http.MethodPost, path: "/admin/api/cards", handler: h.HandleListCards},
		{name: "generate card", method: http.MethodGet, path: "/admin/api/card/generate", handler: h.HandleGenerateCard},
		{name: "batch generate", method: http.MethodGet, path: "/admin/api/card/batch-generate", handler: h.HandleBatchGenerateCards},
		{name: "update card", method: http.MethodGet, path: "/admin/api/card/update", handler: h.HandleUpdateCard},
		{name: "update details", method: http.MethodGet, path: "/admin/api/card/update-details", handler: h.HandleUpdateCardDetails},
		{name: "bulk cards", method: http.MethodGet, path: "/admin/api/cards/bulk", handler: h.HandleBulkCards},
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

func TestCardUpdateHandlersRejectInvalidJSONWithoutLeak(t *testing.T) {
	h := NewHandler(nil, nil)
	tests := []struct {
		name    string
		path    string
		handler http.HandlerFunc
	}{
		{name: "update card", path: "/admin/api/card/update", handler: h.HandleUpdateCard},
		{name: "update details", path: "/admin/api/card/update-details", handler: h.HandleUpdateCardDetails},
		{name: "bulk cards", path: "/admin/api/cards/bulk", handler: h.HandleBulkCards},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString("{"))
			rec := httptest.NewRecorder()
			tt.handler(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			body := rec.Body.String()
			if strings.Contains(body, "unexpected") || strings.Contains(body, "sql:") {
				t.Fatalf("leaked internal detail: %s", body)
			}
		})
	}
}

func TestHandleUpdateCardNotFoundSanitized(t *testing.T) {
	h, _ := setupHandlerTest(t)
	body := []byte(`{"code":"MISSING","action":"disable"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/card/update", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	h.HandleUpdateCard(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "card not found") {
		t.Fatalf("expected sanitized not found body, got %s", rec.Body.String())
	}
}
