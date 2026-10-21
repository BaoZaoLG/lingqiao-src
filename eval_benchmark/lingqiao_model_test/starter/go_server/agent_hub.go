package server

// ============================================================================
// agent_hub.go — 客户端连接池管理（Agent Hub）
//
// 管理所有代理客户端的 WebSocket 长连接。
// 提供注册、注销、广播等核心操作。
//
// 已知缺陷:
//   - clients map 在无锁情况下被并发读写
//   - Broadcast 在迭代 map 时其他 goroutine 可能并发修改
//   - send channel 无容量限制，高负载时 goroutine 泄漏
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Message 表示一条 Bridge 协议消息
type Message struct {
	Version int             `json:"version"`
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *ErrorInfo      `json:"error,omitempty"`
}

// ErrorInfo 表示协议错误信息
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Client 表示一个已连接的代理客户端
type Client struct {
	ID       string
	AgentID  string
	Send     chan []byte // BUG: 无缓冲 channel，发送阻塞时 goroutine 泄漏
	JoinedAt time.Time
}

// AgentHub 管理所有活跃的客户端连接
type AgentHub struct {
	// BUG: clients map 没有锁保护，并发读写会导致 fatal error: concurrent map read and map write
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.Mutex // TODO: 这个锁声明了但从未使用
}

// NewAgentHub 创建一个新的 AgentHub
func NewAgentHub() *AgentHub {
	return &AgentHub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

// Register 注册一个新的客户端连接
func (h *AgentHub) Register(client *Client) {
	// BUG: 直接写入 map，没有加锁
	// 当多个 goroutine 同时调用 Register 时会产生 data race
	h.clients[client.ID] = client
	log.Printf("[HUB] Client registered: %s (agent: %s)", client.ID, client.AgentID)
}

// Unregister 注销一个客户端连接
func (h *AgentHub) Unregister(clientID string) {
	// BUG: 直接操作 map，没有加锁
	client, ok := h.clients[clientID]
	if !ok {
		return
	}
	delete(h.clients, clientID)
	close(client.Send)
	log.Printf("[HUB] Client unregistered: %s", clientID)
}

// Broadcast 向所有已连接客户端广播消息
func (h *AgentHub) Broadcast(message []byte) {
	// BUG: 在迭代 map 的同时，其他 goroutine 可能正在调用 Register/Unregister 修改 map
	// 这会导致 "concurrent map iteration and map write" panic
	for id, client := range h.clients {
		// BUG: Send channel 是无缓冲的（make(chan []byte)），
		// 如果客户端消费慢，这里会永久阻塞，导致广播卡死
		select {
		case client.Send <- message:
			// 发送成功
		default:
			// TODO: 发送失败时应该怎么处理？
			// 当前直接丢弃消息，但应该考虑客户端断开清理
			log.Printf("[HUB] Client %s send buffer full, message dropped", id)
		}
	}
}

// BroadcastToAgent 向指定代理的所有客户端广播消息
func (h *AgentHub) BroadcastToAgent(agentID string, message []byte) {
	// BUG: 同上，无锁访问 map
	for _, client := range h.clients {
		if client.AgentID != agentID {
			continue
		}
		select {
		case client.Send <- message:
		default:
			log.Printf("[HUB] Agent %s client send full", agentID)
		}
	}
}

// ClientCount 返回当前连接的客户端数量
func (h *AgentHub) ClientCount() int {
	// BUG: 无锁读取 map 的 len
	return len(h.clients)
}

// GetClient 获取指定 ID 的客户端
func (h *AgentHub) GetClient(clientID string) (*Client, bool) {
	// BUG: 无锁读取
	client, ok := h.clients[clientID]
	return client, ok
}

// SendToClient 向指定客户端发送消息
func (h *AgentHub) SendToClient(clientID string, message []byte) error {
	client, ok := h.GetClient(clientID)
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	// BUG: 没有超时机制，如果客户端的 Send channel 满了会永久阻塞
	client.Send <- message
	return nil
}

// 以下是未使用的死代码（测试模型是否会清理）
var _ = sync.Mutex{}
