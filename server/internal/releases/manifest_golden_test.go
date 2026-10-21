package releases

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"
)

type manifestGolden struct {
	Version      int    `json:"version"`
	Algorithm    string `json:"algorithm"`
	SeedHex      string `json:"seed_hex"`
	PublicKeyHex string `json:"public_key_hex"`
	Cases        []struct {
		Name     string `json:"name"`
		Manifest struct {
			ReleaseID      string    `json:"release_id"`
			Version        string    `json:"version"`
			Channel        string    `json:"channel"`
			MinVersion     string    `json:"min_version"`
			ForceUpdate    bool      `json:"force_update"`
			ReleaseNotes   string    `json:"release_notes"`
			PackageID      string    `json:"package_id"`
			PackageKind    string    `json:"package_kind"`
			PackageURL     string    `json:"package_url"`
			PackageSize    int64     `json:"package_size"`
			PackageSHA256  string    `json:"package_sha256"`
			ScriptID       string    `json:"script_id"`
			ScriptVersion  string    `json:"script_version"`
			ScriptSHA256   string    `json:"script_sha256"`
			RolloutPercent int       `json:"rollout_percent"`
			CreatedAt      time.Time `json:"created_at"`
		} `json:"manifest"`
		Canonical string `json:"canonical"`
		Signature string `json:"signature"`
	} `json:"cases"`
}

func TestManifestSignatureGoldenVectors(t *testing.T) {
	data, err := os.ReadFile("../../../docs/api/release-manifest-golden-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden manifestGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != 1 || golden.Algorithm != "Ed25519" {
		t.Fatalf("unexpected golden header: %+v", golden)
	}

	seed, err := hex.DecodeString(golden.SeedHex)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewManifestSigner(seed)
	if err != nil {
		t.Fatal(err)
	}
	if signer.PublicKeyHex() != golden.PublicKeyHex {
		t.Fatalf("public key = %s, want %s", signer.PublicKeyHex(), golden.PublicKeyHex)
	}

	for _, tc := range golden.Cases {
		m := Manifest{
			ReleaseID:      tc.Manifest.ReleaseID,
			Version:        tc.Manifest.Version,
			Channel:        Channel(tc.Manifest.Channel),
			MinVersion:     tc.Manifest.MinVersion,
			ForceUpdate:    tc.Manifest.ForceUpdate,
			ReleaseNotes:   tc.Manifest.ReleaseNotes,
			PackageID:      tc.Manifest.PackageID,
			PackageKind:    tc.Manifest.PackageKind,
			PackageURL:     tc.Manifest.PackageURL,
			PackageSize:    tc.Manifest.PackageSize,
			PackageSHA256:  tc.Manifest.PackageSHA256,
			ScriptID:       tc.Manifest.ScriptID,
			ScriptVersion:  tc.Manifest.ScriptVersion,
			ScriptSHA256:   tc.Manifest.ScriptSHA256,
			RolloutPercent: tc.Manifest.RolloutPercent,
			CreatedAt:      tc.Manifest.CreatedAt.UTC(),
		}

		canonical, err := canonicalManifest(m)
		if err != nil {
			t.Fatalf("%s: %v", tc.Name, err)
		}
		if string(canonical) != tc.Canonical {
			t.Errorf("%s canonical mismatch\n got: %s\nwant: %s", tc.Name, canonical, tc.Canonical)
		}

		signed, err := signer.Sign(m)
		if err != nil {
			t.Fatalf("%s: %v", tc.Name, err)
		}
		if signed.Signature != tc.Signature {
			t.Errorf("%s signature = %s, want %s", tc.Name, signed.Signature, tc.Signature)
		}
		if !VerifySignedManifest(golden.PublicKeyHex, signed) {
			t.Errorf("%s: verification failed", tc.Name)
		}

		tampered := signed
		tampered.Manifest.RolloutPercent++
		if VerifySignedManifest(golden.PublicKeyHex, tampered) {
			t.Errorf("%s: tampered manifest must not verify", tc.Name)
		}
	}
}
