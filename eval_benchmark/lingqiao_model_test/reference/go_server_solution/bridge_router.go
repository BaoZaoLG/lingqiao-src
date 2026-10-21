package server

// ============================================================================
// bridge_router.go — 参考解答: Bridge 协议路由器（完全修复版）
// ============================================================================

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

const maxMessageBytes = 65536

// BridgeRouter 处理 Bridge 协议的命令分发
type BridgeRouter struct {
	handlers map[string]CommandHandler
	pending  map[string]chan *Message
	mu       sync.Mutex
	key      []byte
	seenIDs  map[string]time.Time
	seenMu   sync.Mutex
}

// CommandHandler 处理一个 Bridge 命令
type CommandHandler func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)

// NewBridgeRouter 创建一个新的路由器
func NewBridgeRouter(key []byte) *BridgeRouter {
	if len(key) != 32 {
		panic("BridgeRouter: key must be 32 bytes")
	}
	return &BridgeRouter{
		handlers: make(map[string]CommandHandler),
		pending:  make(map[string]chan *Message),
		key:      key,
		seenIDs:  make(map[string]time.Time),
	}
}

// RegisterHandler 注册一个命令处理器
func (r *BridgeRouter) RegisterHandler(method string, handler CommandHandler) {
	r.mu.Lock()
	r.handlers[method] = handler
	r.mu.Unlock()
}

// DispatchMessage 解密并分发一条 Bridge 消息
func (r *BridgeRouter) DispatchMessage(encryptedData []byte) ([]byte, error) {
	plaintext, err := r.DecryptBridgeMessage(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(plaintext, &msg); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if msg.Version != 2 {
		return nil, fmt.Errorf("unsupported protocol version: %d", msg.Version)
	}

	// 修复: 请求 ID 去重检查
	if msg.ID != "" {
		r.seenMu.Lock()
		if _, seen := r.seenIDs[msg.ID]; seen {
			r.seenMu.Unlock()
			return nil, fmt.Errorf("duplicate request ID: %s", msg.ID)
		}
		r.seenIDs[msg.ID] = time.Now()
		// 清理过期 ID（超过 10 分钟）
		now := time.Now()
		for id, ts := range r.seenIDs {
			if now.Sub(ts) > 10*time.Minute {
				delete(r.seenIDs, id)
			}
		}
		r.seenMu.Unlock()
	}

	switch msg.Type {
	case "request":
		return r.handleRequest(msg)
	case "response":
		r.handleResponse(msg)
		return nil, nil
	default:
		// 修复: 添加 default 分支
		return nil, fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleRequest 处理一条请求消息
func (r *BridgeRouter) handleRequest(msg Message) ([]byte, error) {
	r.mu.Lock()
	handler, ok := r.handlers[msg.Method]
	r.mu.Unlock()

	var response Message
	response.Version = 2
	response.Type = "response"
	response.ID = msg.ID

	if !ok {
		// 修复: 设置正确的错误码
		response.Error = &ErrorInfo{
			Code:    "METHOD_NOT_FOUND",
			Message: fmt.Sprintf("unknown method: %s", msg.Method),
		}
	} else {
		result, err := handler(context.Background(), msg.Params)
		if err != nil {
			response.Error = &ErrorInfo{
				Code:    "METHOD_FAILED",
				Message: err.Error(),
			}
		} else {
			response.Params = result
		}
	}

	respJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshal response failed: %w", err)
	}

	encrypted, err := r.EncryptBridgeMessage(respJSON)
	if err != nil {
		return nil, fmt.Errorf("encrypt response failed: %w", err)
	}

	return encrypted, nil
}

// handleResponse 处理来自客户端的响应消息
func (r *BridgeRouter) handleResponse(msg Message) {
	r.mu.Lock()
	ch, ok := r.pending[msg.ID]
	r.mu.Unlock()

	if ok {
		// 修复: 使用 select + 超时防止永久阻塞
		select {
		case ch <- &msg:
		case <-time.After(5 * time.Second):
			log.Printf("[BRIDGE] Response delivery timed out for ID: %s", msg.ID)
		}
	}
}

// SendRequest 发送请求并等待响应（带超时和重试）
func (r *BridgeRouter) SendRequest(method string, params json.RawMessage) (*Message, error) {
	id := fmt.Sprintf("req_%d", time.Now().UnixNano())

	msg := Message{
		Version: 2,
		Type:    "request",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// 修复: 使用带缓冲的 channel
	respCh := make(chan *Message, 1)

	r.mu.Lock()
	r.pending[id] = respCh
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.pending, id)
		r.mu.Unlock()
	}()

	reqJSON, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	_ = reqJSON // 在实际系统中发送加密消息

	// 修复: 带超时和指数退避重试（3 次）
	timeout := 3 * time.Second
	for attempt := 0; attempt < 3; attempt++ {
		select {
		case resp := <-respCh:
			return resp, nil
		case <-time.After(timeout):
			timeout *= 2 // 指数退避
			log.Printf("[BRIDGE] Request %s attempt %d timed out, retrying...", id, attempt+1)
		}
	}

	return nil, fmt.Errorf("request %s timed out after 3 retries", id)
}

// DecryptBridgeMessage 解密 Bridge 消息（完整实现）
func (r *BridgeRouter) DecryptBridgeMessage(data []byte) ([]byte, error) {
	// 最小长度: 4(header) + 12(nonce) + 16(tag)
	minLen := 4 + 12 + 16
	if len(data) < minLen {
		return nil, fmt.Errorf("ciphertext too short: %d bytes", len(data))
	}

	// 读取 payload length
	payloadLen := binary.BigEndian.Uint32(data[:4])

	// 验证长度一致性
	// data 布局: [4 header][12 nonce][ciphertext + tag]
	// ciphertext 长度 = payloadLen + 16(tag)
	expectedTotal := 4 + 12 + int(payloadLen) + 16
	if len(data) != expectedTotal {
		return nil, fmt.Errorf("length header mismatch: declared %d, actual data %d bytes", payloadLen, len(data)-4-12-16)
	}

	nonce := data[4:16]
	ciphertext := data[16:] // 包含 tag

	// 创建 AES-GCM
	block, err := aes.NewCipher(r.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM failed: %w", err)
	}

	// 解密并验证
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: authentication error")
	}

	return plaintext, nil
}

// EncryptBridgeMessage 加密 Bridge 消息（完整实现）
func (r *BridgeRouter) EncryptBridgeMessage(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("plaintext is empty")
	}
	if len(plaintext) > maxMessageBytes {
		return nil, fmt.Errorf("plaintext too large: %d > %d", len(plaintext), maxMessageBytes)
	}

	block, err := aes.NewCipher(r.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM failed: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce failed: %w", err)
	}

	// 加密 (Seal 会在密文后追加 tag)
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// 构造输出: [4 bytes length][12 bytes nonce][ciphertext+tag]
	result := make([]byte, 4+12+len(ciphertext))
	binary.BigEndian.PutUint32(result[:4], uint32(len(plaintext)))
	copy(result[4:16], nonce)
	copy(result[16:], ciphertext)

	return result, nil
}
