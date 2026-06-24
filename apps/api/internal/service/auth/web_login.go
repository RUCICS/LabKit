package auth

import (
	"context"
	"fmt"
	"strings"
)

// WebLoginResult is the outcome of a successful 微人大 web login. It carries only
// the read-only identity needed to issue a browser session — no key binding,
// no device record.
type WebLoginResult struct {
	UserID    int64
	StudentID string
}

// BuildWebLoginAuthorizeURL starts a browser-only login: it mints a fresh CSRF
// state and returns the provider authorize URL alongside that state so the HTTP
// layer can persist it in a double-submit cookie. Unlike the device flow this
// does NOT touch device_auth_requests — the state lives entirely client-side.
func (s *Service) BuildWebLoginAuthorizeURL() (string, string, error) {
	if s == nil {
		return "", "", fmt.Errorf("auth service unavailable")
	}
	if s.provider == nil {
		return "", "", ErrProviderNotConfigured
	}
	state, err := s.newState()
	if err != nil {
		return "", "", err
	}
	authorizeURL, err := s.provider.BuildAuthorizeURL(state)
	if err != nil {
		return "", "", err
	}
	return authorizeURL, state, nil
}

// HandleWebLoginCallback exchanges the OAuth code, fetches the 微人大 identity and
// upserts the user by student_id, returning the identity for a browser session.
// CSRF/state verification is performed at the HTTP layer (double-submit cookie),
// so this method intentionally has no state-store dependency.
func (s *Service) HandleWebLoginCallback(ctx context.Context, code string) (WebLoginResult, error) {
	if s == nil || s.repo == nil {
		return WebLoginResult{}, fmt.Errorf("auth service unavailable")
	}
	if strings.TrimSpace(code) == "" {
		return WebLoginResult{}, ErrInvalidCode
	}
	if s.provider == nil {
		return WebLoginResult{}, ErrProviderNotConfigured
	}
	tokenSet, err := s.provider.ExchangeCode(ctx, code)
	if err != nil {
		return WebLoginResult{}, fmt.Errorf("%w: %v", ErrInvalidCode, err)
	}
	identity, err := s.provider.FetchIdentity(ctx, tokenSet)
	if err != nil {
		return WebLoginResult{}, err
	}
	studentID := strings.TrimSpace(identity.StudentID)
	if studentID == "" {
		return WebLoginResult{}, ErrInvalidCode
	}
	user, err := s.repo.UpsertUser(ctx, studentID)
	if err != nil {
		return WebLoginResult{}, err
	}
	return WebLoginResult{UserID: user.ID, StudentID: user.StudentID}, nil
}
