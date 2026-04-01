package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

const (
	privateKeyType          = "PRIVATE KEY"
	encryptedPrivateKeyType = "ENCRYPTED PRIVATE KEY"
)

type KeyPair struct {
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
}

func GenerateKeyPair() (KeyPair, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Private: private, Public: public}, nil
}

func PublicKeyFingerprint(public ed25519.PublicKey) (string, error) {
	authorized, err := ssh.NewPublicKey(public)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(authorized.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]), nil
}

func WritePrivateKey(path string, private ed25519.PrivateKey) error {
	return writePrivateKey(path, private, nil)
}

func WriteEncryptedPrivateKey(path string, private ed25519.PrivateKey, passphrase []byte) error {
	if len(passphrase) == 0 {
		return fmt.Errorf("passphrase is required for encrypted private keys")
	}
	return writePrivateKey(path, private, passphrase)
}

func writePrivateKey(path string, private ed25519.PrivateKey, passphrase []byte) error {
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return err
	}
	block := &pem.Block{Type: privateKeyType, Bytes: der}
	if len(passphrase) > 0 {
		block, err = x509.EncryptPEMBlock(rand.Reader, encryptedPrivateKeyType, der, passphrase, x509.PEMCipherAES256)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(block), 0o600)
}

func ReadPrivateKey(path string) (ed25519.PrivateKey, error) {
	return readPrivateKey(path, nil)
}

func ReadEncryptedPrivateKey(path string, passphrase []byte) (ed25519.PrivateKey, error) {
	return readPrivateKey(path, passphrase)
}

func readPrivateKey(path string, passphrase []byte) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid private key file")
	}
	if x509.IsEncryptedPEMBlock(block) {
		if len(passphrase) == 0 {
			return nil, fmt.Errorf("private key is encrypted; passphrase required")
		}
		der, err := x509.DecryptPEMBlock(block, passphrase)
		if err != nil {
			return nil, fmt.Errorf("decrypt private key: %w", err)
		}
		return parsePrivateKeyDER(der)
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	private, ok := value.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not ed25519")
	}
	return private, nil
}

func parsePrivateKeyDER(der []byte) (ed25519.PrivateKey, error) {
	value, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	private, ok := value.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not ed25519")
	}
	return private, nil
}

func PublicKeyString(public ed25519.PublicKey) (string, error) {
	authorized, err := ssh.NewPublicKey(public)
	if err != nil {
		return "", err
	}
	return string(ssh.MarshalAuthorizedKey(authorized)), nil
}

func dir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
