package auth

import (
	"context"
	"sync"
)

// NonceStore records whether a nonce has already been used.
type NonceStore interface {
	ReserveNonce(context.Context, string) (bool, error)
}

// MemoryNonceStore is a simple in-memory replay guard for tests and local tooling.
type MemoryNonceStore struct {
	mu   sync.Mutex
	used map[string]struct{}
}

// NewMemoryNonceStore creates an empty in-memory nonce store.
func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{used: make(map[string]struct{})}
}

// ReserveNonce marks a nonce as used and returns false if it was already seen.
func (s *MemoryNonceStore) ReserveNonce(_ context.Context, nonce string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used == nil {
		s.used = make(map[string]struct{})
	}
	if _, ok := s.used[nonce]; ok {
		return false, nil
	}
	s.used[nonce] = struct{}{}
	return true, nil
}

// Used reports whether a nonce has already been reserved.
func (s *MemoryNonceStore) Used(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.used[nonce]
	return ok
}
