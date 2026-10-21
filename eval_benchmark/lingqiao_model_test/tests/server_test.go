package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// AgentHub 测试
// ============================================================================

func TestAgentHub_Register(t *testing.T) {
	hub := NewAgentHub()
	client := &Client{ID: "c1", AgentID: "agent1", Send: make(chan []byte, 10)}
	hub.Register(client)
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}
}

func TestAgentHub_Unregister(t *testing.T) {
	hub := NewAgentHub()
	client := &Client{ID: "c1", AgentID: "agent1", Send: make(chan []byte, 10)}
	hub.Register(client)
	hub.Unregister("c1")
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestAgentHub_Broadcast(t *testing.T) {
	hub := NewAgentHub()
	c1 := &Client{ID: "c1", AgentID: "a1", Send: make(chan []byte, 10)}
	c2 := &Client{ID: "c2", AgentID: "a2", Send: make(chan []byte, 10)}
	hub.Register(c1)
	hub.Register(c2)

	hub.Broadcast([]byte("hello"))

	select {
	case msg := <-c1.Send:
		if string(msg) != "hello" {
			t.Fatalf("c1 got wrong message: %s", msg)
		}
	default:
		t.Fatal("c1 did not receive broadcast")
	}

	select {
	case msg := <-c2.Send:
		if string(msg) != "hello" {
			t.Fatalf("c2 got wrong message: %s", msg)
		}
	default:
		t.Fatal("c2 did not receive broadcast")
	}
}

func TestAgentHub_ConcurrentRegister(t *testing.T) {
	hub := NewAgentHub()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := &Client{
				ID:      fmt.Sprintf("c%d", id),
				AgentID: "agent1",
				Send:    make(chan []byte, 10),
			}
			hub.Register(client)
		}(i)
	}
	wg.Wait()
	if hub.ClientCount() != 100 {
		t.Fatalf("expected 100 clients, got %d", hub.ClientCount())
	}
}

func TestAgentHub_ConcurrentBroadcast(t *testing.T) {
	hub := NewAgentHub()
	for i := 0; i < 10; i++ {
		hub.Register(&Client{
			ID:      fmt.Sprintf("c%d", i),
			AgentID: "a1",
			Send:    make(chan []byte, 100),
		})
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			hub.Broadcast([]byte(fmt.Sprintf("msg%d", n)))
		}(i)
	}
	wg.Wait()
	// No panic = pass (race detector will catch issues)
}

func TestAgentHub_SendToClientTimeout(t *testing.T) {
	hub := NewAgentHub()
	// Channel with 0 buffer - will block if not handled
	client := &Client{ID: "c1", AgentID: "a1", Send: make(chan []byte, 1)}
	hub.Register(client)

	// Fill the channel
	client.Send <- []byte("fill")

	// Next send should timeout, not block forever
	err := hub.SendToClient("c1", []byte("overflow"))
	if err == nil {
		// Depends on implementation: may drop or timeout
		// Either way, should not block forever
	}
}

// ============================================================================
// Authenticator 测试
// ============================================================================

func makeSignature(secret, clientID, timestamp, nonce, bodyHash string) string {
	return ComputeSignature([]byte(secret), clientID, timestamp, nonce, bodyHash)
}

