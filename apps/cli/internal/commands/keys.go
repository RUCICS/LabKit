package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"labkit.local/apps/cli/internal/ui"

	"github.com/spf13/cobra"
)

func NewKeysCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	return &cobra.Command{
		Use:   "keys",
		Short: "List bound keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListKeys(cmd.Context(), deps)
		},
	}
}

func NewRevokeCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	return &cobra.Command{
		Use:   "revoke <key-id>",
		Short: "Revoke a bound key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid key id")
			}
			if err := runRevokeKey(cmd.Context(), deps, keyID); err != nil {
				return err
			}
			theme := ui.DefaultTheme()
			fmt.Fprintln(deps.Out, theme.SuccessStyle.Render("✓")+" "+theme.TitleStyle.Render("Revoked")+"  "+theme.MutedStyle.Render(fmt.Sprintf("key %d", keyID)))
			return nil
		},
	}
}

func runListKeys(ctx context.Context, deps *Dependencies) error {
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
	keys, err := client.listKeys(ctx, cfg, privateKey)
	if err != nil {
		return err
	}
	theme := ui.DefaultTheme()
	now := time.Now()
	if deps != nil && deps.Now != nil {
		now = deps.Now()
	}

	const (
		idW     = 10
		deviceW = 14
		gap     = "  "
	)
	title := theme.TitleStyle.Render("Bound keys") +
		"  " + theme.MutedStyle.Render(fmt.Sprintf("%d total", len(keys)))
	if _, err = fmt.Fprintln(deps.Out, title+"\n"); err != nil {
		return err
	}
	header := ui.PadRight(theme.MutedStyle.Render("ID"), idW) + gap +
		ui.PadRight(theme.MutedStyle.Render("DEVICE"), deviceW) + gap +
		theme.MutedStyle.Render("CREATED")
	rowWidth := idW + len(gap) + deviceW + len(gap) + 8
	sep := theme.SeparatorStyle.Render(strings.Repeat("─", rowWidth))
	if _, err = fmt.Fprintln(deps.Out, "  "+header); err != nil {
		return err
	}
	if _, err = fmt.Fprintln(deps.Out, "  "+sep); err != nil {
		return err
	}
	for _, key := range keys {
		shortID := strconv.FormatInt(key.ID, 10)
		row := ui.PadRight(theme.ValueStyle.Render(shortID), idW) + gap +
			ui.PadRight(theme.ValueStyle.Render(key.DeviceName), deviceW) + gap +
			theme.MutedStyle.Render(ui.RelativeTime(key.CreatedAt, now))
		if _, err = fmt.Fprintln(deps.Out, "  "+row); err != nil {
			return err
		}
	}
	return nil
}

func runRevokeKey(ctx context.Context, deps *Dependencies, keyID int64) error {
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
	return client.revokeKey(ctx, cfg, privateKey, keyID)
}
