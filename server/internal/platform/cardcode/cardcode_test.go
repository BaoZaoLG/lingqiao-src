package cardcode

import "testing"

func TestNormalize(t *testing.T) {
	got := Normalize(" abcd-efgh ijkl ")
	if got != "ABCDEFGHIJKL" {
		t.Fatalf("Normalize() = %q", got)
	}
}

func TestSame(t *testing.T) {
	if !Same("ABCD-EFGH", "abcd efgh") {
		t.Fatal("Same() returned false for equivalent card codes")
	}
}