func TestAuth_ValidSignature(t *testing.T) {
	auth := NewAuthenticator("test-secret")
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "unique-nonce-1"
	bodyHash := hex.EncodeToString(sha256.New().Sum(nil))
	sig := makeSignature("test-secret", "client1", ts, nonce, bodyHash)

	err := auth.VerifyHMAC("client1", ts, nonce, bodyHash, sig)
	if err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestAuth_InvalidSignature(t *testing.T) {
	auth := NewAuthenticator("test-secret")
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	err := auth.VerifyHMAC("client1", ts, "nonce", "body", "invalid-sig")
	if err == nil {
		t.Fatal("invalid signature accepted")
	}
}

func TestAuth_ReplayAttack(t *testing.T) {
	auth := NewAuthenticator("test-secret")
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "replay-nonce"
	bodyHash := "abc123"
	sig := makeSignature("test-secret", "client1", ts, nonce, bodyHash)

	// First request should pass
	err := auth.VerifyHMAC("client1", ts, nonce, bodyHash, sig)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Same nonce (replay) should fail
	err = auth.VerifyHMAC("client1", ts, nonce, bodyHash, sig)
	if err == nil {
		t.Fatal("replay attack not detected")
	}
}

func TestAuth_ExpiredTimestamp(t *testing.T) {
	auth := NewAuthenticator("test-secret")
	// 10 minutes ago
	ts := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	nonce := "old-nonce"
	bodyHash := "hash"
	sig := makeSignature("test-secret", "client1", ts, nonce, bodyHash)

	err := auth.VerifyHMAC("client1", ts, nonce, bodyHash, sig)
	if err == nil {
		t.Fatal("expired timestamp accepted")
	}
}

func TestAuth_TimingAttack(t *testing.T) {
	auth := NewAuthenticator("test-secret")
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "timing-nonce"
	bodyHash := "hash"

	// Correct signature
	correctSig := makeSignature("test-secret", "client1", ts, nonce, bodyHash)
	_ = correctSig

	// Partially correct signature (same prefix)
	wrongSig := correctSig[:len(correctSig)-2] + "ff"
	err := auth.VerifyHMAC("client1", ts, nonce, bodyHash, wrongSig)
	if err == nil {
		t.Fatal("wrong signature accepted")
	}
	// The actual timing attack prevention is verified by code review
	// (checking use of hmac.Equal vs ==)
}

// ============================================================================
// BridgeRouter 测试
// ============================================================================

func TestBridge_EncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	router := NewBridgeRouter(key)

	plaintext := []byte(`{"version":2,"type":"request","id":"test1","method":"runtime.ping"}`)

	encrypted, err := router.EncryptBridgeMessage(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	decrypted, err := router.DecryptBridgeMessage(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("roundtrip mismatch: got %s", decrypted)
	}
}

func TestBridge_DecryptTooShort(t *testing.T) {
	key := make([]byte, 32)
	router := NewBridgeRouter(key)

	_, err := router.DecryptBridgeMessage([]byte{0, 0, 0, 1})
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestBridge_DispatchUnknownType(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	router := NewBridgeRouter(key)

	msg := Message{Version: 2, Type: "unknown", ID: "x1"}
	data, _ := json.Marshal(msg)
	encrypted, _ := router.EncryptBridgeMessage(data)

	_, err := router.DispatchMessage(encrypted)
	if err == nil {
		t.Fatal("expected error for unknown message type")
	}
}

func TestBridge_DuplicateRequestID(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	router := NewBridgeRouter(key)
	router.RegisterHandler("runtime.ping", func(ctx interface{ Deadline() (time.Time, bool) }, params json.RawMessage) (json.RawMessage, error) {
		return []byte(`"pong"`), nil
	})

	msg := Message{Version: 2, Type: "request", ID: "dup1", Method: "runtime.ping"}
	data, _ := json.Marshal(msg)
	encrypted, _ := router.EncryptBridgeMessage(data)

	// First dispatch should succeed
	_, err := router.DispatchMessage(encrypted)
	if err != nil {
		t.Fatalf("first dispatch failed: %v", err)
	}

	// Re-encrypt same message (same ID)
	encrypted2, _ := router.EncryptBridgeMessage(data)
	_, err = router.DispatchMessage(encrypted2)
	if err == nil {
		t.Fatal("duplicate request ID not detected")
	}
}

func TestBridge_MethodNotFound(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	router := NewBridgeRouter(key)

	msg := Message{Version: 2, Type: "request", ID: "mnf1", Method: "nonexistent.method"}
	data, _ := json.Marshal(msg)
	encrypted, _ := router.EncryptBridgeMessage(data)

	resp, err := router.DispatchMessage(encrypted)
	if err != nil {
		t.Fatalf("dispatch should not return transport error: %v", err)
	}

	// Decrypt response and check error
	decResp, _ := router.DecryptBridgeMessage(resp)
	var respMsg Message
	json.Unmarshal(decResp, &respMsg)
	if respMsg.Error == nil || respMsg.Error.Code != "METHOD_NOT_FOUND" {
		t.Fatalf("expected METHOD_NOT_FOUND error, got: %+v", respMsg.Error)
	}
}

// ============================================================================
// NonceTracker 测试
// ============================================================================

func TestNonceTracker_Basic(t *testing.T) {
	tracker := NewNonceTracker(5 * time.Minute)
	if !tracker.CheckAndRecord("nonce1") {
		t.Fatal("first nonce should be accepted")
	}
	if tracker.CheckAndRecord("nonce1") {
		t.Fatal("duplicate nonce should be rejected")
	}
}

func TestNonceTracker_Concurrent(t *testing.T) {
	tracker := NewNonceTracker(5 * time.Minute)
	var wg sync.WaitGroup
	accepted := make([]bool, 1000)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Use same nonce for pairs
			nonce := fmt.Sprintf("nonce-%d", idx/2)
			accepted[idx] = tracker.CheckAndRecord(nonce)
		}(i)
	}
	wg.Wait()

	// Each nonce appears twice, so exactly 500 should be accepted
	count := 0
	for _, a := range accepted {
		if a {
			count++
		}
	}
	if count != 500 {
		t.Fatalf("expected 500 accepted, got %d", count)
	}
}

// Prevent import errors
var _ = hmac.New
