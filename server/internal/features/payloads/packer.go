package payloads

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultChunkSize = 1 << 20 // 1MB
	maxPayloadIDLen  = 64
)

// validatePayloadID checks that the payload ID is safe to use as a directory/file name.
func validatePayloadID(id string) error {
	if id == "" {
		return fmt.Errorf("payload_id cannot be empty")
	}
	if len(id) > maxPayloadIDLen {
		return fmt.Errorf("payload_id exceeds %d characters", maxPayloadIDLen)
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("payload_id cannot contain ..")
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		return fmt.Errorf("payload_id contains invalid character: %q", c)
	}
	return nil
}

// generatePayloadID creates a new payload ID with a timestamp prefix and random suffix.
func generatePayloadID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a timestamp-based ID if random fails.
		return fmt.Sprintf("payload-%d", time.Now().Unix())
	}
	return fmt.Sprintf("payload-%s-%s", time.Now().Format("20060102-150405"), hex.EncodeToString(b))
}

// PackPlainDLL encrypts a raw DLL with AES-256-CTR and splits the ciphertext into chunks.
func PackPlainDLL(payloadID string, dll []byte, chunkSize int) (*PayloadInfo, [][]byte, error) {
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}
	if len(dll) < 2 || dll[0] != 'M' || dll[1] != 'Z' {
		return nil, nil, fmt.Errorf("invalid DLL payload")
	}

	aesKey := make([]byte, 32)
	hmacKey := make([]byte, 32)
	iv := make([]byte, 16)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, nil, fmt.Errorf("failed to generate aes key: %w", err)
	}
	if _, err := rand.Read(hmacKey); err != nil {
		return nil, nil, fmt.Errorf("failed to generate hmac key: %w", err)
	}
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, fmt.Errorf("failed to generate iv: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	stream := cipher.NewCTR(block, iv)
	encrypted := make([]byte, len(dll))
	stream.XORKeyStream(encrypted, dll)

	chunks := make([][]byte, 0, (len(encrypted)+chunkSize-1)/chunkSize)
	for i := 0; i < len(encrypted); i += chunkSize {
		end := i + chunkSize
		if end > len(encrypted) {
			end = len(encrypted)
		}
		chunk := make([]byte, end-i)
		copy(chunk, encrypted[i:end])
		chunks = append(chunks, chunk)
	}

	dllHash := sha256.Sum256(dll)
	info := &PayloadInfo{
		PayloadID:  payloadID,
		AesKey:     hex.EncodeToString(aesKey),
		HmacKey:    hex.EncodeToString(hmacKey),
		IV:         hex.EncodeToString(iv),
		ExeHash:    hex.EncodeToString(dllHash[:]),
		ChunkCount: len(chunks),
		ChunkSize:  chunkSize,
		TotalSize:  len(encrypted),
		CreatedAt:  time.Now(),
	}
	return info, chunks, nil
}

// savePayloadChunks writes encrypted chunks to a temp directory and atomically renames it.
func savePayloadChunks(dataDir string, info *PayloadInfo, chunks [][]byte) error {
	if err := validatePayloadID(info.PayloadID); err != nil {
		return err
	}

	payloadsDir := filepath.Join(dataDir, "payloads")
	tmpDir := filepath.Join(payloadsDir, info.PayloadID+".tmp")
	finalDir := filepath.Join(payloadsDir, info.PayloadID)

	// Clean up any stale temp directory from a previous failed attempt.
	_ = os.RemoveAll(tmpDir)

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	for i, chunk := range chunks {
		chunkPath := filepath.Join(tmpDir, fmt.Sprintf("chunk_%04d.bin", i))
		if err := os.WriteFile(chunkPath, chunk, 0600); err != nil {
			return fmt.Errorf("failed to save chunk %d: %w", i, err)
		}
	}

	if err := os.RemoveAll(finalDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove old payload directory: %w", err)
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return fmt.Errorf("failed to finalize payload directory: %w", err)
	}

	success = true
	return nil
}

// isPEPayload checks whether the data looks like a Windows PE file (MZ header).
func isPEPayload(data []byte) bool {
	return len(data) >= 2 && data[0] == 'M' && data[1] == 'Z'
}

// chunkFilePath returns the expected filesystem path for a single chunk.
func chunkFilePath(dataDir, payloadID string, idx int) string {
	return filepath.Join(dataDir, "payloads", payloadID, fmt.Sprintf("chunk_%04d.bin", idx))
}

// payloadDirPath returns the directory where a payload's chunks are stored.
func payloadDirPath(dataDir, payloadID string) string {
	return filepath.Join(dataDir, "payloads", payloadID)
}

// escapeFilenameForAudit removes newlines from a filename for safe audit logging.
func escapeFilenameForAudit(name string) string {
	return strconv.QuoteToASCII(name)
}
