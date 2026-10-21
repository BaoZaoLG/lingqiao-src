package payloads

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type payloadCryptoGolden struct {
	Version int `json:"version"`
	Case    struct {
		NormalizedCardCode       string `json:"normalized_card_code"`
		MachineID                string `json:"machine_id"`
		PayloadID                string `json:"payload_id"`
		AesKeyHex                string `json:"aes_key_hex"`
		HmacKeyHex               string `json:"hmac_key_hex"`
		IVHex                    string `json:"iv_hex"`
		EncryptionIVHex          string `json:"encryption_iv_hex"`
		ExpectedKEKHex           string `json:"expected_kek_hex"`
		ExpectedPaddedLength     int    `json:"expected_padded_length"`
		ExpectedEncryptedKeyHex  string `json:"expected_encrypted_key_hex"`
		ExpectedEncryptedHmacHex string `json:"expected_encrypted_hmac_hex"`
		PlaintextDLL             string `json:"plaintext_dll_utf8"`
		ExpectedCTRHex           string `json:"expected_ctr_ciphertext_hex"`
	} `json:"case"`
}

func TestPayloadCryptoGoldenVectors(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/api/payload-crypto-golden-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden payloadCryptoGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != 1 {
		t.Fatalf("unexpected golden version %d", golden.Version)
	}
	c := golden.Case

	aesKey := mustHex(t, c.AesKeyHex, 32)
	hmacKey := mustHex(t, c.HmacKeyHex, 32)
	iv := mustHex(t, c.IVHex, 16)
	encIV := mustHex(t, c.EncryptionIVHex, 16)

	kek := pbkdf2DeriveKey([]byte(c.NormalizedCardCode+c.MachineID), []byte("payload_key_"+c.PayloadID), 100000, 32)
	if got := hex.EncodeToString(kek); got != c.ExpectedKEKHex {
		t.Fatalf("kek = %s, want %s", got, c.ExpectedKEKHex)
	}

	combined := make([]byte, 0, 80)
	combined = append(combined, aesKey...)
	combined = append(combined, hmacKey...)
	combined = append(combined, iv...)
	padded := pkcs7Pad(combined, aes.BlockSize)
	if len(padded) != c.ExpectedPaddedLength {
		t.Fatalf("padded length = %d, want %d", len(padded), c.ExpectedPaddedLength)
	}

	kekBlock, err := aes.NewCipher(kek)
	if err != nil {
		t.Fatal(err)
	}
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(kekBlock, encIV).CryptBlocks(encrypted, padded)
	if got := hex.EncodeToString(encrypted); got != c.ExpectedEncryptedKeyHex {
		t.Errorf("encrypted key = %s, want %s", got, c.ExpectedEncryptedKeyHex)
	}

	mac := hmac.New(sha256.New, kek)
	mac.Write(encrypted)
	if got := hex.EncodeToString(mac.Sum(nil)); got != c.ExpectedEncryptedHmacHex {
		t.Errorf("encrypted hmac = %s, want %s", got, c.ExpectedEncryptedHmacHex)
	}

	payloadBlock, err := aes.NewCipher(aesKey)
	if err != nil {
		t.Fatal(err)
	}
	ctrCiphertext := make([]byte, len(c.PlaintextDLL))
	cipher.NewCTR(payloadBlock, iv).XORKeyStream(ctrCiphertext, []byte(c.PlaintextDLL))
	if got := hex.EncodeToString(ctrCiphertext); got != c.ExpectedCTRHex {
		t.Errorf("ctr ciphertext = %s, want %s", got, c.ExpectedCTRHex)
	}
}

func mustHex(t *testing.T, value string, size int) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != size {
		t.Fatalf("hex %s decoded to %d bytes, want %d", value, len(decoded), size)
	}
	return decoded
}
