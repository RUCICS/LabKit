package auth

import (
	"labkit.local/apps/api/internal/config"
	"labkit.local/apps/api/internal/service/auth/providers"
)

type OAuthHTTPClient = providers.OAuthHTTPClient

func NewOAuthHTTPClient(cfg config.CASRUCConfig) *OAuthHTTPClient {
	return providers.NewOAuthHTTPClient(cfg)
}

func parseAccessToken(body []byte) (string, error) {
	return providers.ParseAccessToken(body)
}
