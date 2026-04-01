package main

import (
	"fmt"
	"os"
	"strings"

	"labkit.local/apps/cli/internal/ui"
	"labkit.local/apps/cli/internal/commands"
	"labkit.local/apps/cli/internal/config"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		printCLIError(err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	return commands.NewRootCommand(&commands.Dependencies{
		ConfigDir: config.ResolveDir(),
	})
}

func printCLIError(err error) {
	theme := ui.DefaultTheme()
	msg := strings.TrimSpace(err.Error())
	prefix := theme.ErrorStyle.Render("✗") + " " + theme.TitleStyle.Render("Error")
	fmt.Fprintln(os.Stderr, prefix)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  "+theme.MutedStyle.Render(msg))
}
