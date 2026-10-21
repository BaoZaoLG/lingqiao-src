package cards

import (
	"encoding/json"
	"os"
	"testing"
)

type cardCodeGolden struct {
	Version          int    `json:"version"`
	Alphabet         string `json:"alphabet"`
	NormalizedLength int    `json:"normalized_length"`
	Normalize        []struct {
		Input      string `json:"input"`
		Normalized string `json:"normalized"`
	} `json:"normalize"`
	Same []struct {
		A     string `json:"a"`
		B     string `json:"b"`
		Equal bool   `json:"equal"`
	} `json:"same"`
	FormatCases []struct {
		Input     string `json:"input"`
		Formatted string `json:"formatted"`
	} `json:"format_cases"`
	Validate []struct {
		Input      string `json:"input"`
		Valid      bool   `json:"valid"`
		Normalized string `json:"normalized"`
	} `json:"validate"`
}

func loadCardCodeGolden(t *testing.T) cardCodeGolden {
	t.Helper()
	data, err := os.ReadFile("../../../../docs/api/cardcode-golden-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden cardCodeGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != 1 || golden.Alphabet != crockford {
		t.Fatalf("unexpected golden manifest: version=%d alphabet=%q", golden.Version, golden.Alphabet)
	}
	return golden
}

func TestCardCodeGoldenVectors(t *testing.T) {
	golden := loadCardCodeGolden(t)

	for _, tc := range golden.Normalize {
		if got := NormalizeCode(tc.Input); got != tc.Normalized {
			t.Errorf("NormalizeCode(%q) = %q, want %q", tc.Input, got, tc.Normalized)
		}
	}
	for _, tc := range golden.Same {
		got := NormalizeCode(tc.A) == NormalizeCode(tc.B)
		if got != tc.Equal {
			t.Errorf("same(%q,%q) = %v, want %v", tc.A, tc.B, got, tc.Equal)
		}
	}
	for _, tc := range golden.FormatCases {
		if got := FormatCardCode(tc.Input); got != tc.Formatted {
			t.Errorf("FormatCardCode(%q) = %q, want %q", tc.Input, got, tc.Formatted)
		}
	}
	for _, tc := range golden.Validate {
		normalized, err := ValidateImportCardCode(tc.Input)
		if tc.Valid {
			if err != nil {
				t.Errorf("ValidateImportCardCode(%q) unexpected error: %v", tc.Input, err)
				continue
			}
			if normalized != tc.Normalized {
				t.Errorf("ValidateImportCardCode(%q) = %q, want %q", tc.Input, normalized, tc.Normalized)
			}
			continue
		}
		if err == nil {
			t.Errorf("ValidateImportCardCode(%q) should fail", tc.Input)
		}
	}
}

func TestGenerateCardCodeMatchesGoldenFormat(t *testing.T) {
	golden := loadCardCodeGolden(t)
	code, err := GenerateCardCode()
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatCardCode(code); got != code {
		t.Fatalf("generated code %q is not canonical (%q)", code, got)
	}
	normalized, err := ValidateImportCardCode(code)
	if err != nil {
		t.Fatalf("generated code %q rejected: %v", code, err)
	}
	if len(normalized) != golden.NormalizedLength {
		t.Fatalf("generated normalized length = %d, want %d", len(normalized), golden.NormalizedLength)
	}
}
