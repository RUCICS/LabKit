package providers

import (
	"fmt"
	"strings"

	"labkit.local/apps/api/internal/config"
)

// New constructs the configured auth provider.
func New(cfg config.OAuthConfig) (Provider, error) {
	providerName := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if providerName == "" {
		providerName = "cas_ruc"
	}

	switch providerName {
	case "cas_ruc":
		return NewCASRUCProvider(NewOAuthHTTPClient(cfg.CASRUC), cfg.CASRUC), nil
	case "school_devcenter":
		return NewSchoolDevcenterProvider(cfg.SchoolDevcenter), nil
	default:
		return nil, fmt.Errorf("unsupported auth provider %q", providerName)
	}
}
