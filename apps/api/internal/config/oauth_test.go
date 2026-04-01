package config

import (
	"strings"
	"testing"
	"time"
)

func TestResolveOAuthConfigFromEnvUsesExplicitProvider(t *testing.T) {
	t.Setenv(oauthProviderEnv, "cas_ruc")
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
	if got.Provider != "cas_ruc" {
		t.Fatalf("Provider = %q, want %q", got.Provider, "cas_ruc")
	}
	if got.CASRUC.ClientID != "client-id" {
		t.Fatalf("CASRUC.ClientID = %q, want %q", got.CASRUC.ClientID, "client-id")
	}
	if got.CASRUC.ClientSecret != "client-secret" {
		t.Fatalf("CASRUC.ClientSecret = %q, want %q", got.CASRUC.ClientSecret, "client-secret")
	}
	if got.CASRUC.RedirectURL != "https://labkit.example.edu/api/device/verify" {
		t.Fatalf("CASRUC.RedirectURL = %q, want %q", got.CASRUC.RedirectURL, "https://labkit.example.edu/api/device/verify")
	}
	if got.CASRUC.AuthorizeURL != "https://sso.example.edu/oauth2/authorize" {
		t.Fatalf("CASRUC.AuthorizeURL = %q, want %q", got.CASRUC.AuthorizeURL, "https://sso.example.edu/oauth2/authorize")
	}
	if got.CASRUC.TokenURL != "https://sso.example.edu/oauth2/token" {
		t.Fatalf("CASRUC.TokenURL = %q, want %q", got.CASRUC.TokenURL, "https://sso.example.edu/oauth2/token")
	}
	if got.CASRUC.ProfileURL != "https://sso.example.edu/oauth2/profile" {
		t.Fatalf("CASRUC.ProfileURL = %q, want %q", got.CASRUC.ProfileURL, "https://sso.example.edu/oauth2/profile")
	}
	if got.DeviceAuthTTL != 15*time.Minute {
		t.Fatalf("DeviceAuthTTL = %v, want %v", got.DeviceAuthTTL, 15*time.Minute)
	}
}

func TestResolveOAuthConfigFromEnvDefaultsProviderToCASRUC(t *testing.T) {
	t.Setenv(oauthProviderEnv, "")
	t.Setenv(oauthClientIDEnv, "client-id")
	t.Setenv(oauthClientSecretEnv, "client-secret")
	t.Setenv(oauthRedirectURLEnv, "https://labkit.example.edu/api/device/verify")
	t.Setenv(oauthAuthorizeURLEnv, "https://sso.example.edu/oauth2/authorize")
	t.Setenv(oauthTokenURLEnv, "https://sso.example.edu/oauth2/token")
	t.Setenv(oauthProfileURLEnv, "https://sso.example.edu/oauth2/profile")

	got, err := ResolveOAuthConfigFromEnv()
	if err != nil {
		t.Fatalf("ResolveOAuthConfigFromEnv() error = %v", err)
	}
	if got.Provider != "cas_ruc" {
		t.Fatalf("Provider = %q, want default %q", got.Provider, "cas_ruc")
	}
	if got.CASRUC.ClientID != "client-id" {
		t.Fatalf("CASRUC.ClientID = %q, want %q", got.CASRUC.ClientID, "client-id")
	}
}

func TestResolveOAuthConfigFromEnvRejectsMissingRequiredValuesForProvider(t *testing.T) {
	t.Setenv(oauthProviderEnv, "cas_ruc")
	t.Setenv(oauthClientIDEnv, "")
	t.Setenv(oauthClientSecretEnv, "client-secret")
	t.Setenv(oauthRedirectURLEnv, "https://labkit.example.edu/api/device/verify")
	t.Setenv(oauthAuthorizeURLEnv, "https://sso.example.edu/oauth2/authorize")
	t.Setenv(oauthTokenURLEnv, "https://sso.example.edu/oauth2/token")
	t.Setenv(oauthProfileURLEnv, "https://sso.example.edu/oauth2/profile")

	_, err := ResolveOAuthConfigFromEnv()
	if err == nil {
		t.Fatal("ResolveOAuthConfigFromEnv() error = nil, want error")
	}
	if !strings.Contains(err.Error(), oauthClientIDEnv) {
		t.Fatalf("error %q does not mention missing env %q", err, oauthClientIDEnv)
	}
	if !strings.Contains(err.Error(), "cas_ruc") {
		t.Fatalf("error %q does not mention provider %q", err, "cas_ruc")
	}
}

