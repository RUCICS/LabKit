package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// OAuthConfig holds the selected auth provider and its config.
type OAuthConfig struct {
	Provider        string
	CASRUC          CASRUCConfig
	SchoolDevcenter SchoolDevcenterConfig
	DeviceAuthTTL   time.Duration
}

// CASRUCConfig is the provider-specific configuration for cas_ruc.
type CASRUCConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthorizeURL string
	TokenURL     string
	ProfileURL   string
}

// SchoolDevcenterConfig is the provider-specific configuration for school_devcenter.
type SchoolDevcenterConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthorizeURL string
	TokenURL     string
	UserURL      string
	ProfileURL   string
	Scope        string
}

const (
	oauthProviderEnv      = "LABKIT_AUTH_PROVIDER"
	oauthClientIDEnv      = "LABKIT_OAUTH_CLIENT_ID"
	oauthClientSecretEnv  = "LABKIT_OAUTH_CLIENT_SECRET"
	oauthRedirectURLEnv   = "LABKIT_OAUTH_REDIRECT_URL"
	oauthAuthorizeURLEnv  = "LABKIT_OAUTH_AUTHORIZE_URL"
	oauthTokenURLEnv      = "LABKIT_OAUTH_TOKEN_URL"
	oauthProfileURLEnv    = "LABKIT_OAUTH_PROFILE_URL"
	oauthUserURLEnv       = "LABKIT_OAUTH_USER_URL"
	oauthScopeEnv         = "LABKIT_OAUTH_SCOPE"
	oauthDeviceAuthTTLEnv = "LABKIT_OAUTH_DEVICE_AUTH_TTL"

	defaultOAuthProvider = "cas_ruc"
)

func ResolveOAuthConfigFromEnv() (OAuthConfig, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv(oauthProviderEnv)))
	if provider == "" {
		provider = defaultOAuthProvider
	}

	switch provider {
	case defaultOAuthProvider:
	case "school_devcenter":
	default:
		return OAuthConfig{}, fmt.Errorf("unsupported auth provider %q", provider)
	}

	cfg := OAuthConfig{
		Provider: provider,
	}

	clientID := strings.TrimSpace(os.Getenv(oauthClientIDEnv))
	clientSecret := strings.TrimSpace(os.Getenv(oauthClientSecretEnv))
	redirectURL := strings.TrimSpace(os.Getenv(oauthRedirectURLEnv))

	switch provider {
	case defaultOAuthProvider:
		cfg.CASRUC = CASRUCConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			AuthorizeURL: strings.TrimSpace(os.Getenv(oauthAuthorizeURLEnv)),
			TokenURL:     strings.TrimSpace(os.Getenv(oauthTokenURLEnv)),
			ProfileURL:   strings.TrimSpace(os.Getenv(oauthProfileURLEnv)),
		}
	case "school_devcenter":
		cfg.SchoolDevcenter = SchoolDevcenterConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			AuthorizeURL: strings.TrimSpace(os.Getenv(oauthAuthorizeURLEnv)),
			TokenURL:     strings.TrimSpace(os.Getenv(oauthTokenURLEnv)),
			UserURL:      strings.TrimSpace(os.Getenv(oauthUserURLEnv)),
			ProfileURL:   strings.TrimSpace(os.Getenv(oauthProfileURLEnv)),
			Scope:        strings.TrimSpace(os.Getenv(oauthScopeEnv)),
		}
	}

	if ttl := strings.TrimSpace(os.Getenv(oauthDeviceAuthTTLEnv)); ttl != "" {
		duration, err := time.ParseDuration(ttl)
		if err != nil {
			return OAuthConfig{}, fmt.Errorf("%s: %w", oauthDeviceAuthTTLEnv, err)
		}
		cfg.DeviceAuthTTL = duration
	}

	missing := []string(nil)
	switch provider {
	case defaultOAuthProvider:
		missing = missingCASRUCEnvVars(cfg.CASRUC)
	case "school_devcenter":
		missing = missingSchoolDevcenterEnvVars(cfg.SchoolDevcenter)
	}
	if len(missing) > 0 {
		return OAuthConfig{}, fmt.Errorf("missing required %s env vars: %s", provider, strings.Join(missing, ", "))
	}

	return cfg, nil
}

func missingCASRUCEnvVars(cfg CASRUCConfig) []string {
	missing := make([]string, 0, 6)
	appendMissing := func(envName, value string) {
		if value == "" {
			missing = append(missing, envName)
		}
	}
	appendMissing(oauthClientIDEnv, cfg.ClientID)
	appendMissing(oauthClientSecretEnv, cfg.ClientSecret)
	appendMissing(oauthRedirectURLEnv, cfg.RedirectURL)
	appendMissing(oauthAuthorizeURLEnv, cfg.AuthorizeURL)
	appendMissing(oauthTokenURLEnv, cfg.TokenURL)
	appendMissing(oauthProfileURLEnv, cfg.ProfileURL)
	return missing
}

func missingSchoolDevcenterEnvVars(cfg SchoolDevcenterConfig) []string {
	missing := make([]string, 0, 7)
	appendMissing := func(envName, value string) {
		if value == "" {
			missing = append(missing, envName)
		}
	}
	appendMissing(oauthClientIDEnv, cfg.ClientID)
	appendMissing(oauthClientSecretEnv, cfg.ClientSecret)
	appendMissing(oauthRedirectURLEnv, cfg.RedirectURL)
	appendMissing(oauthAuthorizeURLEnv, cfg.AuthorizeURL)
	appendMissing(oauthTokenURLEnv, cfg.TokenURL)
	appendMissing(oauthUserURLEnv, cfg.UserURL)
	appendMissing(oauthProfileURLEnv, cfg.ProfileURL)
	appendMissing(oauthScopeEnv, cfg.Scope)
	return missing
}
