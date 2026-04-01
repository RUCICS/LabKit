package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestPublicKeyFingerprint(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(bytesRepeat(0x11, ed25519.SeedSize))
	pub := priv.Public().(ed25519.PublicKey)
	authorized, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh.NewPublicKey() error = %v", err)
	}
	sum := fingerprintDigest(authorized.Marshal())
	want := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum)

	got, err := PublicKeyFingerprint(priv.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("PublicKeyFingerprint() error = %v", err)
	}
	if got != want {
		t.Fatalf("PublicKeyFingerprint() = %q, want %q", got, want)
	}
}

func TestWriteAndReadPrivateKeyPlaintext(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	priv := ed25519.NewKeyFromSeed(bytesRepeat(0x22, ed25519.SeedSize))

	if err := WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	got, err := ReadPrivateKey(keyPath)
	if err != nil {
		t.Fatalf("ReadPrivateKey() error = %v", err)
	}
	if string(got) != string(priv) {
		t.Fatalf("ReadPrivateKey() = %x, want %x", got, priv)
	}
}

func TestWriteAndReadPrivateKeyEncrypted(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	priv := ed25519.NewKeyFromSeed(bytesRepeat(0x33, ed25519.SeedSize))
	passphrase := []byte("correct horse battery staple")

	if err := WriteEncryptedPrivateKey(keyPath, priv, passphrase); err != nil {
		t.Fatalf("WriteEncryptedPrivateKey() error = %v", err)
	}

	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatalf("pem.Decode() = nil for %q", string(raw))
	}
	if block.Type != encryptedPrivateKeyType {
		t.Fatalf("encrypted key block type = %q, want %q", block.Type, encryptedPrivateKeyType)
	}
	if got := block.Headers["Proc-Type"]; got != "4,ENCRYPTED" {
		t.Fatalf("encrypted key Proc-Type = %q, want %q", got, "4,ENCRYPTED")
	}
	if got := block.Headers["DEK-Info"]; got == "" {
		t.Fatal("encrypted key missing DEK-Info header")
	}
	if strings.Contains(string(raw), "PRIVATE KEY") && !strings.Contains(string(raw), "DEK-Info") {
		t.Fatalf("encrypted key file = %q, want encrypted PEM", string(raw))
	}

	got, err := ReadEncryptedPrivateKey(keyPath, passphrase)
	if err != nil {
		t.Fatalf("ReadEncryptedPrivateKey() error = %v", err)
	}
	if string(got) != string(priv) {
		t.Fatalf("ReadEncryptedPrivateKey() = %x, want %x", got, priv)
	}
}

func TestReadEncryptedPrivateKeyRejectsWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	priv := ed25519.NewKeyFromSeed(bytesRepeat(0x44, ed25519.SeedSize))

	if err := WriteEncryptedPrivateKey(keyPath, priv, []byte("correct")); err != nil {
		t.Fatalf("WriteEncryptedPrivateKey() error = %v", err)
	}

	if _, err := ReadEncryptedPrivateKey(keyPath, []byte("wrong")); err == nil {
		t.Fatal("ReadEncryptedPrivateKey() error = nil, want failure")
	}
}

func TestWriteEncryptedPrivateKeyRejectsEmptyPassphrase(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	priv := ed25519.NewKeyFromSeed(bytesRepeat(0x55, ed25519.SeedSize))

	if err := WriteEncryptedPrivateKey(keyPath, priv, nil); err == nil {
		t.Fatal("WriteEncryptedPrivateKey() error = nil, want failure")
	}

	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("key file exists after rejected write, stat error = %v", err)
	}
}

func bytesRepeat(value byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = value
	}
	return out
}

func fingerprintDigest(public ed25519.PublicKey) []byte {
	sum := sha256.Sum256(public)
	return sum[:]
}
