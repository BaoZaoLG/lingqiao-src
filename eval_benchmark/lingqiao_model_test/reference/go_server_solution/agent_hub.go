package server

// ============================================================================
// reference/c_hook_solution 对应的 Go 服务端参考解答
// ============================================================================
// 本文件为 agent_hub.go 的完整修复版本

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
	Send     chan []byte // 修复: 使用带缓冲的 channel
	JoinedAt time.Time
}

// AgentHub 管理所有活跃的客户端连接
type AgentHub struct {
	clients map[string]*Client
	mu      sync.RWMutex // 修复: 使用读写锁保护 clients map
}

// NewAgentHub 创建一个新的 AgentHub
func NewAgentHub() *AgentHub {
	return &AgentHub{
		clients: make(map[string]*Client),
	}
}

// Register 注册一个新的客户端连接
func (h *AgentHub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// 修复: 为 Send channel 设置缓冲区容量 (256)
	if client.Send == nil {
		client.Send = make(chan []byte, 256)
	}
	h.clients[client.ID] = client
	log.Printf("[HUB] Client registered: %s (agent: %s)", client.ID, client.AgentID)
}

// Unregister 注销一个客户端连接
func (h *AgentHub) Unregister(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
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
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, client := range h.clients {
		select {
		case client.Send <- message:
		default:
			log.Printf("[HUB] Client %s send buffer full, message dropped", id)
		}
	}
}

// BroadcastToAgent 向指定代理的所有客户端广播消息
func (h *AgentHub) BroadcastToAgent(agentID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
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
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetClient 获取指定 ID 的客户端
func (h *AgentHub) GetClient(clientID string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.clients[clientID]
	return client, ok
}

// SendToClient 向指定客户端发送消息（带超时）
func (h *AgentHub) SendToClient(clientID string, message []byte) error {
	client, ok := h.GetClient(clientID)
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	// 修复: 添加发送超时
	select {
	case client.Send <- message:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send to client %s timed out", clientID)
	}
}