func TestResolveOAuthConfigFromEnvRejectsUnsupportedProvider(t *testing.T) {
	t.Setenv(oauthProviderEnv, "unknown")

	if _, err := ResolveOAuthConfigFromEnv(); err == nil {
		t.Fatal("ResolveOAuthConfigFromEnv() error = nil, want error")
	} else if !strings.Contains(err.Error(), "unsupported auth provider") {
		t.Fatalf("error %q does not mention unsupported provider", err)
	}
}

func TestResolveOAuthConfigFromEnvSupportsSchoolDevcenter(t *testing.T) {
	t.Setenv(oauthProviderEnv, "school_devcenter")
	t.Setenv(oauthClientIDEnv, "client-id")
	t.Setenv(oauthClientSecretEnv, "client-secret")
	t.Setenv(oauthRedirectURLEnv, "https://labkit.example.edu/api/device/verify")
	t.Setenv(oauthAuthorizeURLEnv, "https://sso.example.edu/oauth2/authorize")
	t.Setenv(oauthTokenURLEnv, "https://sso.example.edu/oauth2/token")
	t.Setenv(oauthUserURLEnv, "https://sso.example.edu/oauth2/user")
	t.Setenv(oauthProfileURLEnv, "https://sso.example.edu/oauth2/profile")
	t.Setenv(oauthScopeEnv, "openid profile")

	got, err := ResolveOAuthConfigFromEnv()
	if err != nil {
		t.Fatalf("ResolveOAuthConfigFromEnv() error = %v", err)
	}
	if got.Provider != "school_devcenter" {
		t.Fatalf("Provider = %q, want %q", got.Provider, "school_devcenter")
	}
	if got.SchoolDevcenter.ClientID != "client-id" {
		t.Fatalf("SchoolDevcenter.ClientID = %q, want %q", got.SchoolDevcenter.ClientID, "client-id")
	}
	if got.SchoolDevcenter.ClientSecret != "client-secret" {
		t.Fatalf("SchoolDevcenter.ClientSecret = %q, want %q", got.SchoolDevcenter.ClientSecret, "client-secret")
	}
	if got.SchoolDevcenter.RedirectURL != "https://labkit.example.edu/api/device/verify" {
		t.Fatalf("SchoolDevcenter.RedirectURL = %q, want %q", got.SchoolDevcenter.RedirectURL, "https://labkit.example.edu/api/device/verify")
	}
	if got.SchoolDevcenter.AuthorizeURL != "https://sso.example.edu/oauth2/authorize" {
		t.Fatalf("SchoolDevcenter.AuthorizeURL = %q, want %q", got.SchoolDevcenter.AuthorizeURL, "https://sso.example.edu/oauth2/authorize")
	}
	if got.SchoolDevcenter.TokenURL != "https://sso.example.edu/oauth2/token" {
		t.Fatalf("SchoolDevcenter.TokenURL = %q, want %q", got.SchoolDevcenter.TokenURL, "https://sso.example.edu/oauth2/token")
	}
	if got.SchoolDevcenter.UserURL != "https://sso.example.edu/oauth2/user" {
		t.Fatalf("SchoolDevcenter.UserURL = %q, want %q", got.SchoolDevcenter.UserURL, "https://sso.example.edu/oauth2/user")
	}
	if got.SchoolDevcenter.ProfileURL != "https://sso.example.edu/oauth2/profile" {
		t.Fatalf("SchoolDevcenter.ProfileURL = %q, want %q", got.SchoolDevcenter.ProfileURL, "https://sso.example.edu/oauth2/profile")
	}
	if got.SchoolDevcenter.Scope != "openid profile" {
		t.Fatalf("SchoolDevcenter.Scope = %q, want %q", got.SchoolDevcenter.Scope, "openid profile")
	}
}
