package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteOKIncludesStatusDataAndMeta(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteOK(rr, map[string]any{"name": "lingqiao"}, map[string]any{"page": 1})

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["status"] != "ok" {
		t.Fatalf("status = %v, want ok", got["status"])
	}
	if got["data"].(map[string]any)["name"] != "lingqiao" {
		t.Fatalf("data.name mismatch: %#v", got["data"])
	}
	if got["meta"].(map[string]any)["page"].(float64) != 1 {
		t.Fatalf("meta.page mismatch: %#v", got["meta"])
	}
}

func TestWriteErrorIncludesCodeAndMessage(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteError(rr, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["status"] != "error" {
		t.Fatalf("status = %v, want error", got["status"])
	}
	if got["code"] != "UNAUTHORIZED" {
		t.Fatalf("code = %v, want UNAUTHORIZED", got["code"])
	}
	if got["message"] != "unauthorized" {
		t.Fatalf("message = %v, want unauthorized", got["message"])
	}
}
