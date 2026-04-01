package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

func mustSigningBytes(t *testing.T, payload Payload) []byte {
	t.Helper()
	got, err := payload.SigningBytes()
	if err != nil {
		t.Fatalf("SigningBytes() error = %v", err)
	}
	return got
}

func TestVerifySignatureAcceptsValidPayload(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	store := NewMemoryNonceStore()
	verifier := Verifier{
		Now:     func() time.Time { return time.Unix(1700000000, 0).UTC() },
		MaxSkew: 5 * time.Minute,
		Nonces:  store,
	}
	payload := NewPayload("lab-1", time.Unix(1700000000, 0).UTC(), "nonce-1", []string{"a.c", "b.c"})
	sig := ed25519.Sign(priv, mustSigningBytes(t, payload))

	if err := verifier.Verify(context.Background(), pub, payload, sig); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !store.Used("nonce-1") {
		t.Fatalf("expected nonce to be reserved")
	}
}

func TestVerifySignatureRejectsInvalidSignature(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	verifier := Verifier{
		Now:     func() time.Time { return time.Unix(1700000000, 0).UTC() },
		MaxSkew: 5 * time.Minute,
		Nonces:  NewMemoryNonceStore(),
	}
	payload := NewPayload("lab-1", time.Unix(1700000000, 0).UTC(), "nonce-2", []string{"a.c"})

	err = verifier.Verify(context.Background(), pub, payload, bytes.Repeat([]byte{0x42}, ed25519.SignatureSize))
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("Verify() error = %v, want ErrInvalidSignature", err)
	}
}

func TestVerifySignatureRejectsExpiredTimestamp(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	verifier := Verifier{
		Now:     func() time.Time { return time.Unix(1700000000, 0).UTC() },
		MaxSkew: 5 * time.Minute,
		Nonces:  NewMemoryNonceStore(),
	}
	payload := NewPayload("lab-1", time.Unix(1700000000-600, 0).UTC(), "nonce-3", []string{"a.c"})
	sig := ed25519.Sign(priv, mustSigningBytes(t, payload))

	err = verifier.Verify(context.Background(), pub, payload, sig)
	if !errors.Is(err, ErrExpiredTimestamp) {
		t.Fatalf("Verify() error = %v, want ErrExpiredTimestamp", err)
	}
}

func TestVerifySignatureRejectsReusedNonce(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	store := NewMemoryNonceStore()
	verifier := Verifier{
		Now:     func() time.Time { return time.Unix(1700000000, 0).UTC() },
		MaxSkew: 5 * time.Minute,
		Nonces:  store,
	}
	payload := NewPayload("lab-1", time.Unix(1700000000, 0).UTC(), "nonce-4", []string{"a.c"})
	sig := ed25519.Sign(priv, mustSigningBytes(t, payload))

	if err := verifier.Verify(context.Background(), pub, payload, sig); err != nil {
		t.Fatalf("first Verify() error = %v", err)
	}
	if err := verifier.Verify(context.Background(), pub, payload, sig); !errors.Is(err, ErrNonceReplayed) {
		t.Fatalf("second Verify() error = %v, want ErrNonceReplayed", err)
	}
}

func TestVerifySignatureRejectsMissingNonceStore(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	verifier := Verifier{
		Now:     func() time.Time { return time.Unix(1700000000, 0).UTC() },
		MaxSkew: 5 * time.Minute,
	}
	payload := NewPayload("lab-1", time.Unix(1700000000, 0).UTC(), "nonce-5", []string{"a.c"})
	sig := ed25519.Sign(priv, mustSigningBytes(t, payload))

	err = verifier.Verify(context.Background(), pub, payload, sig)
	if !errors.Is(err, ErrNonceStoreRequired) {
		t.Fatalf("Verify() error = %v, want ErrNonceStoreRequired", err)
	}
}

func TestPayloadCanonicalizationStability(t *testing.T) {
	files := []string{"b.c", "a.c"}
	payload := NewPayload("lab-1", time.Unix(1700000000, 123000000).UTC(), "nonce-5", files)

	got1 := mustSigningBytes(t, payload)
	files[0] = "mutated.c"
	got2 := mustSigningBytes(t, payload)

	if !bytes.Equal(got1, got2) {
		t.Fatalf("canonical bytes changed: %q vs %q", got1, got2)
	}
}

func TestPayloadSigningBytesIgnoresInputOrder(t *testing.T) {
	left := NewPayload("lab-1", time.Unix(1700000000, 123000000).UTC(), "nonce-6", []string{"b.c", "a.c"})
	right := NewPayload("lab-1", time.Unix(1700000000, 123000000).UTC(), "nonce-6", []string{"a.c", "b.c"})

	leftBytes := mustSigningBytes(t, left)
	rightBytes := mustSigningBytes(t, right)

	if !bytes.Equal(leftBytes, rightBytes) {
		t.Fatalf("signing bytes differ for equivalent file sets: %q vs %q", leftBytes, rightBytes)
	}
}

func TestPayloadSigningBytesIncludeContentHash(t *testing.T) {
	left := NewPayload("lab-1", time.Unix(1700000000, 123000000).UTC(), "nonce-6", []string{"a.c"}).WithContentHash("hash-a")
	right := NewPayload("lab-1", time.Unix(1700000000, 123000000).UTC(), "nonce-6", []string{"a.c"}).WithContentHash("hash-b")

	leftBytes := mustSigningBytes(t, left)
	rightBytes := mustSigningBytes(t, right)

	if bytes.Equal(leftBytes, rightBytes) {
		t.Fatalf("signing bytes unexpectedly equal for different content hashes: %q", leftBytes)
	}
}

func TestPayloadSigningBytesReturnsMarshalError(t *testing.T) {
	originalMarshal := jsonMarshal
	jsonMarshal = func(any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() {
		jsonMarshal = originalMarshal
	})

	_, err := NewPayload("lab-1", time.Unix(1700000000, 0).UTC(), "nonce-7", []string{"a.c"}).SigningBytes()
	if err == nil || err.Error() != "marshal failed" {
		t.Fatalf("SigningBytes() error = %v, want marshal failed", err)
	}
}
