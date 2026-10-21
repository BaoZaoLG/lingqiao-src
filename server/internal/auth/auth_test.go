package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionStoreLifecycle(t *testing.T) {
	store := NewSessionStore("admin", time.Minute)
	token, err := store.Create("user-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if store.RawTokenStoredForTest(token) {
		t.Fatal("raw token must not be stored as map key")
	}
	actor, ok := store.Check(token)
	if !ok || actor != "user-1" {
		t.Fatalf("Check = (%q, %v), want user-1 true", actor, ok)
	}
	store.Invalidate(token)
	if _, ok := store.Check(token); ok {
		t.Fatal("expected invalidated token to fail Check")
	}
}

func TestSessionStoreCleanupAndDefaultTTL(t *testing.T) {
	store := NewSessionStore("", 0)
	token, err := store.Create("a")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "sess_") {
		t.Fatalf("token prefix = %q, want sess_", token)
	}
	removed := store.Cleanup(time.Now().Add(5 * time.Hour))
	if removed != 1 {
		t.Fatalf("Cleanup removed = %d, want 1", removed)
	}
	if _, ok := store.Check(token); ok {
		t.Fatal("expired session should not check ok")
	}
}

func TestHashPasswordAndVerify(t *testing.T) {
	hash := HashPassword("secret-pass")
	if !IsBcryptHash(hash) {
		t.Fatalf("expected bcrypt hash, got %q", hash)
	}
	ok, newHash := VerifyPassword("secret-pass", hash)
	if !ok {
		t.Fatal("bcrypt verify failed")
	}
	if newHash != "" {
		t.Fatalf("unexpected rehash for bcrypt: %q", newHash)
	}
	ok, _ = VerifyPassword("wrong", hash)
	if ok {
		t.Fatal("wrong password should not match bcrypt hash")
	}
}

func TestVerifyPasswordLegacyUpgrade(t *testing.T) {
	// legacy password storage is raw sha256 hex of the password bytes
	legacy := HashToken("old-pass")
	ok, newHash := VerifyPassword("old-pass", legacy)
	if !ok {
		t.Fatal("legacy password should match")
	}
	if newHash == "" || !IsBcryptHash(newHash) {
		t.Fatalf("expected bcrypt upgrade hash, got %q", newHash)
	}
	ok, _ = VerifyPassword("wrong", legacy)
	if ok {
		t.Fatal("wrong password should not match")
	}
}

func TestExtractToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "admin_token", Value: "cookie-tok"})
	if got := ExtractToken(req, "admin_token"); got != "cookie-tok" {
		t.Fatalf("cookie token = %q", got)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Authorization", "Bearer header-tok")
	if got := ExtractToken(req2, "admin_token"); got != "header-tok" {
		t.Fatalf("header token = %q", got)
	}
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := ExtractToken(req3, "admin_token"); got != "" {
		t.Fatalf("empty = %q", got)
	}
}

func TestHMACRoundTrip(t *testing.T) {
	t.Setenv("HMAC_SECRET", "")
	os.Unsetenv("HMAC_SECRET")
	dir := t.TempDir()
	Init(dir)
	if err := os.WriteFile(filepath.Join(dir, "hmac_secret.key"), []byte("test-secret-value-0123456789abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	ConfigureClients()

	key, err := GetDerivedKey("injector_v1")
	if err != nil {
		t.Fatalf("GetDerivedKey: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("derived key len = %d", len(key))
	}
	dllKey, err := GetDLLKey("injector_v1")
	if err != nil {
		t.Fatalf("GetDLLKey: %v", err)
	}
	if len(dllKey) != 32 {
		t.Fatalf("dll key len = %d", len(dllKey))
	}

	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("nonce-%d", time.Now().UnixNano())
	body := `{"ok":true}`
	signed := ts + "|" + nonce + "|" + body
	sig := signHMACBytes(key, signed)
	if err := VerifyHMAC("injector_v1", body, sig, ts, nonce); err != nil {
		t.Fatalf("VerifyHMAC: %v", err)
	}
	if err := VerifyHMAC("injector_v1", body, sig, ts, nonce); err == nil {
		t.Fatal("expected replay failure")
	}
	if err := VerifyHMAC("", body, sig, ts, "n2"); err == nil {
		t.Fatal("expected missing fields error")
	}
	if err := VerifyHMAC("injector_v1", body, sig, ts, ""); err == nil {
		t.Fatal("expected empty nonce error")
	}

	simpleSig := signHMACBytes(key, "GET|/api/v1/dll")
	if err := VerifyHMACSimple("injector_v1", "GET|/api/v1/dll", simpleSig); err != nil {
		t.Fatalf("VerifyHMACSimple: %v", err)
	}
	if err := VerifyHMACSimple("injector_v1", "GET|/api/v1/dll", "deadbeef"); err == nil {
		t.Fatal("expected invalid simple signature")
	}

	nonce2 := "another-nonce-" + nonce
	sig2 := signHMACBytes(key, ts+"|"+nonce2+"|"+body)
	req := httptest.NewRequest(http.MethodPost, "/api", nil)
	req.Header.Set("X-HMAC-Signature", sig2)
	req.Header.Set("X-Client-ID", "injector_v1")
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce2)
	if err := VerifyRequest(req, body); err != nil {
		t.Fatalf("VerifyRequest: %v", err)
	}
	reqBad := httptest.NewRequest(http.MethodPost, "/api", nil)
	if err := VerifyRequest(reqBad, body); err == nil {
		t.Fatal("expected missing header error")
	}
	if _, err := GetDerivedKey("no-such-client"); err == nil {
		t.Fatal("expected unknown client")
	}

	// SignHMAC uses raw secret string (legacy helper)
	if SignHMAC("sec", "data") == "" {
		t.Fatal("SignHMAC empty")
	}
}

func TestNonceTrackerRejectsEmpty(t *testing.T) {
	nt := &nonceTracker{nonces: map[string]time.Time{}}
	if nt.checkAndStore("", time.Now()) {
		t.Fatal("empty nonce must be rejected")
	}
	if !nt.checkAndStore("n1", time.Now()) {
		t.Fatal("first nonce should accept")
	}
	if nt.checkAndStore("n1", time.Now()) {
		t.Fatal("duplicate nonce must reject")
	}
}
