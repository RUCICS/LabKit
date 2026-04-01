package runtime

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"labkit.local/packages/go/labkit"
	"labkit.local/packages/go/manifest"
)

func TestDockerRunnerFollowsDocumentedLifecycle(t *testing.T) {
	t.Parallel()

	submissionDir := t.TempDir()
	writeSubmissionFile(t, submissionDir, "zeta.txt", "z")
	writeSubmissionFile(t, submissionDir, "alpha.txt", "a")

	workspaceRoot := t.TempDir()
		exec := &recordingExecutor{
			steps: []execStep{
				{stdout: []byte("container-123\n")},
				{},
				{stdout: []byte("{\"verdict\":\"build_failed\"}\n")},
				{},
			},
		}
	runner := DockerRunner{
		exec:       exec,
		Memory:     "512m",
		CPUs:       "2",
		Workspaces: TempDirManager{Root: workspaceRoot},
	}

	_, err := runner.Evaluate(context.Background(), RunRequest{
		Image:         "registry.example.edu/lab-eval:2026sp",
		SubmissionDir: submissionDir,
		Timeout:       5 * time.Second,
		Manifest:      manifestWithFiles("alpha.txt", "zeta.txt"),
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

		if len(exec.calls) != 4 {
			t.Fatalf("command count = %d, want 4", len(exec.calls))
		}

	assertCall(t, exec.calls[0], "docker", []string{
		"create",
		"--network=none",
		"--memory", "512m",
		"--cpus", "2",
		"registry.example.edu/lab-eval:2026sp",
	})

		assertCopyCall(t, exec.calls[1], workspaceRoot, "submission", "container-123:/")
		assertCall(t, exec.calls[2], "docker", []string{"start", "-a", "container-123"})
		assertCall(t, exec.calls[3], "docker", []string{"rm", "-f", "container-123"})
}

func TestDockerRunnerMapsTimeoutToEvaluatorError(t *testing.T) {
	t.Parallel()

	submissionDir := t.TempDir()
	writeSubmissionFile(t, submissionDir, "main.c", "int main(void) { return 0; }")

	exec := &recordingExecutor{
		steps: []execStep{
			{stdout: []byte("container-123\n")},
			{},
			{waitForContext: true},
			{},
		},
	}
	runner := DockerRunner{
		exec:       exec,
		Workspaces: TempDirManager{Root: t.TempDir()},
	}

	_, err := runner.Evaluate(context.Background(), RunRequest{
		Image:         "registry.example.edu/lab-eval:2026sp",
		SubmissionDir: submissionDir,
		Timeout:       10 * time.Millisecond,
		Manifest:      manifestWithFiles("main.c"),
	})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want timeout error")
	}
	if got := labkit.ClassifyError(err); got != labkit.ErrorClassEvaluator {
		t.Fatalf("ClassifyError() = %q, want %q", got, labkit.ErrorClassEvaluator)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %q, want timeout message", err)
	}
	if len(exec.calls) != 4 {
		t.Fatalf("command count = %d, want 4", len(exec.calls))
	}
	assertCall(t, exec.calls[3], "docker", []string{"rm", "-f", "container-123"})
}

func TestDockerRunnerClassifiesStartCancellationAsSystemInterruption(t *testing.T) {
	t.Parallel()

	submissionDir := t.TempDir()
	writeSubmissionFile(t, submissionDir, "main.c", "int main(void) { return 0; }")

	ctx, cancel := context.WithCancel(context.Background())
	exec := &recordingExecutor{
		steps: []execStep{
			{stdout: []byte("container-123\n")},
			{},
			{waitForContext: true},
			{},
		},
	}
	runner := DockerRunner{
		exec:       exec,
		Workspaces: TempDirManager{Root: t.TempDir()},
	}

	done := make(chan error, 1)
	go func() {
		_, err := runner.Evaluate(ctx, RunRequest{
			Image:         "registry.example.edu/lab-eval:2026sp",
			SubmissionDir: submissionDir,
			Timeout:       time.Second,
			Manifest:      manifestWithFiles("main.c"),
		})
		done <- err
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	if err == nil {
		t.Fatal("Evaluate() error = nil, want interruption error")
	}
	if got := labkit.ClassifyError(err); got != labkit.ErrorClassSystem {
		t.Fatalf("ClassifyError() = %q, want %q", got, labkit.ErrorClassSystem)
	}
	if !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("error = %q, want interruption message", err)
	}
	assertCall(t, exec.calls[3], "docker", []string{"rm", "-f", "container-123"})
}

func TestDockerRunnerDoesNotClassifyPreStartTimeoutAsEvaluatorTimeout(t *testing.T) {
	t.Parallel()

	submissionDir := t.TempDir()
	writeSubmissionFile(t, submissionDir, "main.c", "int main(void) { return 0; }")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	exec := &recordingExecutor{
		steps: []execStep{
			{waitForContext: true},
		},
	}
	runner := DockerRunner{
		exec:       exec,
		Workspaces: TempDirManager{Root: t.TempDir()},
	}

	_, err := runner.Evaluate(ctx, RunRequest{
		Image:         "registry.example.edu/lab-eval:2026sp",
		SubmissionDir: submissionDir,
		Timeout:       time.Second,
		Manifest:      manifestWithFiles("main.c"),
	})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want pre-start failure")
	}
	if got := labkit.ClassifyError(err); got != labkit.ErrorClassSystem {
		t.Fatalf("ClassifyError() = %q, want %q", got, labkit.ErrorClassSystem)
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %q, did not want evaluator timeout message", err)
	}
}

