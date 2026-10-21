package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lingqiao/server/internal/features/payloads"
)

type testActivePayloadStore struct {
	active *payloads.PayloadInfo
}

func (s testActivePayloadStore) Active() *payloads.PayloadInfo {
	return s.active
}

func testDLLBytes() []byte {
	data := make([]byte, 1024)
	data[0] = 'M'
	data[1] = 'Z'
	for i := 2; i < len(data); i++ {
		data[i] = byte(i % 251)
	}
	return data
}

func TestLoadDLLSourceUsesActivePayloadBeforeLegacyFile(t *testing.T) {
	dataDir := t.TempDir()
	activeDLL := testDLLBytes()
	legacyDLL := append([]byte("legacy-"), testDLLBytes()...)

	if err := os.WriteFile(filepath.Join(dataDir, "CefHook.dll"), legacyDLL, 0600); err != nil {
		t.Fatalf("write legacy dll: %v", err)
	}

	info, chunks, err := payloads.PackPlainDLL("active-dll", activeDLL, 128)
	if err != nil {
		t.Fatalf("pack active dll: %v", err)
	}
	writePayloadChunksForTest(t, dataDir, info.PayloadID, chunks)

	h := &APIHandler{payloadStore: testActivePayloadStore{active: info}, dataDir: dataDir}
	source, err := h.loadDLLSource(context.Background())
	if err != nil {
		t.Fatalf("loadDLLSource returned error: %v", err)
	}

	if source.Kind != "active-payload" {
		t.Fatalf("source kind = %q, want active-payload", source.Kind)
	}
	if source.PayloadID != info.PayloadID {
		t.Fatalf("payload id = %q, want %q", source.PayloadID, info.PayloadID)
	}
	if !bytes.Equal(source.Bytes, activeDLL) {
		t.Fatalf("loaded DLL bytes did not match active payload")
	}
}

func TestLoadDLLSourceFallsBackToLegacyFile(t *testing.T) {
	dataDir := t.TempDir()
	legacyDLL := testDLLBytes()
	if err := os.WriteFile(filepath.Join(dataDir, "CefHook.dll"), legacyDLL, 0600); err != nil {
		t.Fatalf("write legacy dll: %v", err)
	}

	h := &APIHandler{payloadStore: testActivePayloadStore{}, dataDir: dataDir}
	source, err := h.loadDLLSource(context.Background())
	if err != nil {
		t.Fatalf("loadDLLSource returned error: %v", err)
	}

	if source.Kind != "legacy-file" {
		t.Fatalf("source kind = %q, want legacy-file", source.Kind)
	}
	if !bytes.Equal(source.Bytes, legacyDLL) {
		t.Fatalf("loaded DLL bytes did not match legacy file")
	}
}

func writePayloadChunksForTest(t *testing.T, dataDir, payloadID string, chunks [][]byte) {
	t.Helper()
	dir := filepath.Join(dataDir, "payloads", payloadID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create payload dir: %v", err)
	}
	for i, chunk := range chunks {
		path := filepath.Join(dir, "chunk_"+formatChunkIndexForTest(i)+".bin")
		if err := os.WriteFile(path, chunk, 0600); err != nil {
			t.Fatalf("write chunk %d: %v", i, err)
		}
	}
}

func formatChunkIndexForTest(i int) string {
	return []string{
		"0000", "0001", "0002", "0003", "0004", "0005", "0006", "0007",
		"0008", "0009", "0010", "0011", "0012", "0013", "0014", "0015",
	}[i]
}
