package server

// ============================================================================
// auth_middleware.go — HMAC 签名鉴权中间件
//
// 验证客户端请求的 HMAC-SHA256 签名，防止请求伪造和重放攻击。
//
// 签名算法:
//   canonical = clientID + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash
//   signature = hex(HMAC-SHA256(secret, canonical))
//
// 已知缺陷:
//   - 缺少 Nonce 去重（重放攻击）
//   - 时间戳校验窗口为 0（不校验）
//   - 使用 == 比较签名（时序攻击）
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

// 签名有效期窗口
// BUG: 窗口设为 0 意味着不做时间戳校验，任意旧请求都能重放
const SignatureWindow = 0 * time.Second

// TODO: 正确的窗口应该是 5 分钟
// const SignatureWindow = 5 * time.Minute

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
// @return true 表示 nonce 是新的（可以接受请求）
//
//	false 表示 nonce 已被使用（重放攻击）
func (t *NonceTracker) CheckAndRecord(nonce string) bool {
	// TODO: 实现 nonce 去重逻辑
	//
	// 步骤:
	// 1. 加锁保护 seen map
	// 2. 检查 nonce 是否已在 seen 中且未过期
	//    - 如果存在且未过期 -> 返回 false（重放）
	// 3. 记录 nonce 及当前时间
	// 4. 定期清理过期的 nonce（如每分钟一次）
	// 5. 返回 true
	//
	// 注意: 必须是并发安全的，会被多个 goroutine 同时调用

	// BUG: 当前实现直接返回 true，完全不做去重检查
	return true
}

// Cleanup 清理过期的 nonce 记录
func (t *NonceTracker) Cleanup() {
	// TODO: 实现过期 nonce 清理
	// 遍历 seen map，删除超过 ttl 的记录
}

// Size 返回当前记录的 nonce 数量（用于测试）
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

	// BUG: 时间戳窗口校验 — SignatureWindow 为 0，条件永远不成立，
	// 意味着任意时间的请求都会通过校验（可重放几年前的请求）
	// TODO: 修复窗口值，并检查请求时间是否在 [now-window, now+window] 内
	reqTime := time.Unix(ts, 0)
	if SignatureWindow > 0 {
		diff := time.Since(reqTime)
		if diff > SignatureWindow || diff < -SignatureWindow {
			return fmt.Errorf("timestamp out of window: %v", diff)
		}
	}
	_ = reqTime // suppress unused warning

	// BUG: 缺少 Nonce 去重检查
	// TODO: 调用 a.nonceTracker.CheckAndRecord(nonce)
	//       如果返回 false，返回 "nonce already used" 错误
	_ = a.nonceTracker // suppress unused warning

	// 计算期望签名
	expected := ComputeSignature(a.secret, clientID, timestamp, nonce, bodyHash)

	// BUG: 使用 == 直接比较签名，存在时序攻击（timing attack）风险
	// 攻击者可以通过测量响应时间逐字节猜测签名
	// TODO: 使用 hmac.Equal() 或 subtle.ConstantTimeCompare() 替换
	if signature == expected {
		return nil
	}
	return fmt.Errorf("signature mismatch")
}

// Middleware 返回一个 HTTP 鉴权中间件
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.Header.Get("X-Client-ID")
		timestamp := r.Header.Get("X-Timestamp")
		nonce := r.Header.Get("X-Nonce")
		signature := r.Header.Get("X-HMAC-Signature")

		// 读取 body 并计算 hash
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