func TestDockerRunnerMapsNonZeroExitToEvaluatorError(t *testing.T) {
	t.Parallel()

	submissionDir := t.TempDir()
	writeSubmissionFile(t, submissionDir, "main.c", "int main(void) { return 0; }")

	exec := &recordingExecutor{
		steps: []execStep{
			{stdout: []byte("container-123\n")},
			{},
			{stdout: []byte("compile failed\n"), err: exitError(t)},
			{},
		},
	}
	runner := DockerRunner{
		exec:       exec,
		Workspaces: TempDirManager{Root: t.TempDir()},
	}

	_, err := runner.Evaluate(context.Background(), RunRequest{
		Image:         "registry.example.edu/lab-eval:2026sp",
		SubmissionDir: submissionDir,
		Timeout:       time.Second,
		Manifest:      manifestWithFiles("main.c"),
	})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want exit error")
	}
	if got := labkit.ClassifyError(err); got != labkit.ErrorClassEvaluator {
		t.Fatalf("ClassifyError() = %q, want %q", got, labkit.ErrorClassEvaluator)
	}
	if !strings.Contains(err.Error(), "exited abnormally") {
		t.Fatalf("error = %q, want abnormal exit message", err)
	}
	assertCall(t, exec.calls[3], "docker", []string{"rm", "-f", "container-123"})
}

func TestDockerRunnerParsesLastStdoutLineOnly(t *testing.T) {
	t.Parallel()

	submissionDir := t.TempDir()
	writeSubmissionFile(t, submissionDir, "main.c", "int main(void) { return 0; }")

	exec := &recordingExecutor{
		steps: []execStep{
			{stdout: []byte("container-123\n")},
			{},
			{
				stdout: []byte("building...\r\nstill running...\r\n{\"verdict\":\"scored\",\"scores\":{\"score\":1.5}}\r\n"),
				stderr: []byte("stderr noise that must be ignored\n{\"verdict\":\"rejected\"}\n"),
			},
			{},
		},
	}
	runner := DockerRunner{
		exec:       exec,
		Workspaces: TempDirManager{Root: t.TempDir()},
	}

	got, err := runner.Evaluate(context.Background(), RunRequest{
		Image:         "registry.example.edu/lab-eval:2026sp",
		SubmissionDir: submissionDir,
		Timeout:       time.Second,
		Manifest:      manifestWithFiles("main.c"),
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if got.Result.Verdict != "scored" {
		t.Fatalf("verdict = %q, want scored", got.Result.Verdict)
	}
	if score := got.Result.Scores["score"]; score != 1.5 {
		t.Fatalf("score = %v, want 1.5", score)
	}
	if strings.Contains(string(got.Stdout), "stderr noise") {
		t.Fatalf("stdout captured stderr content: %q", got.Stdout)
	}
}

func TestDockerRunnerRejectsUnexpectedExtraSubmissionFile(t *testing.T) {
	t.Parallel()

	submissionDir := t.TempDir()
	writeSubmissionFile(t, submissionDir, "main.c", "int main(void) { return 0; }")
	writeSubmissionFile(t, submissionDir, "notes.txt", "unexpected")

	runner := DockerRunner{
		exec:       &recordingExecutor{},
		Workspaces: TempDirManager{Root: t.TempDir()},
	}

	_, err := runner.Evaluate(context.Background(), RunRequest{
		Image:         "registry.example.edu/lab-eval:2026sp",
		SubmissionDir: submissionDir,
		Timeout:       time.Second,
		Manifest:      manifestWithFiles("main.c"),
	})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want extra-file rejection")
	}
	if got := labkit.ClassifyError(err); got != labkit.ErrorClassSystem {
		t.Fatalf("ClassifyError() = %q, want %q", got, labkit.ErrorClassSystem)
	}
	if !strings.Contains(err.Error(), "unexpected submission file") {
		t.Fatalf("error = %q, want unexpected file message", err)
	}
}

func TestDockerRunnerRejectsSymlinkSubmissionFile(t *testing.T) {
	t.Parallel()

	submissionDir := t.TempDir()
	target := filepath.Join(submissionDir, "real.c")
	if err := os.WriteFile(target, []byte("int main(void) { return 0; }"), 0o644); err != nil {
		t.Fatalf("WriteFile(real.c) error = %v", err)
	}
	link := filepath.Join(submissionDir, "main.c")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	runner := DockerRunner{
		exec:       &recordingExecutor{},
		Workspaces: TempDirManager{Root: t.TempDir()},
	}

	_, err := runner.Evaluate(context.Background(), RunRequest{
		Image:         "registry.example.edu/lab-eval:2026sp",
		SubmissionDir: submissionDir,
		Timeout:       time.Second,
		Manifest:      manifestWithFiles("main.c"),
	})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want symlink rejection")
	}
	if got := labkit.ClassifyError(err); got != labkit.ErrorClassSystem {
		t.Fatalf("ClassifyError() = %q, want %q", got, labkit.ErrorClassSystem)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %q, want symlink rejection message", err)
	}
}

