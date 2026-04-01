package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"labkit.local/packages/go/evaluator"
	"labkit.local/packages/go/labkit"
	"labkit.local/packages/go/manifest"
)

const (
	defaultEvalTimeout = 300 * time.Second
	defaultMemoryLimit = "512m"
	defaultCPULimit    = "2"
)

type commandExecutor interface {
	Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error
}

type osCommandExecutor struct{}

func (osCommandExecutor) Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

type RunRequest struct {
	Image         string
	SubmissionDir string
	Timeout       time.Duration
	Manifest      *manifest.Manifest
}

type RunResult struct {
	Result evaluator.Result
	Stdout []byte
}

type DockerRunner struct {
	exec       commandExecutor
	Memory     string
	CPUs       string
	Workspaces TempDirManager
}

func (r DockerRunner) Evaluate(ctx context.Context, req RunRequest) (RunResult, error) {
	if req.Image == "" {
		return RunResult{}, fmt.Errorf("runtime: evaluator image is required")
	}
	if req.SubmissionDir == "" {
		return RunResult{}, fmt.Errorf("runtime: submission directory is required")
	}
	if req.Manifest == nil {
		return RunResult{}, fmt.Errorf("runtime: manifest is required")
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultEvalTimeout
	}

	execRunner := r.exec
	if execRunner == nil {
		execRunner = osCommandExecutor{}
	}

	stagedSubmissionDir, cleanup, err := r.workspaces().StageSubmissionDir(req.SubmissionDir, req.Manifest.Submit.Files)
	if err != nil {
		return RunResult{}, labkit.WrapSystemError("stage submission workspace", err)
	}
	defer func() {
		_ = cleanup()
	}()

	containerID, err := createContainer(ctx, execRunner, req.Image, r.Memory, r.CPUs)
	if err != nil {
		return RunResult{}, labkit.WrapSystemError("create evaluator container", err)
	}
	defer removeContainer(execRunner, containerID)

	if err := copySubmission(ctx, execRunner, containerID, stagedSubmissionDir); err != nil {
		return RunResult{}, labkit.WrapSystemError("copy submission into evaluator container", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stdout, err := startContainer(runCtx, execRunner, containerID)
	if errors.Is(runCtx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return RunResult{Stdout: stdout}, labkit.WrapSystemError("evaluator execution interrupted", context.Canceled)
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return RunResult{Stdout: stdout}, labkit.WrapEvaluatorError("evaluator timed out", context.DeadlineExceeded)
	}
	if err != nil {
		return RunResult{Stdout: stdout}, labkit.WrapEvaluatorError("evaluator exited abnormally", err)
	}

	result, err := evaluator.ParseResult(req.Manifest, stdout)
	if err != nil {
		return RunResult{Stdout: stdout}, labkit.WrapEvaluatorError("parse evaluator output", err)
	}

	return RunResult{
		Result: result,
		Stdout: stdout,
	}, nil
}

func (r DockerRunner) workspaces() TempDirManager {
	if r.Workspaces == (TempDirManager{}) {
		return TempDirManager{}
	}
	return r.Workspaces
}

func createContainer(ctx context.Context, execRunner commandExecutor, image, memory, cpus string) (string, error) {
	var stdout bytes.Buffer
	if err := execRunner.Run(ctx, &stdout, io.Discard, "docker", createArgs(image, memory, cpus)...); err != nil {
		return "", err
	}

	containerID := strings.TrimSpace(stdout.String())
	if containerID == "" {
		return "", fmt.Errorf("docker create returned empty container id")
	}
	return containerID, nil
}

func copySubmission(ctx context.Context, execRunner commandExecutor, containerID, submissionDir string) error {
	source := filepath.Clean(submissionDir)
	target := containerID + ":/"
	return execRunner.Run(ctx, io.Discard, io.Discard, "docker", "cp", source, target)
}

func startContainer(ctx context.Context, execRunner commandExecutor, containerID string) ([]byte, error) {
	var stdout bytes.Buffer
	err := execRunner.Run(ctx, &stdout, io.Discard, "docker", "start", "-a", containerID)
	return stdout.Bytes(), err
}

func removeContainer(execRunner commandExecutor, containerID string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = execRunner.Run(cleanupCtx, io.Discard, io.Discard, "docker", "rm", "-f", containerID)
}

func createArgs(image, memory, cpus string) []string {
	return []string{
		"create",
		"--network=none",
		"--memory", valueOrDefault(memory, defaultMemoryLimit),
		"--cpus", valueOrDefault(cpus, defaultCPULimit),
		image,
	}
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
