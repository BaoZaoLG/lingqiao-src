package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lingqiao/server/internal/httpapi"
)

func TestWriteProblemUsesRFC9457Shape(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/activate", nil)
	rec := httptest.NewRecorder()

	httpapi.WriteProblem(rec, req, http.StatusForbidden, "CARD_EXPIRED", "Card expired", "卡密已过期")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json; charset=utf-8" {
		t.Errorf("content type = %q, want application/problem+json; charset=utf-8", ct)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	expectedType := "https://lingqiao.local/problems/card-expired"
	if tVal, ok := payload["type"].(string); !ok || tVal != expectedType {
		t.Errorf("type = %v, want %q", payload["type"], expectedType)
	}

	if tVal, ok := payload["title"].(string); !ok || tVal != "Card expired" {
		t.Errorf("title = %v, want %q", payload["title"], "Card expired")
	}

	if sVal, ok := payload["status"].(float64); !ok || int(sVal) != http.StatusForbidden {
		t.Errorf("status = %v, want %d", payload["status"], http.StatusForbidden)
	}

	if dVal, ok := payload["detail"].(string); !ok || dVal != "卡密已过期" {
		t.Errorf("detail = %v, want %q", payload["detail"], "卡密已过期")
	}

	if iVal, ok := payload["instance"].(string); !ok || iVal != "/api/v1/activate" {
		t.Errorf("instance = %v, want %q", payload["instance"], "/api/v1/activate")
	}
}
