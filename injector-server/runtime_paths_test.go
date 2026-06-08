package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeDataDirAppliesToAdminPasswordAndPayloadFiles(t *testing.T) {
	workDir := t.TempDir()
	configuredDataDir := filepath.Join(t.TempDir(), "configured-data")
	t.Chdir(workDir)
	restore := configureRuntimeForTest(t, RuntimeConfig{
		DataDir:    configuredDataDir,
		SessionTTL: 4 * time.Hour,
	})
	defer restore()
	t.Setenv("ADMIN_PASSWORD", "admin-password")

	cm := NewCardManager(NewJSONStorage(configuredDataDir))
	NewAdminHandler(cm)

	if _, err := os.Stat(filepath.Join(configuredDataDir, "admin_password.hash")); err != nil {
		t.Fatalf("admin password hash was not written to configured DATA_DIR: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "data", "admin_password.hash")); !os.IsNotExist(err) {
		t.Fatalf("admin password hash leaked into cwd data directory: %v", err)
	}

	store := NewPayloadStore(NewJSONStorage(configuredDataDir))
	handler := &PayloadHandler{store: store, uploadKey: "upload-key"}
	body, err := json.Marshal(map[string]any{
		"payload_id":  "payload-1",
		"aes_key":     strings.Repeat("a", 64),
		"hmac_key":    strings.Repeat("b", 64),
		"iv":          strings.Repeat("c", 32),
		"exe_hash":    strings.Repeat("d", 64),
		"chunk_count": 1,
		"chunk_size":  3,
		"total_size":  3,
		"chunks":      []string{base64.StdEncoding.EncodeToString([]byte("abc"))},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/payload/upload", bytes.NewReader(body))
	req.Header.Set("X-Upload-Key", "upload-key")
	rr := httptest.NewRecorder()

	handler.HandleUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(configuredDataDir, "payloads", "payload-1", "chunk_0000.bin")); err != nil {
		t.Fatalf("payload chunk was not written to configured DATA_DIR: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "data", "payloads", "payload-1", "chunk_0000.bin")); !os.IsNotExist(err) {
		t.Fatalf("payload chunk leaked into cwd data directory: %v", err)
	}
}

func TestRuntimeDataDirAppliesToGeneratedHMACSecret(t *testing.T) {
	workDir := t.TempDir()
	configuredDataDir := filepath.Join(t.TempDir(), "configured-data")
	t.Chdir(workDir)
	t.Setenv("HMAC_SECRET", "")
	restore := configureRuntimeForTest(t, RuntimeConfig{
		DataDir:    configuredDataDir,
		SessionTTL: 4 * time.Hour,
	})
	defer restore()

	if _, err := getClientSecret("injector_v1"); err != nil {
		t.Fatalf("getClientSecret returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(configuredDataDir, "hmac_secret.key")); err != nil {
		t.Fatalf("HMAC secret was not written to configured DATA_DIR: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "data", "hmac_secret.key")); !os.IsNotExist(err) {
		t.Fatalf("HMAC secret leaked into cwd data directory: %v", err)
	}
}
