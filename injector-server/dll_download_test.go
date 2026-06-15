package main

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func addTestHMACHeaders(req *http.Request, signedBody string) {
	derivedKey, _ := getDerivedKey("injector_v1")
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("test-%d", time.Now().UnixNano())
	signedData := timestamp + "|" + nonce + "|" + signedBody
	req.Header.Set("X-Client-ID", "injector_v1")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-HMAC-Signature", SignHMAC(string(derivedKey), signedData))
}

func TestHandleDllDownloadReturnsClientDecryptableAesGCM(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0755); err != nil {
		t.Fatal(err)
	}
	plain := minimalPEDLL()
	if err := os.WriteFile(filepath.Join("data", "CefHook.dll"), plain, 0600); err != nil {
		t.Fatal(err)
	}

	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "data")))
	addBoundTestSession(cm, "session-1", "ABCDEF-GHJKLM-NPQRST", "machine-1")
	h := NewAPIHandler(cm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dll", nil)
	req.Header.Set("X-Session-Token", "session-1")
	req.Header.Set("X-Machine-ID", "machine-1")
	req.Header.Set("X-Card-Code", "ABCDEF-GHJKLM-NPQRST")
	// Add required HMAC signature headers
	addTestHMACHeaders(req, "/api/v1/dll")
	rr := httptest.NewRecorder()
	h.HandleDllDownload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	decrypted := decryptClientDLLResponse(t, rr.Body.Bytes())
	if string(decrypted[:2]) != "MZ" {
		t.Fatalf("decrypted DLL missing MZ header: %x", decrypted[:2])
	}
	peOffset := int(decrypted[0x3c])
	if string(decrypted[peOffset:peOffset+4]) != "PE\x00\x00" {
		t.Fatalf("decrypted DLL missing PE header at %d", peOffset)
	}
}

func TestHandleDllDownloadAcceptsLegacyClientWithoutCardHeader(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "CefHook.dll"), minimalPEDLL(), 0600); err != nil {
		t.Fatal(err)
	}

	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "data")))
	addBoundTestSession(cm, "session-legacy", "ABCDEF-GHJKLM-NPQRST", "machine-legacy")
	h := NewAPIHandler(cm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dll", nil)
	req.Header.Set("X-Session-Token", "session-legacy")
	req.Header.Set("X-Machine-ID", "machine-legacy")
	addTestHMACHeaders(req, "/api/v1/dll")
	rr := httptest.NewRecorder()
	h.HandleDllDownload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("legacy client status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	decrypted := decryptClientDLLResponse(t, rr.Body.Bytes())
	if string(decrypted[:2]) != "MZ" {
		t.Fatalf("decrypted DLL missing MZ header: %x", decrypted[:2])
	}
}

func TestHandleDllDownloadRejectsQueryTokenAndMachineMismatch(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "CefHook.dll"), minimalPEDLL(), 0600); err != nil {
		t.Fatal(err)
	}

	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "data")))
	addBoundTestSession(cm, "session-1", "ABCDEF-GHJKLM-NPQRST", "machine-1")
	h := NewAPIHandler(cm)

	queryTokenReq := httptest.NewRequest(http.MethodGet, "/api/v1/dll?token=session-1&machine_id=machine-1&card=ABCDEF-GHJKLM-NPQRST", nil)
	addTestHMACHeaders(queryTokenReq, "/api/v1/dll")
	queryTokenRR := httptest.NewRecorder()
	h.HandleDllDownload(queryTokenRR, queryTokenReq)
	if queryTokenRR.Code != http.StatusBadRequest {
		t.Fatalf("query token status = %d, want 400", queryTokenRR.Code)
	}

	mismatchReq := httptest.NewRequest(http.MethodGet, "/api/v1/dll?machine_id=machine-2&card=ABCDEF-GHJKLM-NPQRST", nil)
	mismatchReq.Header.Set("X-Session-Token", "session-1")
	mismatchReq.Header.Set("X-Machine-ID", "machine-2")
	mismatchReq.Header.Set("X-Card-Code", "ABCDEF-GHJKLM-NPQRST")
	addTestHMACHeaders(mismatchReq, "/api/v1/dll")
	mismatchRR := httptest.NewRecorder()
	h.HandleDllDownload(mismatchRR, mismatchReq)
	if mismatchRR.Code != http.StatusUnauthorized {
		t.Fatalf("machine mismatch status = %d, want 401", mismatchRR.Code)
	}
}

func addBoundTestSession(cm *CardManager, token, cardCode, machineID string) {
	now := time.Now()
	normalized := normalizeCardCode(cardCode)
	cm.cards[normalized] = &Card{
		Code:        cardCode,
		MachineID:   machineID,
		CreatedAt:   now.Add(-time.Hour),
		ActivatedAt: &now,
		ExpiresAt:   now.Add(time.Hour),
		Status:      CardActive,
		MaxSessions: 1,
	}
	cm.sessions[token] = &Session{
		Token:     token,
		CardCode:  normalized,
		MachineID: machineID,
		CreatedAt: now,
		LastSeen:  now,
		ExpiresAt: now.Add(time.Hour),
	}
}

func decryptClientDLLResponse(t *testing.T, body []byte) []byte {
	t.Helper()
	if len(body) < 12+16 {
		t.Fatalf("encrypted body too short: %d", len(body))
	}
	key := pbkdf2([]byte(defaultClientSecret()), []byte("CefBridge-DLL-Salt-v1"), 100000, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := gcm.Open(nil, body[:12], body[12:], nil)
	if err != nil {
		t.Fatalf("client-compatible AES-GCM decrypt failed: %v", err)
	}
	return plain
}

func minimalPEDLL() []byte {
	data := make([]byte, 4096)
	data[0] = 'M'
	data[1] = 'Z'
	data[0x3c] = 0x80
	copy(data[0x80:], []byte{'P', 'E', 0, 0})
	data[0x80+24] = 0x0b
	data[0x80+25] = 0x02
	return data
}
