package sessions

import "testing"

func TestGenerateTokenFormatAndUniqueness(t *testing.T) {
	first := GenerateToken()
	second := GenerateToken()
	if first == second {
		t.Fatal("GenerateToken returned duplicate values")
	}
	if len(first) != 36 {
		t.Fatalf("token length = %d, want 36", len(first))
	}
}
