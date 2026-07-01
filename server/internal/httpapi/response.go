package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

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

// MapToErrorCode infers a standard error code from an error message.
func MapToErrorCode(status int, msg string) string {
	msgStr := strings.ToLower(msg)
	switch {
	case strings.Contains(msgStr, "invalid json"):
		return "INVALID_JSON"
	case strings.Contains(msgStr, "method not allowed"):
		return "METHOD_NOT_ALLOWED"
	case strings.Contains(msgStr, "hmac"):
		if strings.Contains(msgStr, "missing") {
			return "HMAC_MISSING"
		}
		return "HMAC_INVALID"
	case strings.Contains(msgStr, "卡密不存在") || strings.Contains(msgStr, "card not found"):
		return "CARD_NOT_FOUND"
	case strings.Contains(msgStr, "黑名单"):
		return "CARD_BLACKLISTED"
	case strings.Contains(msgStr, "已被禁用"):
		return "CARD_DISABLED"
	case strings.Contains(msgStr, "已过期"):
		if strings.Contains(msgStr, "会话") {
			return "SESSION_EXPIRED"
		}
		return "CARD_EXPIRED"
	case strings.Contains(msgStr, "绑定了其他设备") || strings.Contains(msgStr, "machine mismatch"):
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			return "MACHINE_MISMATCH"
		}
		return "CARD_BOUND_TO_OTHER_MACHINE"
	case strings.Contains(msgStr, "达到最大同时登录数"):
		return "MAX_SESSIONS_REACHED"
	case strings.Contains(msgStr, "无效的会话") || strings.Contains(msgStr, "session not found"):
		return "SESSION_NOT_FOUND"
	default:
		if status == http.StatusBadRequest {
			return "BAD_REQUEST"
		} else if status == http.StatusUnauthorized {
			return "UNAUTHORIZED"
		} else if status == http.StatusForbidden {
			return "FORBIDDEN"
		} else if status == http.StatusNotFound {
			return "NOT_FOUND"
		} else if status == http.StatusInternalServerError {
			return "INTERNAL_ERROR"
		}
		return "UNKNOWN_ERROR"
	}
}

// WriteOK writes a standard JSON success response.
func WriteOK(w http.ResponseWriter, data map[string]any) {
	if data == nil {
		data = make(map[string]any)
	}
	data["status"] = "ok"
	WriteJSON(w, http.StatusOK, data)
}

// WriteError writes an RFC 9457 Problem Details response, automatically mapping the error code.
func WriteError(w http.ResponseWriter, status int, message string) {
	code := MapToErrorCode(status, message)
	WriteProblem(w, nil, status, code, "", message)
}

// WriteProblem writes an RFC 9457 Problem Details response with explicit code, title, and instance support.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	if title == "" {
		title = http.StatusText(status)
	}
	instance := ""
	if r != nil {
		instance = r.URL.Path
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type:     "https://lingqiao.local/problems/" + strings.ToLower(strings.ReplaceAll(code, "_", "-")),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	})
}
