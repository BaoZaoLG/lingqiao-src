package main

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// compareVersion returns true if client version is older than latest.
func compareVersion(client, latest string) bool {
	parse := func(s string) []int {
		parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
		nums := make([]int, len(parts))
		for i, p := range parts {
			n := 0
			for _, c := range p {
				if c >= '0' && c <= '9' {
					n = n*10 + int(c-'0')
				} else {
					break
				}
			}
			nums[i] = n
		}
		return nums
	}
	cv := parse(client)
	lv := parse(latest)
	maxLen := len(cv)
	if len(lv) > maxLen {
		maxLen = len(lv)
	}
	for i := 0; i < maxLen; i++ {
		a, b := 0, 0
		if i < len(cv) {
			a = cv[i]
		}
		if i < len(lv) {
			b = lv[i]
		}
		if a < b {
			return true
		}
		if a > b {
			return false
		}
	}
	return false
}

// generateCardCode creates an 18-char Crockford Base32 card code in XXXXXX-XXXXXX-XXXXXX format.
func generateCardCode() (string, error) {
	buf := make([]byte, 15)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	var sb strings.Builder
	for i := 0; i < 24 && sb.Len() < 18; i++ {
		idx := int(buf[i%15]) & 31
		if i < 15 {
			idx = int(buf[i]) & 31
		} else {
			idx = int(buf[i-15]>>5|buf[i%15]<<3) & 31
		}
		sb.WriteByte(crockford[idx])
	}
	result := sb.String()[:18]

	return fmt.Sprintf("%s-%s-%s",
		result[0:6],
		result[6:12],
		result[12:18],
	), nil
}

func generateSessionToken() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}

func normalizeCardCode(code string) string {
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func FormatCardCode(code string) string {
	code = normalizeCardCode(code)
	if len(code) != 18 {
		return code
	}
	return fmt.Sprintf("%s-%s-%s", code[0:6], code[6:12], code[12:18])
}

func generateShortID() string {
	buf := make([]byte, 6)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}
