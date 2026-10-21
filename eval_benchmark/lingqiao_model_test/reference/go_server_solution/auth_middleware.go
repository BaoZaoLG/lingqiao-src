package server

// ============================================================================
// auth_middleware.go — 参考解答: HMAC 鉴权中间件（完全修复版）
// ============================================================================

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// 修复: 签名有效期窗口设为 5 分钟
const SignatureWindow = 5 * time.Minute

// NonceTracker 用于记录已使用的 nonce，防止重放攻击
type NonceTracker struct {
	mu     sync.Mutex
	seen   map[string]time.Time
	ttl    time.Duration
	lastGC time.Time
}

// NewNonceTracker 创建一个 nonce 追踪器
func NewNonceTracker(ttl time.Duration) *NonceTracker {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &NonceTracker{
		seen:   make(map[string]time.Time),
		ttl:    ttl,
		lastGC: time.Now(),
	}
}

// CheckAndRecord 检查 nonce 是否已被使用，未使用则记录
// 修复: 完整实现 nonce 去重逻辑
func (t *NonceTracker) CheckAndRecord(nonce string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 定期清理过期 nonce（每分钟一次）
	now := time.Now()
	if now.Sub(t.lastGC) > time.Minute {
		t.cleanup(now)
		t.lastGC = now
	}

	// 检查是否已存在
	if ts, exists := t.seen[nonce]; exists {
		if now.Sub(ts) < t.ttl {
			return false // 重放攻击
		}
		// 已过期，可以重用
	}

	// 记录 nonce
	t.seen[nonce] = now
	return true
}

// cleanup 清理过期的 nonce 记录（必须在持锁情况下调用）
func (t *NonceTracker) cleanup(now time.Time) {
	for nonce, ts := range t.seen {
		if now.Sub(ts) > t.ttl {
			delete(t.seen, nonce)
		}
	}
}

// Cleanup 公开的清理方法
func (t *NonceTracker) Cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cleanup(time.Now())
}

// Size 返回当前记录的 nonce 数量
func (t *NonceTracker) Size() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.seen)
}

// Authenticator 处理 HMAC 签名验证
type Authenticator struct {
	secret       []byte
	nonceTracker *NonceTracker
}

// NewAuthenticator 创建一个鉴权器
func NewAuthenticator(secret string) *Authenticator {
	return &Authenticator{
		secret:       []byte(secret),
		nonceTracker: NewNonceTracker(5 * time.Minute),
	}
}

// ComputeSignature 计算规范化字符串的 HMAC-SHA256 签名
func ComputeSignature(secret []byte, clientID, timestamp, nonce, bodyHash string) string {
	canonical := clientID + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC 验证签名是否有效
func (a *Authenticator) VerifyHMAC(clientID, timestamp, nonce, bodyHash, signature string) error {
	if clientID == "" || timestamp == "" || nonce == "" || signature == "" {
		return fmt.Errorf("missing required auth headers")
	}

	// 解析时间戳
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	// 修复: 检查请求时间是否在窗口内
	reqTime := time.Unix(ts, 0)
	diff := time.Since(reqTime)
	if diff > SignatureWindow || diff < -SignatureWindow {
		return fmt.Errorf("timestamp out of window: %v", diff)
	}

	// 修复: Nonce 去重检查
	if !a.nonceTracker.CheckAndRecord(nonce) {
		return fmt.Errorf("nonce already used (replay attack detected)")
	}

	// 计算期望签名
	expected := ComputeSignature(a.secret, clientID, timestamp, nonce, bodyHash)

	// 修复: 使用常量时间比较，防止时序攻击
	expectedBytes, _ := hex.DecodeString(expected)
	signatureBytes, _ := hex.DecodeString(signature)
	if !hmac.Equal(expectedBytes, signatureBytes) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// Middleware 返回一个 HTTP 鉴权中间件
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.Header.Get("X-Client-ID")
		timestamp := r.Header.Get("X-Timestamp")
		nonce := r.Header.Get("X-Nonce")
		signature := r.Header.Get("X-HMAC-Signature")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		bodyHashBytes := sha256.Sum256(body)
		bodyHash := hex.EncodeToString(bodyHashBytes[:])

		if err := a.VerifyHMAC(clientID, timestamp, nonce, bodyHash, signature); err != nil {
			http.Error(w, fmt.Sprintf("authentication failed: %v", err), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
