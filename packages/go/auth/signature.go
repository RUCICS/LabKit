package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrExpiredTimestamp   = errors.New("expired timestamp")
	ErrNonceReplayed      = errors.New("nonce already used")
	ErrNonceStoreRequired = errors.New("nonce store required")
)

const defaultSkew = 5 * time.Minute

// Verifier checks signatures and replay conditions for signed CLI requests.
type Verifier struct {
	Now     func() time.Time
	MaxSkew time.Duration
	Nonces  NonceStore
}

// Verify checks signature validity, timestamp freshness, and nonce replay.
func (v Verifier) Verify(ctx context.Context, pub ed25519.PublicKey, payload Payload, sig []byte) error {
	if err := v.VerifySignature(pub, payload, sig); err != nil {
		return err
	}
	return v.ReserveNonce(ctx, payload.Nonce)
}

// VerifySignature checks signature validity and timestamp freshness without reserving the nonce.
func (v Verifier) VerifySignature(pub ed25519.PublicKey, payload Payload, sig []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: invalid public key", ErrInvalidSignature)
	}
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		return fmt.Errorf("build signing bytes: %w", err)
	}
	if !ed25519.Verify(pub, signingBytes, sig) {
		return ErrInvalidSignature
	}
	now := time.Now().UTC()
	if v.Now != nil {
		now = v.Now().UTC()
	}
	maxSkew := v.MaxSkew
	if maxSkew <= 0 {
		maxSkew = defaultSkew
	}
	if !freshWithin(now, payload.Timestamp.UTC(), maxSkew) {
		return ErrExpiredTimestamp
	}
	return nil
}

// ReserveNonce marks the nonce as used.
func (v Verifier) ReserveNonce(ctx context.Context, nonce string) error {
	if v.Nonces == nil {
		return ErrNonceStoreRequired
	}
	reserved, err := v.Nonces.ReserveNonce(ctx, nonce)
	if err != nil {
		return fmt.Errorf("reserve nonce: %w", err)
	}
	if !reserved {
		return ErrNonceReplayed
	}
	return nil
}

func freshWithin(now, ts time.Time, maxSkew time.Duration) bool {
	diff := now.Sub(ts)
	if diff < 0 {
		diff = -diff
	}
	return diff <= maxSkew
}
