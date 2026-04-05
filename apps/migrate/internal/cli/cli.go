package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
)

type Engine interface {
	Up(context.Context) error
	Baseline(context.Context, uint) error
	Version(context.Context) (uint, bool, error)
}

func Execute(ctx context.Context, stdout io.Writer, stderr io.Writer, engine Engine, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: labkit-migrate <up|baseline|version>")
	}

	switch args[0] {
	case "up":
		if len(args) != 1 {
			return fmt.Errorf("usage: labkit-migrate up")
		}
		if err := engine.Up(ctx); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout, "migrations are up to date")
		return nil
	case "baseline":
		fs := flag.NewFlagSet("baseline", flag.ContinueOnError)
		fs.SetOutput(stderr)
		var version uint
		fs.UintVar(&version, "version", 0, "baseline migration version")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if version == 0 {
			return fmt.Errorf("--version is required")
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("usage: labkit-migrate baseline --version <n>")
		}
		if err := engine.Baseline(ctx, version); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "baseline recorded at version %d\n", version)
		return nil
	case "version":
		if len(args) != 1 {
			return fmt.Errorf("usage: labkit-migrate version")
		}
		version, dirty, err := engine.Version(ctx)
		if err != nil {
			return err
		}
		if version == 0 {
			_, _ = fmt.Fprintln(stdout, "version none")
			return nil
		}
		if dirty {
			_, _ = fmt.Fprintf(stdout, "version %d (dirty)\n", version)
			return nil
		}
		_, _ = fmt.Fprintf(stdout, "version %d\n", version)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
