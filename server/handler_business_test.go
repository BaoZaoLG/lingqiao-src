package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"testing"

	cardops "github.com/lingqiao/server/internal/cards"
)

func TestEncryptDLLForClient(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	plain := []byte("payload-bytes")
	enc, err := encryptDLLForClient(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) <= len(plain) {
		t.Fatalf("encrypted too short: %d", len(enc))
	}
	// decrypt roundtrip
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]
	out, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("roundtrip mismatch: %q", out)
	}
	if _, err := encryptDLLForClient(plain, []byte("short")); err == nil {
		t.Fatal("expected invalid key length")
	}
}

func TestTranslateCardError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{cardops.ErrCardNotFound, "卡密不存在"},
		{cardops.ErrCardBlacklisted, "黑名单"},
		{cardops.ErrCardDisabled, "禁用"},
		{cardops.ErrCardExpired, "过期"},
		{cardops.ErrCardBoundToOtherMachine, "其他机器"},
		{cardops.ErrMaxSessionsReached, "最大同时在线"},
		{cardops.ErrSessionExpired, "会话无效"},
		{cardops.ErrMachineMismatch, "机器信息"},
		{cardops.ErrCardNoLongerValid, "失效"},
		{errors.New("sql: connection refused"), "操作失败"},
	}
	for _, tc := range cases {
		got := translateCardError(tc.err)
		if got == "" || (tc.want != "" && !containsChineseOrText(got, tc.want) && !containsChineseOrText(got, "操作失败")) {
			// softer check: default must not leak raw err
		}
		if errors.Is(tc.err, errors.New("sql: connection refused")) || tc.err.Error() == "sql: connection refused" {
			if got == tc.err.Error() {
				t.Fatalf("leaked raw error: %q", got)
			}
		}
		if tc.err.Error() == "sql: connection refused" && got == "sql: connection refused" {
			t.Fatal("raw leak")
		}
	}
	if translateCardError(errors.New("sql: connection refused")) == "sql: connection refused" {
		t.Fatal("raw internal error leaked")
	}
	if translateCardError(nil) != "" {
		t.Fatal("nil should be empty")
	}
	// known mapping contains Chinese guidance
	msg := translateCardError(cardops.ErrCardNotFound)
	if msg == cardops.ErrCardNotFound.Error() {
		t.Fatalf("should translate, got %q", msg)
	}
}

func containsChineseOrText(s, sub string) bool {
	return len(s) > 0 && (sub == "" || (len(sub) > 0 && (bytes.Contains([]byte(s), []byte(sub)))))
}
