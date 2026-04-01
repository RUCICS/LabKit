package providers

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"labkit.local/apps/api/internal/config"
)

type TokenSet struct {
	AccessToken string
	TokenType   string
	Scope       string
	Expiry      time.Time
}

type OAuthProfile struct {
	LoginName string
}

type ExternalIdentity struct {
	Provider  string
	Subject   string
	StudentID string
	Name      string
	Email     string
}

type Provider interface {
	Name() string
	BuildAuthorizeURL(state string) (string, error)
	ExchangeCode(context.Context, string) (TokenSet, error)
	FetchIdentity(context.Context, TokenSet) (ExternalIdentity, error)
}

type OAuthClient interface {
	ExchangeCode(context.Context, string) (string, error)
	FetchProfile(context.Context, string) (OAuthProfile, error)
}

type CASRUCProvider struct {
	oauth OAuthClient
	cfg   config.CASRUCConfig
}

func NewCASRUCProvider(oauth OAuthClient, cfg config.CASRUCConfig) Provider {
	return &CASRUCProvider{oauth: oauth, cfg: cfg}
}

func (p *CASRUCProvider) Name() string {
	return "cas_ruc"
}

func (p *CASRUCProvider) BuildAuthorizeURL(state string) (string, error) {
	authorizeURL := strings.TrimSpace(p.cfg.AuthorizeURL)
	if authorizeURL == "" {
		return "", fmt.Errorf("oauth authorize url is required")
	}
	redirectURL := strings.TrimSpace(p.cfg.RedirectURL)
	if redirectURL == "" {
		redirectURL = "/api/device/verify"
	}
	parsed, err := url.Parse(authorizeURL)
	if err != nil {
		return "", err
	}
	params := parsed.Query()
	params.Set("response_type", "code")
	params.Set("scope", "all")
	params.Set("redirect_uri", redirectURL)
	params.Set("state", state)
	if clientID := strings.TrimSpace(p.cfg.ClientID); clientID != "" {
		params.Set("client_id", clientID)
	}
	parsed.RawQuery = params.Encode()
	return parsed.String(), nil
}

func (p *CASRUCProvider) ExchangeCode(ctx context.Context, code string) (TokenSet, error) {
	if p.oauth == nil {
		return TokenSet{}, fmt.Errorf("oauth backend is required")
	}
	accessToken, err := p.oauth.ExchangeCode(ctx, code)
	if err != nil {
		return TokenSet{}, err
	}
	return TokenSet{AccessToken: accessToken}, nil
}

func (p *CASRUCProvider) FetchIdentity(ctx context.Context, token TokenSet) (ExternalIdentity, error) {
	if p.oauth == nil {
		return ExternalIdentity{}, fmt.Errorf("oauth backend is required")
	}
	profile, err := p.oauth.FetchProfile(ctx, token.AccessToken)
	if err != nil {
		return ExternalIdentity{}, err
	}
	subject := strings.TrimSpace(profile.LoginName)
	if subject == "" {
		return ExternalIdentity{}, fmt.Errorf("oauth profile missing loginName")
	}
	return ExternalIdentity{
		Provider:  p.Name(),
		Subject:   subject,
		StudentID: subject,
		Name:      subject,
	}, nil
}
