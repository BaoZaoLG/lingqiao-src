package cards

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
