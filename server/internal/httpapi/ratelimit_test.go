package httpapi

import (
	"testing"
	"time"
)

func TestRateLimiterAllowAndClear(t *testing.T) {
	rl := NewRateLimiter(time.Minute, 2)
	if !rl.Allow("k") || !rl.Allow("k") {
		t.Fatal("first two should allow")
	}
	if rl.Allow("k") {
		t.Fatal("third should deny")
	}
	rl.Clear("k")
	if !rl.Allow("k") {
		t.Fatal("after clear should allow")
	}
}

func TestRateLimiterMaxKeys(t *testing.T) {
	rl := NewRateLimiter(time.Minute, 5)
	rl.maxKeys = 2
	if !rl.Allow("a") || !rl.Allow("b") {
		t.Fatal("expected allow within cap")
	}
	if rl.Allow("c") {
		t.Fatal("expected deny when key cap reached")
	}
}