type execStep struct {
	stdout         []byte
	stderr         []byte
	err            error
	waitForContext bool
}

type execCall struct {
	name string
	args []string
}

type recordingExecutor struct {
	steps []execStep
	calls []execCall
}

func (e *recordingExecutor) Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	e.calls = append(e.calls, execCall{
		name: name,
		args: append([]string(nil), args...),
	})

	step := execStep{}
	if len(e.steps) >= len(e.calls) {
		step = e.steps[len(e.calls)-1]
	}

	if step.waitForContext {
		<-ctx.Done()
		return ctx.Err()
	}
	if len(step.stdout) > 0 {
		if _, err := stdout.Write(step.stdout); err != nil {
			return err
		}
	}
	if len(step.stderr) > 0 {
		if _, err := stderr.Write(step.stderr); err != nil {
			return err
		}
	}
	return step.err
}

func assertCall(t *testing.T, call execCall, wantName string, wantArgs []string) {
	t.Helper()

	if call.name != wantName {
		t.Fatalf("command = %q, want %q", call.name, wantName)
	}
	if len(call.args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d (%v)", len(call.args), len(wantArgs), call.args)
	}
	for i := range wantArgs {
		if call.args[i] != wantArgs[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, call.args[i], wantArgs[i])
		}
	}
}

func assertCopyCall(t *testing.T, call execCall, workspaceRoot, wantBase, target string) {
	t.Helper()

	if call.name != "docker" {
		t.Fatalf("command = %q, want docker", call.name)
	}
	if len(call.args) != 3 {
		t.Fatalf("copy args len = %d, want 3", len(call.args))
	}
	if call.args[0] != "cp" {
		t.Fatalf("copy subcommand = %q, want cp", call.args[0])
	}
	if !strings.HasPrefix(call.args[1], workspaceRoot) {
		t.Fatalf("copy source = %q, want under workspace root %q", call.args[1], workspaceRoot)
	}
	if filepath.Base(call.args[1]) != wantBase {
		t.Fatalf("copy source base = %q, want %q", filepath.Base(call.args[1]), wantBase)
	}
	if call.args[2] != target {
		t.Fatalf("copy target = %q, want %q", call.args[2], target)
	}
}

func exitError(t *testing.T) error {
	t.Helper()

	cmd := exec.Command("sh", "-c", "exit 17")
	err := cmd.Run()
	if err == nil {
		t.Fatal("Run() error = nil, want exit error")
	}
	return err
}

func manifestWithFiles(files ...string) *manifest.Manifest {
	return &manifest.Manifest{
		Submit: manifest.SubmitSection{
			Files: append([]string(nil), files...),
		},
		Metrics: []manifest.MetricSection{
			{ID: "score", Sort: manifest.MetricSortDesc},
		},
	}
}

func writeSubmissionFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
