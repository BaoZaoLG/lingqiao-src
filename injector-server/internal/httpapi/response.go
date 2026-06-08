package httpapi

import (
	"encoding/json"
	"net/http"
)

type OKResponse struct {
	Status string         `json:"status"`
	Data   map[string]any `json:"data,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteOK(w http.ResponseWriter, data map[string]any, meta map[string]any) {
	WriteJSON(w, http.StatusOK, OKResponse{
		Status: "ok",
		Data:   data,
		Meta:   meta,
	})
}

func WriteError(w http.ResponseWriter, status int, code string, message string) {
	WriteJSON(w, status, ErrorResponse{
		Status:  "error",
		Code:    code,
		Message: message,
	})
}
