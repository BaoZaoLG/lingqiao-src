package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// short returns the first n characters of s, or all of s if shorter.
func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ── JSON Response Helpers ────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func mapToErrorCode(status int, msg string) string {
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
		if status == 401 || status == 403 {
			return "MACHINE_MISMATCH"
		}
		return "CARD_BOUND_TO_OTHER_MACHINE"
	case strings.Contains(msgStr, "达到最大同时登录数"):
		return "MAX_SESSIONS_REACHED"
	case strings.Contains(msgStr, "无效的会话") || strings.Contains(msgStr, "session not found"):
		return "SESSION_NOT_FOUND"
	default:
		if status == 400 {
			return "BAD_REQUEST"
		} else if status == 401 {
			return "UNAUTHORIZED"
		} else if status == 403 {
			return "FORBIDDEN"
		} else if status == 404 {
			return "NOT_FOUND"
		} else if status == 500 {
			return "INTERNAL_ERROR"
		}
		return "UNKNOWN_ERROR"
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	code := mapToErrorCode(status, msg)
	problemType := "https://lingqiao.local/problems/" + strings.ToLower(strings.ReplaceAll(code, "_", "-"))
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemResponse{
		Type:     problemType,
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   msg,
		Code:     code,
	})
}

func writeOK(w http.ResponseWriter, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["status"] = "ok"
	writeJSON(w, data)
}

// ── Request Parsing ──────────────────────────────────────────────────────────

func readJSON(r *http.Request, dst interface{}) error {
	body, err := readBody(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	return body, nil
}

func jsonUnmarshal(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// verifyRequestHMAC verifies HMAC from standard request headers.
func verifyRequestHMAC(r *http.Request, body string) error {
	sig := r.Header.Get("X-HMAC-Signature")
	clientID := r.Header.Get("X-Client-ID")
	timestamp := r.Header.Get("X-Timestamp")
	nonce := r.Header.Get("X-Nonce")
	if sig == "" || clientID == "" || timestamp == "" || nonce == "" {
		return fmt.Errorf("missing HMAC signature, client ID, timestamp, or nonce")
	}
	if err := VerifyHMAC(clientID, body, sig, timestamp, nonce); err != nil {
		log.Printf("[AUTH] HMAC verification failed from %s: %v", getClientIP(r), err)
		return fmt.Errorf("HMAC verification failed")
	}
	return nil
}

// requireMethod returns an error response if the request method doesn't match.
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

// ── Network Helpers ──────────────────────────────────────────────────────────

func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Only trust X-Forwarded-For from localhost (where nginx runs)
	if host == "127.0.0.1" || host == "::1" {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	return host
}

// ── Generic Rate Limiter ─────────────────────────────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	window   time.Duration
	limit    int
}

func newRateLimiter(window time.Duration, limit int) *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string][]time.Time),
		window:   window,
		limit:    limit,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	valid := make([]time.Time, 0, len(rl.attempts[key]))
	for _, t := range rl.attempts[key] {
		if now.Sub(t) < rl.window {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.attempts[key] = valid
		return false
	}
	if len(valid) == 0 {
		delete(rl.attempts, key)
	} else {
		rl.attempts[key] = append(valid, now)
	}
	return true
}

func (rl *rateLimiter) clear(key string) {
	rl.mu.Lock()
	delete(rl.attempts, key)
	rl.mu.Unlock()
}

// ── Common Types ─────────────────────────────────────────────────────────────

type errorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type problemResponse struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	if title == "" {
		title = http.StatusText(status)
	}
	problemType := "https://lingqiao.local/problems/" + strings.ToLower(strings.ReplaceAll(code, "_", "-"))
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemResponse{
		Type:     problemType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
		Code:     code,
	})
}

// ── Security Headers ─────────────────────────────────────────────────────────

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
}

func setCSP(w http.ResponseWriter, extraConnectSrc ...string) {
	connectSrc := "'self'"
	if len(extraConnectSrc) > 0 {
		connectSrc = strings.Join(append([]string{"'self'"}, extraConnectSrc...), " ")
	}
	w.Header().Set("Content-Security-Policy",
		fmt.Sprintf("default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src %s", connectSrc))
}
