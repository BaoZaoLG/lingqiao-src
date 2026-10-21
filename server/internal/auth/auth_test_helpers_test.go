package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func signHMACBytes(key []byte, data string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
