package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// ClientCredentials holds the client ID and secret for HMAC signing.
type ClientCredentials struct {
	ClientID string
	Secret   string
}

// knownClients is the registry of known API clients.
var knownClients []ClientCredentials
var knownClientsMu sync.RWMutex

func configureAuthClients() {
	knownClientsMu.Lock()
	knownClients = nil
	derivedKeyCache = sync.Map{}
	knownClientsMu.Unlock()
}

func ensureKnownClients() {
	knownClientsMu.RLock()
	ready := len(knownClients) > 0
	knownClientsMu.RUnlock()
	if ready {
		return
	}

	knownClientsMu.Lock()
	defer knownClientsMu.Unlock()
	if len(knownClients) > 0 {
		return
	}

	secret := os.Getenv("HMAC_SECRET")
	if secret == "" {
		// Try to load persisted secret, or generate a new one
		if data, err := os.ReadFile(dataPath("hmac_secret.key")); err == nil {
			secret = strings.TrimSpace(string(data))
		}
		if secret == "" {
			b := make([]byte, 32)
			rand.Read(b)
			secret = hex.EncodeToString(b)
			os.MkdirAll(dataDir(), 0755)
			os.WriteFile(dataPath("hmac_secret.key"), []byte(secret), 0600)
			log.Printf("[AUTH] Generated new HMAC secret (saved to %s)", dataPath("hmac_secret.key"))
		}
	}
	knownClients = []ClientCredentials{
		{ClientID: "injector_v1", Secret: secret},
	}
}

// nonceTracker prevents replay attacks by remembering recently used nonces.
type nonceTracker struct {
	mu     sync.Mutex
	nonces map[string]time.Time
}

var usedNonces = &nonceTracker{nonces: make(map[string]time.Time)}

func (nt *nonceTracker) checkAndStore(nonce string, ts time.Time) bool {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	// Cleanup old nonces (older than 2 minutes)
	cutoff := time.Now().Add(-2 * time.Minute)
	for k, t := range nt.nonces {
		if t.Before(cutoff) {
			delete(nt.nonces, k)
		}
	}

	if _, exists := nt.nonces[nonce]; exists {
		return false // nonce already used
	}
	nt.nonces[nonce] = time.Now() // use server time, not client timestamp
	return true
}

// pbkdf2 implements PBKDF2-HMAC-SHA256 using only the standard library.
func pbkdf2(password, salt []byte, iterations, keyLen int) []byte {
	hLen := 32 // SHA-256 output length
	numBlocks := (keyLen + hLen - 1) / hLen
	result := make([]byte, numBlocks*hLen)

	for block := 1; block <= numBlocks; block++ {
		// U1 = HMAC(password, salt || INT_32_BE(block))
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)

		// Initialize block with U1
		offset := (block - 1) * hLen
		copy(result[offset:], u)

		// U2..Un = HMAC(password, U_{i-1}), XOR into result
		for i := 1; i < iterations; i++ {
			mac.Reset()
			mac.Write(u)
			u = mac.Sum(nil)
			for j := 0; j < hLen; j++ {
				result[offset+j] ^= u[j]
			}
		}
	}

	return result[:keyLen]
}

func getClientSecret(clientID string) (string, error) {
	ensureKnownClients()
	knownClientsMu.RLock()
	defer knownClientsMu.RUnlock()
	for _, c := range knownClients {
		if c.ClientID == clientID {
			return c.Secret, nil
		}
	}
	return "", fmt.Errorf("unknown client: %s", clientID)
}

func defaultClientSecret() string {
	secret, _ := getClientSecret("injector_v1")
	return secret
}

// derivedKeyCache caches PBKDF2 output so we don't re-derive on every request.
var derivedKeyCache sync.Map

// getDerivedKey returns the PBKDF2-derived HMAC key for a client (cached).
func getDerivedKey(clientID string) ([]byte, error) {
	if cached, ok := derivedKeyCache.Load(clientID); ok {
		return cached.([]byte), nil
	}
	secret, err := getClientSecret(clientID)
	if err != nil {
		return nil, err
	}
	salt := []byte("CefBridge-HMAC-Salt-v2")
	key := pbkdf2([]byte(secret), salt, 100000, 32)
	derivedKeyCache.Store(clientID, key)
	return key, nil
}

// VerifyHMAC verifies an HMAC-SHA256 signature with timestamp + nonce anti-replay.
// signedData format: "timestamp|nonce|body"
func VerifyHMAC(clientID, body, signature, timestamp, nonce string) error {
	// Validate timestamp (must be within ±30 seconds)
	var ts int64
	if _, err := fmt.Sscanf(timestamp, "%d", &ts); err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	now := time.Now().Unix()
	if diff := now - ts; diff < -30 || diff > 30 {
		return fmt.Errorf("request expired (clock skew: %ds)", diff)
	}

	// Check nonce uniqueness (anti-replay)
	if !usedNonces.checkAndStore(nonce, time.Unix(ts, 0)) {
		return fmt.Errorf("nonce already used (replay detected)")
	}

	// Get PBKDF2-derived key
	derivedKey, err := getDerivedKey(clientID)
	if err != nil {
		return err
	}

	// Verify HMAC over "timestamp|nonce|body"
	signedData := timestamp + "|" + nonce + "|" + body
	mac := hmac.New(sha256.New, derivedKey)
	mac.Write([]byte(signedData))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("invalid HMAC signature")
	}
	return nil
}

// VerifyHMACSimple verifies an HMAC-SHA256 signature without timestamp/nonce (for legacy/GET).
func VerifyHMACSimple(clientID, data, signature string) error {
	derivedKey, err := getDerivedKey(clientID)
	if err != nil {
		return err
	}

	mac := hmac.New(sha256.New, derivedKey)
	mac.Write([]byte(data))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("invalid HMAC signature")
	}
	return nil
}

// SignHMAC signs data with the given client secret.
func SignHMAC(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
