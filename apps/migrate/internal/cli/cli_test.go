package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestExecuteUpInvokesRunner(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{}
	var stdout bytes.Buffer

	if err := Execute(context.Background(), &stdout, &stdout, engine, []string{"up"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !engine.upCalled {
		t.Fatal("up was not called")
	}
	if got, want := strings.TrimSpace(stdout.String()), "migrations are up to date"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestExecuteBaselineRequiresVersionFlag(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{}
	var stderr bytes.Buffer

	err := Execute(context.Background(), &bytes.Buffer{}, &stderr, engine, []string{"baseline"})
	if err == nil {
		t.Fatal("Execute() error = nil, want missing flag error")
	}
	if !strings.Contains(err.Error(), "--version") {
		t.Fatalf("error = %v, want --version guidance", err)
	}
	if engine.baselineCalled {
		t.Fatal("baseline should not be called")
	}
}

func TestExecuteBaselineInvokesRunner(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{}
	var stdout bytes.Buffer

	if err := Execute(context.Background(), &stdout, &stdout, engine, []string{"baseline", "--version", "7"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !engine.baselineCalled {
		t.Fatal("baseline was not called")
	}
	if got, want := engine.baselineVersion, uint(7); got != want {
		t.Fatalf("baseline version = %d, want %d", got, want)
	}
	if got, want := strings.TrimSpace(stdout.String()), "baseline recorded at version 7"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestExecuteVersionPrintsDirtyState(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{
		version: 7,
		dirty:   true,
	}
	var stdout bytes.Buffer

	if err := Execute(context.Background(), &stdout, &stdout, engine, []string{"version"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !engine.versionCalled {
		t.Fatal("version was not called")
	}
	if got, want := strings.TrimSpace(stdout.String()), "version 7 (dirty)"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestExecuteVersionPrintsNoneWhenUninitialized(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{}
	var stdout bytes.Buffer

	if err := Execute(context.Background(), &stdout, &stdout, engine, []string{"version"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := strings.TrimSpace(stdout.String()), "version none"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

type fakeEngine struct {
	upCalled        bool
	baselineCalled  bool
	versionCalled   bool
	baselineVersion uint
	version         uint
	dirty           bool
}

func (f *fakeEngine) Up(context.Context) error {
	f.upCalled = true
	return nil
}

func (f *fakeEngine) Baseline(_ context.Context, version uint) error {
	f.baselineCalled = true
	f.baselineVersion = version
	return nil
}

func (f *fakeEngine) Version(context.Context) (uint, bool, error) {
	f.versionCalled = true
	return f.version, f.dirty, nil
}
