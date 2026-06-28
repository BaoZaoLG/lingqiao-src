package httpapi

import (
	"encoding/json"
	"net/http"
)

// OKResponse ...
type OKResponse struct {
	Status string         `json:"status"`
	Data   map[string]any `json:"data,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// ProblemDetails implements RFC 9457
type ProblemDetails struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteJSON ...
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// WriteOK ...
func WriteOK(w http.ResponseWriter, data map[string]any, meta map[string]any) {
	WriteJSON(w, http.StatusOK, OKResponse{
		Status: "ok",
		Data:   data,
		Meta:   meta,
	})
}

// WriteError ...
func WriteError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type:   "https://lingqiao.local/problems/" + code,
		Title:  http.StatusText(status),
		Status: status,
		Detail: message,
		Code:   code,
	})
}
