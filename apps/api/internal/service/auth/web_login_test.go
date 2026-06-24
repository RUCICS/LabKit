package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"labkit.local/apps/api/internal/config"
	"labkit.local/packages/go/db/sqlc"
)

func TestBuildWebLoginAuthorizeURLReturnsStateAndURL(t *testing.T) {
	repo := newFakeRepository()
	provider := &fakeProvider{}
	svc := newTestServiceWithProvider(repo, provider)

	url, state, err := svc.BuildWebLoginAuthorizeURL()
	if err != nil {
		t.Fatalf("BuildWebLoginAuthorizeURL() error = %v", err)
	}
	if state != "oauth-state" {
		t.Fatalf("state = %q, want %q", state, "oauth-state")
	}
	if url != "https://example.invalid/auth?state=oauth-state" {
		t.Fatalf("url = %q, want provider output", url)
	}
	if provider.authorizeCalls != 1 {
		t.Fatalf("BuildAuthorizeURL called %d times, want 1", provider.authorizeCalls)
	}
}

func TestBuildWebLoginAuthorizeURLRequiresProvider(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, nil, config.OAuthConfig{DeviceAuthTTL: time.Minute})

	_, _, err := svc.BuildWebLoginAuthorizeURL()
	if !errors.Is(err, ErrProviderNotConfigured) {
		t.Fatalf("BuildWebLoginAuthorizeURL() error = %v, want ErrProviderNotConfigured", err)
	}
}

func TestHandleWebLoginCallbackUpsertsUser(t *testing.T) {
	repo := newFakeRepository()
	provider := &fakeProvider{
		tokenSet: TokenSet{AccessToken: "access-token"},
		identity: ExternalIdentity{StudentID: "2026001"},
	}
	svc := newTestServiceWithProvider(repo, provider)

	result, err := svc.HandleWebLoginCallback(context.Background(), "auth-code")
	if err != nil {
		t.Fatalf("HandleWebLoginCallback() error = %v", err)
	}
	if result.StudentID != "2026001" {
		t.Fatalf("StudentID = %q, want %q", result.StudentID, "2026001")
	}
	if result.UserID != 1 {
		t.Fatalf("UserID = %d, want %d", result.UserID, 1)
	}
	if got := repo.usersByStudentID["2026001"].ID; got != 1 {
		t.Fatalf("user id = %d, want %d", got, 1)
	}
	if provider.exchangeCalls != 1 {
		t.Fatalf("ExchangeCode called %d times, want 1", provider.exchangeCalls)
	}
	if provider.identityCalls != 1 {
		t.Fatalf("FetchIdentity called %d times, want 1", provider.identityCalls)
	}
	// Web login must never touch the device tables.
	if repo.completeCalls != 0 {
		t.Fatalf("CompleteDeviceAuthRequest called %d times, want 0", repo.completeCalls)
	}
	if len(repo.keysByID) != 0 {
		t.Fatalf("unexpected user key inserted: %d", len(repo.keysByID))
	}
}

func TestHandleWebLoginCallbackReusesExistingUser(t *testing.T) {
	repo := newFakeRepository()
	// Simulate a user that already exists (e.g. created via the CLI device flow).
	repo.usersByStudentID["2026001"] = sqlc.Users{ID: 7, StudentID: "2026001"}
	repo.nextUserID = 8
	provider := &fakeProvider{identity: ExternalIdentity{StudentID: "2026001"}}
	svc := newTestServiceWithProvider(repo, provider)

	result, err := svc.HandleWebLoginCallback(context.Background(), "auth-code")
	if err != nil {
		t.Fatalf("HandleWebLoginCallback() error = %v", err)
	}
	if result.UserID != 7 {
		t.Fatalf("UserID = %d, want %d (existing CLI user)", result.UserID, 7)
	}
}

func TestHandleWebLoginCallbackRejectsEmptyStudentID(t *testing.T) {
	repo := newFakeRepository()
	provider := &fakeProvider{identity: ExternalIdentity{StudentID: ""}}
	svc := newTestServiceWithProvider(repo, provider)

	_, err := svc.HandleWebLoginCallback(context.Background(), "auth-code")
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("HandleWebLoginCallback() error = %v, want ErrInvalidCode", err)
	}
}

func TestHandleWebLoginCallbackRejectsEmptyCode(t *testing.T) {
	repo := newFakeRepository()
	provider := &fakeProvider{}
	svc := newTestServiceWithProvider(repo, provider)

	_, err := svc.HandleWebLoginCallback(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("HandleWebLoginCallback() error = %v, want ErrInvalidCode", err)
	}
	if provider.exchangeCalls != 0 {
		t.Fatalf("ExchangeCode called %d times, want 0", provider.exchangeCalls)
	}
}
