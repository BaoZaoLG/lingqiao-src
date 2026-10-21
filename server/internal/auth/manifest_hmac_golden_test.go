package auth

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type manifestHmacGolden struct {
	Version    int `json:"version"`
	DerivedKey struct {
		Salt       string `json:"salt"`
		Iterations int    `json:"iterations"`
		KeyLength  int    `json:"key_length"`
		Cases      []struct {
			ClientSecret   string `json:"client_secret"`
			ExpectedKeyHex string `json:"expected_key_hex"`
		} `json:"cases"`
	} `json:"derived_key"`
	ManifestHMAC struct {
		Cases []struct {
			ClientSecret    string `json:"client_secret"`
			Payload         string `json:"payload"`
			ExpectedHmacHex string `json:"expected_hmac_hex"`
		} `json:"cases"`
	} `json:"manifest_hmac"`
}

func TestManifestHMACGoldenVectors(t *testing.T) {
	data, err := os.ReadFile("../../../docs/api/manifest-hmac-golden-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden manifestHmacGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != 1 {
		t.Fatalf("unexpected golden version %d", golden.Version)
	}
	if golden.DerivedKey.Salt != "CefBridge-HMAC-Salt-v2" || golden.DerivedKey.Iterations != 100000 {
		t.Fatalf("unexpected kdf parameters: %+v", golden.DerivedKey)
	}

	for _, tc := range golden.DerivedKey.Cases {
		key := pbkdf2([]byte(tc.ClientSecret), []byte(golden.DerivedKey.Salt),
			golden.DerivedKey.Iterations, golden.DerivedKey.KeyLength)
		if got := hex.EncodeToString(key); got != tc.ExpectedKeyHex {
			t.Errorf("derived key for %q = %s, want %s", tc.ClientSecret, got, tc.ExpectedKeyHex)
		}
	}

	for _, tc := range golden.ManifestHMAC.Cases {
		key := pbkdf2([]byte(tc.ClientSecret), []byte(golden.DerivedKey.Salt),
			golden.DerivedKey.Iterations, golden.DerivedKey.KeyLength)
		if got := SignHMAC(string(key), tc.Payload); got != tc.ExpectedHmacHex {
			t.Errorf("manifest hmac = %s, want %s", got, tc.ExpectedHmacHex)
		}
	}
}
