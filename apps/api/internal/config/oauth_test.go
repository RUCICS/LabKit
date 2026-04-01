package config

import (
	"testing"
	"time"
)

func TestResolveOAuthConfigFromEnv(t *testing.T) {
	t.Setenv(oauthClientIDEnv, "client-id")
	t.Setenv(oauthClientSecretEnv, "client-secret")
	t.Setenv(oauthRedirectURLEnv, "https://labkit.example.edu/api/device/verify")
	t.Setenv(oauthAuthorizeURLEnv, "https://sso.example.edu/oauth2/authorize")
	t.Setenv(oauthTokenURLEnv, "https://sso.example.edu/oauth2/token")
	t.Setenv(oauthProfileURLEnv, "https://sso.example.edu/oauth2/profile")
	t.Setenv(oauthDeviceAuthTTLEnv, "15m")

	got, err := ResolveOAuthConfigFromEnv()
	if err != nil {
		t.Fatalf("ResolveOAuthConfigFromEnv() error = %v", err)
	}
	if got.ClientID != "client-id" {
		t.Fatalf("ClientID = %q, want %q", got.ClientID, "client-id")
	}
	if got.ClientSecret != "client-secret" {
		t.Fatalf("ClientSecret = %q, want %q", got.ClientSecret, "client-secret")
	}
	if got.RedirectURL != "https://labkit.example.edu/api/device/verify" {
		t.Fatalf("RedirectURL = %q, want %q", got.RedirectURL, "https://labkit.example.edu/api/device/verify")
	}
	if got.AuthorizeURL != "https://sso.example.edu/oauth2/authorize" {
		t.Fatalf("AuthorizeURL = %q, want %q", got.AuthorizeURL, "https://sso.example.edu/oauth2/authorize")
	}
	if got.TokenURL != "https://sso.example.edu/oauth2/token" {
		t.Fatalf("TokenURL = %q, want %q", got.TokenURL, "https://sso.example.edu/oauth2/token")
	}
	if got.ProfileURL != "https://sso.example.edu/oauth2/profile" {
		t.Fatalf("ProfileURL = %q, want %q", got.ProfileURL, "https://sso.example.edu/oauth2/profile")
	}
	if got.DeviceAuthTTL != 15*time.Minute {
		t.Fatalf("DeviceAuthTTL = %v, want %v", got.DeviceAuthTTL, 15*time.Minute)
	}
}

func TestResolveOAuthConfigFromEnvRejectsMissingRequiredValues(t *testing.T) {
	t.Setenv(oauthClientIDEnv, "")
	t.Setenv(oauthClientSecretEnv, "")
	t.Setenv(oauthRedirectURLEnv, "")
	t.Setenv(oauthAuthorizeURLEnv, "")
	t.Setenv(oauthTokenURLEnv, "")
	t.Setenv(oauthProfileURLEnv, "")

	if _, err := ResolveOAuthConfigFromEnv(); err == nil {
		t.Fatal("ResolveOAuthConfigFromEnv() error = nil, want error")
	}
}
