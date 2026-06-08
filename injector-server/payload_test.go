package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func validPayloadUpload(chunks []string) map[string]any {
	return map[string]any{
		"payload_id":  "payload-1",
		"aes_key":     strings.Repeat("a", 64),
		"hmac_key":    strings.Repeat("b", 64),
		"iv":          strings.Repeat("c", 32),
		"exe_hash":    strings.Repeat("d", 64),
		"chunk_count": len(chunks),
		"chunk_size":  3,
		"total_size":  3 * len(chunks),
		"chunks":      chunks,
	}
}

func performPayloadUpload(t *testing.T, handler *PayloadHandler, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/payload/upload", bytes.NewReader(body))
	req.Header.Set("X-Upload-Key", "upload-key")
	rr := httptest.NewRecorder()
	handler.HandleUpload(rr, req)
	return rr
}

func TestHandleUploadRejectsMismatchedChunkMetadata(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	store := NewPayloadStore(NewJSONStorage(dataDir()))
	handler := &PayloadHandler{store: store, uploadKey: "upload-key"}
	payload := validPayloadUpload([]string{base64.StdEncoding.EncodeToString([]byte("abc"))})
	payload["chunk_count"] = 2

	rr := performPayloadUpload(t, handler, payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	if got := store.Get("payload-1"); got != nil {
		t.Fatalf("payload was persisted despite invalid metadata: %#v", got)
	}
}

func TestHandleUploadRejectsInvalidKeyMaterial(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	store := NewPayloadStore(NewJSONStorage(dataDir()))
	handler := &PayloadHandler{store: store, uploadKey: "upload-key"}
	payload := validPayloadUpload([]string{base64.StdEncoding.EncodeToString([]byte("abc"))})
	payload["aes_key"] = "not-hex"

	rr := performPayloadUpload(t, handler, payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	if got := store.Get("payload-1"); got != nil {
		t.Fatalf("payload was persisted despite invalid key material: %#v", got)
	}
}

func TestPayloadAdminListRedactsSecretsAndRecordsAudit(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	cm := NewCardManager(NewJSONStorage(dataDir()))
	store := NewPayloadStore(NewJSONStorage(dataDir()))
	handler := &PayloadHandler{store: store, uploadKey: "upload-key"}
	handler.SetAuditSink(cm.RecordAudit)
	payload := validPayloadUpload([]string{base64.StdEncoding.EncodeToString([]byte("abc"))})

	uploadRR := performPayloadUpload(t, handler, payload)
	if uploadRR.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200; body=%s", uploadRR.Code, uploadRR.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/payloads", nil)
	rr := httptest.NewRecorder()
	handler.HandleAdminList(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, forbidden := range []string{"aes_key", "hmac_key", `"iv"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("payload admin list leaked %s: %s", forbidden, body)
		}
	}
	var resp struct {
		Payloads []PayloadSummary `json:"payloads"`
		Total    int              `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Payloads) != 1 || resp.Payloads[0].PayloadID != "payload-1" {
		t.Fatalf("unexpected payload list: %#v", resp)
	}
	foundAudit := false
	for _, entry := range cm.AuditLog() {
		if entry.Action == "payload_uploaded" && strings.Contains(entry.Detail, "payload-1") {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("payload upload audit not recorded: %#v", cm.AuditLog())
	}
}

func TestPayloadStoreCanActivateAndRollbackPayloads(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	store := NewPayloadStore(NewJSONStorage(dataDir()))
	first := &PayloadInfo{PayloadID: "payload-a", AesKey: strings.Repeat("a", 64), HmacKey: strings.Repeat("b", 64), IV: strings.Repeat("c", 32), ExeHash: strings.Repeat("d", 64), ChunkCount: 1, ChunkSize: 3, TotalSize: 3, CreatedAt: time.Now()}
	second := &PayloadInfo{PayloadID: "payload-b", AesKey: strings.Repeat("a", 64), HmacKey: strings.Repeat("b", 64), IV: strings.Repeat("c", 32), ExeHash: strings.Repeat("e", 64), ChunkCount: 1, ChunkSize: 3, TotalSize: 3, CreatedAt: time.Now().Add(time.Second)}
	store.Add(first)
	store.Add(second)

	if err := store.SetActive("payload-b"); err != nil {
		t.Fatal(err)
	}
	active := store.Active()
	if active == nil || active.PayloadID != "payload-b" {
		t.Fatalf("active payload = %#v, want payload-b", active)
	}
	if err := store.SetActive("payload-a"); err != nil {
		t.Fatal(err)
	}
	active = store.Active()
	if active == nil || active.PayloadID != "payload-a" {
		t.Fatalf("active payload after rollback = %#v, want payload-a", active)
	}
	if err := store.Delete("payload-a"); err == nil {
		t.Fatalf("Delete(active payload) succeeded, want error")
	}
	if err := store.Delete("payload-b"); err != nil {
		t.Fatalf("Delete(non-active payload) returned error: %v", err)
	}
	if got := store.Get("payload-b"); got != nil {
		t.Fatalf("deleted payload still returned: %#v", got)
	}
}

func TestPayloadAdminManageActivateAndDelete(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	cm := NewCardManager(NewJSONStorage(dataDir()))
	store := NewPayloadStore(NewJSONStorage(dataDir()))
	handler := &PayloadHandler{store: store, uploadKey: "upload-key"}
	handler.SetAuditSink(cm.RecordAudit)
	store.Add(&PayloadInfo{PayloadID: "payload-a", AesKey: strings.Repeat("a", 64), HmacKey: strings.Repeat("b", 64), IV: strings.Repeat("c", 32), ExeHash: strings.Repeat("d", 64), ChunkCount: 1, ChunkSize: 3, TotalSize: 3, CreatedAt: time.Now()})

	req := httptest.NewRequest(http.MethodPost, "/admin/api/payloads/manage", bytes.NewBufferString(`{"action":"activate","payload_id":"payload-a"}`))
	rr := httptest.NewRecorder()
	handler.HandleAdminManage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("activate status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if active := store.Active(); active == nil || active.PayloadID != "payload-a" {
		t.Fatalf("active payload = %#v, want payload-a", active)
	}

	foundAudit := false
	for _, entry := range cm.AuditLog() {
		if entry.Action == "payload_activated" {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("payload activation audit not recorded: %#v", cm.AuditLog())
	}
}

func TestHandleKeyExchangeRejectsStoredInvalidKeyMaterial(t *testing.T) {
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: t.TempDir(), SessionTTL: 4 * time.Hour})
	defer restore()
	cm := NewCardManager(NewJSONStorage(dataDir()))
	cm.mu.Lock()
	cm.sessions["session-1"] = &Session{
		Token:     "session-1",
		CardCode:  "ABCDEF-GHJKMN-PQRSTV",
		MachineID: "machine-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	cm.mu.Unlock()
	store := NewPayloadStore(NewJSONStorage(dataDir()))
	store.Add(&PayloadInfo{
		PayloadID: "bad-payload",
		AesKey:    "zz",
		HmacKey:   strings.Repeat("b", 64),
		IV:        strings.Repeat("c", 32),
	})
	handler := &PayloadHandler{store: store, uploadKey: "upload-key"}
	body := []byte(`{"session_token":"session-1","machine_id":"machine-1","payload_id":"bad-payload"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payload/key", bytes.NewReader(body))
	signPayloadRequest(t, req, body)
	rr := httptest.NewRecorder()

	handler.HandleKeyExchange(rr, req, cm)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rr.Code, rr.Body.String())
	}
}

func signPayloadRequest(t *testing.T, req *http.Request, body []byte) {
	t.Helper()
	derivedKey, err := getDerivedKey("injector_v1")
	if err != nil {
		t.Fatal(err)
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("payload-%d", time.Now().UnixNano())
	signedData := timestamp + "|" + nonce + "|" + string(body)
	req.Header.Set("X-Client-ID", "injector_v1")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-HMAC-Signature", SignHMAC(string(derivedKey), signedData))
}
