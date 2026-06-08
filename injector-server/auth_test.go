package main

import (
	"fmt"
	"testing"
	"time"
)

func TestPBKDF2(t *testing.T) {
	// Verify PBKDF2 produces consistent output
	secret := "test-only-pbkdf2-secret"
	salt := []byte("CefBridge-HMAC-Salt-v2")
	key1 := pbkdf2([]byte(secret), salt, 100000, 32)
	key2 := pbkdf2([]byte(secret), salt, 100000, 32)

	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key1))
	}
	for i := range key1 {
		if key1[i] != key2[i] {
			t.Fatal("same input should produce same derived key")
		}
	}
}

func TestVerifyHMAC(t *testing.T) {
	// Sign using the same derived key the server will use
	derivedKey, err := getDerivedKey("injector_v1")
	if err != nil {
		t.Fatalf("getDerivedKey failed: %v", err)
	}

	body := `{"test":"data"}`
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "abcdef1234567890abcdef1234567890"
	signedData := ts + "|" + nonce + "|" + body
	sig := SignHMAC(string(derivedKey), signedData)

	err = VerifyHMAC("injector_v1", body, sig, ts, nonce)
	if err != nil {
		t.Fatalf("VerifyHMAC should succeed: %v", err)
	}
}

func TestVerifyHMACInvalidSignature(t *testing.T) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "11111111111111111111111111111111"
	err := VerifyHMAC("injector_v1", `{"test":"data"}`, "invalid_signature", ts, nonce)
	if err == nil {
		t.Fatal("VerifyHMAC should fail with invalid signature")
	}
}

func TestVerifyHMACUnknownClient(t *testing.T) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "22222222222222222222222222222222"
	err := VerifyHMAC("unknown_client", `{"test":"data"}`, "abc123", ts, nonce)
	if err == nil {
		t.Fatal("VerifyHMAC should fail with unknown client")
	}
}

func TestVerifyHMACExpiredTimestamp(t *testing.T) {
	derivedKey, _ := getDerivedKey("injector_v1")
	body := `{"test":"data"}`
	ts := fmt.Sprintf("%d", time.Now().Unix()-60)
	nonce := "33333333333333333333333333333333"
	signedData := ts + "|" + nonce + "|" + body
	sig := SignHMAC(string(derivedKey), signedData)

	err := VerifyHMAC("injector_v1", body, sig, ts, nonce)
	if err == nil {
		t.Fatal("VerifyHMAC should fail with expired timestamp")
	}
}

func TestVerifyHMACReplayDetection(t *testing.T) {
	derivedKey, _ := getDerivedKey("injector_v1")
	body := `{"test":"data"}`
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "44444444444444444444444444444444"
	signedData := ts + "|" + nonce + "|" + body
	sig := SignHMAC(string(derivedKey), signedData)

	// First request should succeed
	err := VerifyHMAC("injector_v1", body, sig, ts, nonce)
	if err != nil {
		t.Fatalf("First VerifyHMAC should succeed: %v", err)
	}

	// Replay with same nonce should fail
	err = VerifyHMAC("injector_v1", body, sig, ts, nonce)
	if err == nil {
		t.Fatal("Replayed request should fail")
	}
}

func TestSignHMAC(t *testing.T) {
	sig1 := SignHMAC("key1", "data1")
	sig2 := SignHMAC("key1", "data1")
	if sig1 != sig2 {
		t.Fatal("same input should produce same signature")
	}

	sig3 := SignHMAC("key1", "data2")
	if sig1 == sig3 {
		t.Fatal("different input should produce different signature")
	}
}
