package releases

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const ManifestSeedSize = ed25519.SeedSize

// ManifestSigner ...
type ManifestSigner struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

// NewManifestSigner ...
func NewManifestSigner(seed []byte) (*ManifestSigner, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("manifest signer seed must be %d bytes", ed25519.SeedSize)
	}
	private := ed25519.NewKeyFromSeed(seed)
	public := make([]byte, ed25519.PublicKeySize)
	copy(public, private.Public().(ed25519.PublicKey))
	return &ManifestSigner{private: private, public: public}, nil
}

// PublicKeyHex ...
func (s *ManifestSigner) PublicKeyHex() string {
	return hex.EncodeToString(s.public)
}

// Sign ...
func (s *ManifestSigner) Sign(manifest Manifest) (SignedManifest, error) {
	data, err := canonicalManifest(manifest)
	if err != nil {
		return SignedManifest{}, err
	}
	signature := ed25519.Sign(s.private, data)
	return SignedManifest{
		Manifest:  manifest,
		Signature: hex.EncodeToString(signature),
	}, nil
}

// VerifySignedManifest ...
func VerifySignedManifest(publicKeyHex string, signed SignedManifest) bool {
	publicKey, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	signature, err := hex.DecodeString(signed.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return false
	}
	data, err := canonicalManifest(signed.Manifest)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), data, signature)
}

func canonicalManifest(manifest Manifest) ([]byte, error) {
	return json.Marshal(manifest)
}
