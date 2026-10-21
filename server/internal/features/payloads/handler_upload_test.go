package payloads

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lingqiao/server/internal/storage"
)

func newTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	dataDir := t.TempDir()
	store := NewStore(storage.NewJSONStore(dataDir))
	h, err := NewHandler(store, nil, dataDir)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	return h, dataDir
}

func minimalDLL() []byte {
	data := make([]byte, 1024)
	data[0] = 'M'
	data[1] = 'Z'
	for i := 2; i < len(data); i++ {
		data[i] = byte(i % 256)
	}
	return data
}

func buildMultipartRequest(t *testing.T, filename string, data []byte, fields map[string]string) *http.Request {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("failed to write form file: %v", err)
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("failed to write field: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/payloads/upload-dll", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestHandleAdminDLLUploadCreatesPayload(t *testing.T) {
	h, dataDir := newTestHandler(t)
	dll := minimalDLL()
	req := buildMultipartRequest(t, "test.dll", dll, nil)
	rr := httptest.NewRecorder()

	h.HandleAdminDLLUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	payloads := h.store.List()
	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	info := payloads[0]
	hash := sha256.Sum256(dll)
	if info.ExeHash != hex.EncodeToString(hash[:]) {
		t.Fatalf("exe_hash mismatch")
	}
	if info.TotalSize != len(dll) {
		t.Fatalf("total_size = %d, want %d", info.TotalSize, len(dll))
	}

	chunkDir := filepath.Join(dataDir, "payloads", info.PayloadID)
	files, err := os.ReadDir(chunkDir)
	if err != nil {
		t.Fatalf("failed to read chunk dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no chunk files written")
	}
}

func TestHandleAdminDLLUploadActivatesWhenRequested(t *testing.T) {
	h, _ := newTestHandler(t)
	dll := minimalDLL()
	req := buildMultipartRequest(t, "test.dll", dll, map[string]string{"activate": "true"})
	rr := httptest.NewRecorder()

	h.HandleAdminDLLUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	active := h.store.Active()
	if active == nil {
		t.Fatalf("expected active payload")
	}
	payloads := h.store.List()
	if payloads[0].PayloadID != active.PayloadID {
		t.Fatalf("active payload mismatch")
	}
}

func TestHandleAdminDLLUploadDoesNotActivateWhenFalse(t *testing.T) {
	h, _ := newTestHandler(t)
	dll := minimalDLL()
	req := buildMultipartRequest(t, "test.dll", dll, map[string]string{"activate": "false"})
	rr := httptest.NewRecorder()

	h.HandleAdminDLLUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if h.store.Active() != nil {
		t.Fatalf("expected no active payload")
	}
}

func TestHandleAdminDLLUploadRejectsInvalidExtension(t *testing.T) {
	h, _ := newTestHandler(t)
	dll := minimalDLL()
	req := buildMultipartRequest(t, "test.txt", dll, nil)
	rr := httptest.NewRecorder()

	h.HandleAdminDLLUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), "only .dll payloads are supported") {
		t.Fatalf("unexpected error: %s", rr.Body.String())
	}
}

func TestHandleAdminDLLUploadRejectsInvalidPE(t *testing.T) {
	h, _ := newTestHandler(t)
	req := buildMultipartRequest(t, "test.dll", []byte("not a dll"), nil)
	rr := httptest.NewRecorder()

	h.HandleAdminDLLUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), "invalid DLL payload") {
		t.Fatalf("unexpected error: %s", rr.Body.String())
	}
}

func TestLegacyUploadStillRequiresUploadKey(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/payload/upload", nil)
	rr := httptest.NewRecorder()

	h.HandleUpload(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandleKeyExchangeRejectsNonPOST(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/key-exchange", nil)
	rr := httptest.NewRecorder()

	h.HandleKeyExchange(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusMethodNotAllowed, rr.Body.String())
	}
}

func TestHandlePayloadInfoRejectsNonGET(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payload/payload-1/info", nil)
	rr := httptest.NewRecorder()

	h.HandlePayloadInfo(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusMethodNotAllowed, rr.Body.String())
	}
}

func TestHandleChunkDownloadRejectsNonGET(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payload/payload-1/chunk/0", nil)
	rr := httptest.NewRecorder()

	h.HandleChunkDownload(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusMethodNotAllowed, rr.Body.String())
	}
}

func TestHandleAdminDLLUploadRejectsDuplicatePayloadID(t *testing.T) {
	h, _ := newTestHandler(t)
	dll := minimalDLL()
	req := buildMultipartRequest(t, "test.dll", dll, map[string]string{"payload_id": "custom-id"})
	rr := httptest.NewRecorder()
	h.HandleAdminDLLUpload(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first upload failed: %s", rr.Body.String())
	}

	req2 := buildMultipartRequest(t, "test.dll", dll, map[string]string{"payload_id": "custom-id"})
	rr2 := httptest.NewRecorder()
	h.HandleAdminDLLUpload(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr2.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr2.Body.String(), "already exists") {
		t.Fatalf("unexpected error: %s", rr2.Body.String())
	}
}

func TestPackPlainDLLRejectsNonPE(t *testing.T) {
	_, _, err := PackPlainDLL("test", []byte("not a dll"), 1024)
	if err == nil || !strings.Contains(err.Error(), "invalid DLL payload") {
		t.Fatalf("expected invalid DLL payload error, got %v", err)
	}
}

func TestValidatePayloadID(t *testing.T) {
	cases := map[string]bool{
		"valid-id":     true,
		"valid_id":     true,
		"valid.id":     true,
		"payload-123":  true,
		"":             false,
		"has/slash":    false,
		"has\\\\back":   false,
		"has..dot":     false,
		"has space":    false,
		"a":            true,
		strings.Repeat("a", 65): false,
	}
	for id, wantOk := range cases {
		err := validatePayloadID(id)
		if wantOk && err != nil {
			t.Fatalf("validatePayloadID(%q) error: %v", id, err)
		}
		if !wantOk && err == nil {
			t.Fatalf("validatePayloadID(%q) expected error", id)
		}
	}
}

func TestHandleUploadUsesNewValidationAndSave(t *testing.T) {
	h, dataDir := newTestHandler(t)
	payloadID := "legacy-test"
	aesKey := hex.EncodeToString(make([]byte, 32))
	hmacKey := hex.EncodeToString(make([]byte, 32))
	iv := hex.EncodeToString(make([]byte, 16))
	exeHash := hex.EncodeToString(make([]byte, 32))
	chunkData := []byte("encrypted chunk data")

	body := fmt.Sprintf(`{
		"payload_id": "%s",
		"aes_key": "%s",
		"hmac_key": "%s",
		"iv": "%s",
		"exe_hash": "%s",
		"chunk_count": 1,
		"chunk_size": %d,
		"total_size": %d,
		"chunks": ["%s"]
	}`, payloadID, aesKey, hmacKey, iv, exeHash, len(chunkData), len(chunkData), base64.StdEncoding.EncodeToString(chunkData))

	req := httptest.NewRequest(http.MethodPost, "/admin/api/payload/upload", nil)
	req.Header.Set("X-Upload-Key", h.uploadKey)
	req.Body = io.NopCloser(bytes.NewReader([]byte(body)))
	rr := httptest.NewRecorder()

	h.HandleUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	chunkPath := filepath.Join(dataDir, "payloads", payloadID, "chunk_0000.bin")
	if _, err := os.Stat(chunkPath); err != nil {
		t.Fatalf("chunk file not found: %v", err)
	}
}
