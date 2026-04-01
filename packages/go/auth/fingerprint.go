package auth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// PublicKeyFingerprint returns the SSH-style SHA256 fingerprint for an Ed25519 public key.
func PublicKeyFingerprint(public ed25519.PublicKey) string {
	authorized, err := ssh.NewPublicKey(public)
	if err != nil {
		sum := sha256.Sum256(public)
		return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(authorized.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// AuthorizedKeyFingerprint parses an SSH authorized-key string and returns its SSH-style SHA256 fingerprint.
func AuthorizedKeyFingerprint(raw string) (string, error) {
	parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(raw)))
	if err != nil {
		return "", err
	}
	cryptoKey, ok := parsed.(ssh.CryptoPublicKey)
	if !ok {
		return "", fmt.Errorf("public key does not expose crypto key")
	}
	edKey, ok := cryptoKey.CryptoPublicKey().(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("public key is not ed25519")
	}
	return PublicKeyFingerprint(edKey), nil
}
