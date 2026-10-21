package auth

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type dllTransportGolden struct {
	Version int `json:"version"`
	DLLKey  struct {
		Salt       string `json:"salt"`
		Iterations int    `json:"iterations"`
		KeyLength  int    `json:"key_length"`
		Cases      []struct {
			ClientSecret   string `json:"client_secret"`
			ExpectedKeyHex string `json:"expected_key_hex"`
		} `json:"cases"`
	} `json:"dll_key"`
}

func TestDLLKeyGoldenVectors(t *testing.T) {
	data, err := os.ReadFile("../../../docs/api/dll-transport-golden-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden dllTransportGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != 1 {
		t.Fatalf("unexpected golden version %d", golden.Version)
	}
	if golden.DLLKey.Salt != "CefBridge-DLL-Salt-v1" || golden.DLLKey.Iterations != 100000 {
		t.Fatalf("unexpected kdf parameters: %+v", golden.DLLKey)
	}

	for _, tc := range golden.DLLKey.Cases {
		key := pbkdf2([]byte(tc.ClientSecret), []byte(golden.DLLKey.Salt),
			golden.DLLKey.Iterations, golden.DLLKey.KeyLength)
		if got := hex.EncodeToString(key); got != tc.ExpectedKeyHex {
			t.Errorf("dll key for %q = %s, want %s", tc.ClientSecret, got, tc.ExpectedKeyHex)
		}
	}
}
