package commands

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"

	"labkit.local/apps/cli/internal/buildinfo"
	"labkit.local/apps/cli/internal/config"
)

const (
	updateRepoSlug          = "RUCICS/LabKit"
	updateReminderInterval  = 24 * time.Hour
	updateReminderTimeout   = 2 * time.Second
	updateExplicitTimeout   = 30 * time.Second
	updateChecksumsFileName = "checksums.txt"
)

type updateOptions struct {
	Check      bool
	Prerelease bool
	Yes        bool
}

func NewUpdateCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	opts := updateOptions{}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update this CLI from GitHub Releases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), updateExplicitTimeout)
			defer cancel()

			updater, err := newGitHubUpdater(opts.Prerelease)
			if err != nil {
				return err
			}

			current := currentSemverForUpdate()
			release, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(updateRepoSlug))
			if err != nil {
				return friendlyUpdateError(err)
			}
			if !found {
				return fmt.Errorf("no releases found for %s (%s/%s)", updateRepoSlug, deps.BinaryName, platformLabel())
			}

			if release.LessOrEqual(current) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is up to date (%s)\n", deps.BinaryName, prettyVersion(current))
				return nil
			}

			if opts.Check {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"Update available: %s -> %s\n",
					prettyVersion(current),
					prettyVersion(release.Version()),
				)
				fmt.Fprintf(cmd.OutOrStdout(), "Run: %s update\n", deps.BinaryName)
				return nil
			}

			if !opts.Yes && deps.IsTTY != nil && deps.IsTTY() {
				ok, err := confirmUpdate(cmd, deps, current, release.Version())
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
					return nil
				}
			}

			exe, err := selfupdate.ExecutablePath()
			if err != nil {
				return fmt.Errorf("could not locate executable: %w", err)
			}

			if err := updater.UpdateTo(ctx, release, exe); err != nil {
				return friendlyUpdateError(err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated successfully to %s\n", prettyVersion(release.Version()))
			fmt.Fprintf(cmd.OutOrStdout(), "Run `%s -v` to verify.\n", deps.BinaryName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.Check, "check", false, "Only check whether a newer version is available")
	cmd.Flags().BoolVar(&opts.Prerelease, "prerelease", false, "Allow updating to pre-release versions")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "Do not prompt for confirmation")
	return cmd
}

func maybeRemindUpdate(cmd *cobra.Command, deps *Dependencies) {
	// Avoid reminder in `update` itself to prevent noisy recursion.
	if cmd != nil && cmd.Name() == "update" {
		return
	}
	if deps == nil {
		return
	}
	if strings.TrimSpace(os.Getenv("LABKIT_DISABLE_UPDATE_REMINDER")) != "" {
		return
	}

	now := time.Now
	if deps.Now != nil {
		now = deps.Now
	}

	state, err := config.ReadUpdateState(deps.ConfigDir)
	if err == nil && state.LastCheckAt != nil && now().Sub(*state.LastCheckAt) < updateReminderInterval {
		return
	}

	// Best-effort: write the timestamp even if the check fails, to avoid spamming slow networks.
	ts := now()
	_ = config.WriteUpdateState(deps.ConfigDir, config.UpdateState{LastCheckAt: &ts})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), updateReminderTimeout)
		defer cancel()

		updater, err := newGitHubUpdater(false)
		if err != nil {
			return
		}

		current := currentSemverForUpdate()
		release, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(updateRepoSlug))
		if err != nil || !found {
			return
		}
		if release.GreaterThan(current) {
			fmt.Fprintf(deps.Err, "Update available: %s -> %s. Run: %s update\n", prettyVersion(current), prettyVersion(release.Version()), deps.BinaryName)
		}
	}()
}

func confirmUpdate(cmd *cobra.Command, deps *Dependencies, from, to string) (bool, error) {
	out := cmd.OutOrStdout()
	in := cmd.InOrStdin()
	fmt.Fprintf(out, "Update %s from %s to %s? [y/N]: ", deps.BinaryName, prettyVersion(from), prettyVersion(to))
	var line string
	if _, err := fmt.Fscanln(in, &line); err != nil {
		// If user just presses enter, Fscanln returns an error; treat as "no".
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func currentSemverForUpdate() string {
	v := strings.TrimSpace(buildinfo.NormalizedVersion())
	if v == "" || v == "dev" || v == "unknown" {
		return "0.0.0"
	}
	return v
}

func prettyVersion(v string) string {
	x := strings.TrimSpace(v)
	if x == "" {
		return "dev"
	}
	if strings.HasPrefix(x, "v") {
		return x
	}
	// go-selfupdate Release.Version() typically yields semver without leading "v",
	// while our tags use "vX.Y.Z". Keep UX consistent.
	if looksLikeSemver(x) {
		return "v" + x
	}
	return x
}

func looksLikeSemver(v string) bool {
	// Minimal check: three dot-separated numeric parts.
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func platformLabel() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func newGitHubUpdater(prerelease bool) (*selfupdate.Updater, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, err
	}
	return selfupdate.NewUpdater(selfupdate.Config{
		Source:     source,
		Prerelease: prerelease,
		Draft:      false,
		Validator:  &selfupdate.ChecksumValidator{UniqueFilename: updateChecksumsFileName},
	})
}

func friendlyUpdateError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "rate limit exceeded") || strings.Contains(msg, "API rate limit exceeded") {
		return fmt.Errorf("%w\n\nTip: set GITHUB_TOKEN to increase GitHub API rate limits.", err)
	}
	return err
}

