package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// OAuthConfig holds the external CAS endpoints and local callback wiring.
type OAuthConfig struct {
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	AuthorizeURL  string
	TokenURL      string
	ProfileURL    string
	DeviceAuthTTL time.Duration
}

const (
	oauthClientIDEnv      = "LABKIT_OAUTH_CLIENT_ID"
	oauthClientSecretEnv  = "LABKIT_OAUTH_CLIENT_SECRET"
	oauthRedirectURLEnv   = "LABKIT_OAUTH_REDIRECT_URL"
	oauthAuthorizeURLEnv  = "LABKIT_OAUTH_AUTHORIZE_URL"
	oauthTokenURLEnv      = "LABKIT_OAUTH_TOKEN_URL"
	oauthProfileURLEnv    = "LABKIT_OAUTH_PROFILE_URL"
	oauthDeviceAuthTTLEnv = "LABKIT_OAUTH_DEVICE_AUTH_TTL"
)

func ResolveOAuthConfigFromEnv() (OAuthConfig, error) {
	cfg := OAuthConfig{
		ClientID:     strings.TrimSpace(os.Getenv(oauthClientIDEnv)),
		ClientSecret: strings.TrimSpace(os.Getenv(oauthClientSecretEnv)),
		RedirectURL:  strings.TrimSpace(os.Getenv(oauthRedirectURLEnv)),
		AuthorizeURL: strings.TrimSpace(os.Getenv(oauthAuthorizeURLEnv)),
		TokenURL:     strings.TrimSpace(os.Getenv(oauthTokenURLEnv)),
		ProfileURL:   strings.TrimSpace(os.Getenv(oauthProfileURLEnv)),
	}

	if ttl := strings.TrimSpace(os.Getenv(oauthDeviceAuthTTLEnv)); ttl != "" {
		duration, err := time.ParseDuration(ttl)
		if err != nil {
			return OAuthConfig{}, fmt.Errorf("%s: %w", oauthDeviceAuthTTLEnv, err)
		}
		cfg.DeviceAuthTTL = duration
	}

	missing := make([]string, 0, 6)
	for envName, value := range map[string]string{
		oauthClientIDEnv:     cfg.ClientID,
		oauthClientSecretEnv: cfg.ClientSecret,
		oauthRedirectURLEnv:  cfg.RedirectURL,
		oauthAuthorizeURLEnv: cfg.AuthorizeURL,
		oauthTokenURLEnv:     cfg.TokenURL,
		oauthProfileURLEnv:   cfg.ProfileURL,
	} {
		if value == "" {
			missing = append(missing, envName)
		}
	}
	if len(missing) > 0 {
		return OAuthConfig{}, fmt.Errorf("missing required OAuth env vars: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
