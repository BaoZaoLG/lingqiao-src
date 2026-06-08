package main

import (
	"crypto/aes"
	"crypto/cipher"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
	cm.sessions["session-1"] = &Session{
		Token:     "session-1",
		CardCode:  "ABCDEF-GHJKLM-NPQRST",
		MachineID: "machine-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	h := NewAPIHandler(cm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dll?token=session-1", nil)
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
