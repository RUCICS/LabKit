package commands

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"labkit.local/apps/cli/internal/config"

	"github.com/spf13/cobra"
)

type webSessionTicketResponse struct {
	RedirectURL string `json:"redirect_url"`
}

func NewWebCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	return &cobra.Command{
		Use:   "web [path]",
		Short: "Open a browser session for LabKit web",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			redirectPath := "/profile"
			if len(args) == 1 {
				redirectPath = strings.TrimSpace(args[0])
			}
			return runWeb(cmd.Context(), deps, redirectPath)
		},
	}
}

func runWeb(ctx context.Context, deps *Dependencies, redirectPath string) error {
	redirectPath, err := normalizeWebRedirectPath(redirectPath)
	if err != nil {
		return err
	}

	cfg, err := resolveConfig(deps)
	if err != nil {
		return err
	}
	if cfg.KeyID == 0 {
		return fmt.Errorf("key id is required; run auth first")
	}
	privateKey, err := readPrivateKeyWithDeps(deps, cfg.KeyPath)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL(deps)
	if err != nil {
		return err
	}

	client, err := newAPIClient(serverURL, deps.HTTPClient, deps.Now)
	if err != nil {
		return err
	}
	waitSpinner := newLineSpinner(deps.Out, deps.IsTTY(), "Requesting browser session…")
	if err := waitSpinner.Start(); err != nil {
		return err
	}
	req, err := client.webSessionTicket(ctx, cfg, privateKey, redirectPath)
	if err != nil {
		_ = waitSpinner.Stop()
		return err
	}

	var ticket webSessionTicketResponse
	if err := client.doJSON(req, &ticket); err != nil {
		_ = waitSpinner.Stop()
		return err
	}

	targetURL, err := resolveWebRedirectURL(serverURL, ticket.RedirectURL)
	if err != nil {
		_ = waitSpinner.Stop()
		return err
	}
	_ = waitSpinner.Update("Opening browser…")

	if err := deps.OpenBrowser(targetURL); err != nil {
		_ = waitSpinner.Stop()
		fmt.Fprintf(deps.Out, "Could not open browser automatically: %v\nOpen this URL: %s\n", err, targetURL)
		return nil
	}
	_ = waitSpinner.Stop()

	fmt.Fprintln(deps.Out, "Opened browser session.")
	return nil
}

func (c *apiClient) webSessionTicket(ctx context.Context, cfg config.Config, private ed25519.PrivateKey, redirectPath string) (*http.Request, error) {
	body := map[string]string{"redirect_path": redirectPath}
	return c.signedRequest(ctx, http.MethodPost, "/api/web/session-ticket", body, cfg, private)
}

func normalizeWebRedirectPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		path = "/profile"
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("redirect path must be a site-relative path")
	}
	parsed, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() || parsed.Host != "" || strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("redirect path must be a site-relative path")
	}
	return path, nil
}

func resolveWebRedirectURL(baseURL, redirectURL string) (string, error) {
	raw := strings.TrimSpace(redirectURL)
	if raw == "" {
		return "", fmt.Errorf("redirect_url is required")
	}

	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if !isAllowedWebScheme(base.Scheme) || base.Host == "" {
		return "", fmt.Errorf("server URL must use http or https")
	}

	ref, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	target := base.ResolveReference(ref)
	if !isAllowedWebScheme(target.Scheme) {
		return "", fmt.Errorf("redirect_url scheme must be http or https")
	}
	if !sameOrigin(base, target) {
		return "", fmt.Errorf("redirect_url must stay on the same origin")
	}
	return target.String(), nil
}

func isAllowedWebScheme(scheme string) bool {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "http", "https":
		return true
	default:
		return false
	}
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(a.Scheme), strings.TrimSpace(b.Scheme)) &&
		strings.EqualFold(strings.TrimSpace(a.Host), strings.TrimSpace(b.Host))
}
