package server

// ============================================================================
// bridge_router.go — Bridge 协议路由分发器
//
// 接收来自客户端的加密 Bridge 消息，解密后根据 method 分发到对应处理器，
// 并将处理结果加密后返回。
//
// 已知缺陷:
//   - switch 缺少 default 分支，未知命令静默丢弃
//   - 响应 channel 无超时，未响应请求永久阻塞
//   - 缺少请求 ID 去重
//   - 加解密函数未实现
// ============================================================================

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// BridgeRouter 处理 Bridge 协议的命令分发
type BridgeRouter struct {
	handlers   map[string]CommandHandler
	pending    map[string]chan *Message // 等待响应的请求
	mu         sync.Mutex
	key        []byte // AES-256 密钥 (32 bytes)
	seenIDs    map[string]bool
	seenMu     sync.Mutex
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
		seenIDs:  make(map[string]bool),
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
	// 解密消息
	plaintext, err := r.DecryptBridgeMessage(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}

	// 解析 JSON
	var msg Message
	if err := json.Unmarshal(plaintext, &msg); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// 验证协议版本
	if msg.Version != 2 {
		return nil, fmt.Errorf("unsupported protocol version: %d", msg.Version)
	}

	// BUG: 缺少请求 ID 去重检查
	// TODO: 检查 msg.ID 是否已经处理过
	//       如果是重复请求，直接返回之前的响应或错误
	//       需要使用 r.seenIDs 和 r.seenMu 实现

	// 根据消息类型分发
	switch msg.Type {
	case "request":
		return r.handleRequest(msg)
	case "response":
		r.handleResponse(msg)
		return nil, nil
	// BUG: 缺少 default 分支
	// 未知的消息类型被静默丢弃，不返回错误
	// TODO: 添加 default 分支，返回 METHOD_NOT_FOUND 错误
	}

	return nil, nil
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
		// BUG: switch 缺少对未知方法的处理
		// 当前只是返回空响应，没有设置错误码
		// TODO: 设置 response.Error 为 METHOD_NOT_FOUND
		log.Printf("[BRIDGE] Unknown method: %s", msg.Method)
	} else {
		// 执行处理器
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

	// 序列化响应
	respJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshal response failed: %w", err)
	}

	// 加密响应
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
		// BUG: 没有超时保护，如果 ch 已满或无人接收，会永久阻塞
		// TODO: 使用 select + time.After 添加超时
		ch <- &msg
	}
}

// SendRequest 发送一条请求并等待响应
func (r *BridgeRouter) SendRequest(method string, params json.RawMessage) (*Message, error) {
	id := fmt.Sprintf("req_%d", time.Now().UnixNano())

	msg := Message{
		Version: 2,
		Type:    "request",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// 创建响应 channel
	// BUG: channel 无缓冲，如果响应在我们还没 select 时到达，会丢失
	respCh := make(chan *Message)

	r.mu.Lock()
	r.pending[id] = respCh
	r.mu.Unlock()

	// 序列化并加密请求
	reqJSON, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	_ = reqJSON // TODO: 实际发送加密消息

	// BUG: 等待响应没有超时机制，如果对端不回复会永久阻塞
	// TODO: 使用 select + time.After 实现超时
	//       超时后应从 r.pending 中移除，并实现重试机制（3 次，指数退避）
	resp := <-respCh

	// 清理
	r.mu.Lock()
	delete(r.pending, id)
	r.mu.Unlock()

	return resp, nil
}

// ============================================================================
// Bridge 消息加解密
//
// 线格式 (Wire Format):
//   [4 bytes: payload length (big-endian)]
//   [12 bytes: AES-GCM nonce]
//   [N bytes: AES-GCM encrypted data]
//   [GCM tag is appended to ciphertext by Go's crypto/aes GCM Seal]
// ============================================================================

// DecryptBridgeMessage 解密一条 Bridge 消息
func (r *BridgeRouter) DecryptBridgeMessage(data []byte) ([]byte, error) {
	// TODO: 实现 AES-256-GCM 解密
	//
	// 步骤:
	// 1. 检查 data 长度 >= 4 + 12 + 16（header + nonce + tag 最小长度）
	// 2. 读取前 4 字节获取 payload length (big-endian uint32)
	// 3. 验证 payload length 与 data 剩余长度匹配
	// 4. 提取 nonce (12 bytes)
	// 5. 提取 ciphertext+tag (剩余部分)
	// 6. 使用 r.key 创建 AES-GCM cipher
	// 7. 调用 gcm.Open() 解密并验证
	//
	// 错误处理:
	//   - data 太短 -> "ciphertext too short"
	//   - length 不匹配 -> "length header mismatch"
	//   - 解密失败 -> "decryption failed: authentication error"

	_ = aes.NewCipher    // hint: use crypto/aes
	_ = cipher.NewGCM    // hint: use cipher.NewGCM
	_ = binary.BigEndian // hint: use encoding/binary

	return nil, fmt.Errorf("NOT IMPLEMENTED")
}

// EncryptBridgeMessage 加密一条 Bridge 消息
func (r *BridgeRouter) EncryptBridgeMessage(plaintext []byte) ([]byte, error) {
	// TODO: 实现 AES-256-GCM 加密
	//
	// 步骤:
	// 1. 验证 plaintext 不为空且长度不超过 LQ_BRIDGE_MAX_MESSAGE_BYTES (65536)
	// 2. 使用 r.key 创建 AES-GCM cipher
	// 3. 生成 12 字节随机 nonce
	// 4. 调用 gcm.Seal() 加密
	// 5. 构造输出: [4 bytes length] + [12 bytes nonce] + [ciphertext+tag]
	//    其中 length = len(plaintext)
	// 6. 返回完整的线格式数据

	return nil, fmt.Errorf("NOT IMPLEMENTED")
}
