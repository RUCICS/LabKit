package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"labkit.local/apps/worker/internal/runtime"
	"labkit.local/apps/worker/internal/service/evaluation"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/evaluator"
	"labkit.local/packages/go/jobs"
	"labkit.local/packages/go/labkit"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
)

type queryStore interface {
	GetSubmission(context.Context, uuid.UUID) (sqlc.Submissions, error)
	GetLab(context.Context, string) (sqlc.Labs, error)
}

type jobAcknowledger interface {
	Acknowledge(context.Context, uuid.UUID, string, string, string) error
}

type resultPersister interface {
	MarkRunning(context.Context, uuid.UUID, time.Time) error
	Persist(context.Context, evaluation.PersistInput) error
}

type evaluationRunner interface {
	Evaluate(context.Context, runtime.RunRequest) (runtime.RunResult, error)
}

type imageInspector interface {
	Resolve(context.Context, string) (string, error)
}

type runtimeHandler struct {
	workerID       string
	queue          jobAcknowledger
	queries        queryStore
	persister      resultPersister
	artifactRoot   string
	now            func() time.Time
	runner         evaluationRunner
	imageInspector imageInspector
	workspaces     runtime.TempDirManager
}

func (h *runtimeHandler) Handle(ctx context.Context, job *sqlc.EvaluationJobs) error {
	if h == nil || h.queue == nil || h.queries == nil || h.persister == nil || h.runner == nil {
		return fmt.Errorf("runtime handler is not configured")
	}
	if job == nil {
		return nil
	}

	startedAt := h.timestamp()
	submission, parsed, err := h.loadSubmission(ctx, job.SubmissionID)
	if err != nil {
		return err
	}
	if err := h.persister.MarkRunning(ctx, submission.ID, startedAt); err != nil {
		return err
	}

	imageDigest, _ := h.resolveImageDigest(ctx, parsed.Eval.Image)
	result, err := h.evaluate(ctx, submission, parsed)
	if err != nil {
		result = evaluator.Result{
			Verdict: evaluator.VerdictError,
			Message: err.Error(),
		}
	}

	if err := h.persister.Persist(ctx, evaluation.PersistInput{
		Manifest:    parsed,
		Submission:  submission,
		Result:      result,
		ImageDigest: imageDigest,
		StartedAt:   startedAt,
		FinishedAt:  h.timestamp(),
	}); err != nil {
		return err
	}

	status := jobs.StatusDone
	lastError := ""
	if result.Verdict == evaluator.VerdictError {
		status = jobs.StatusError
		lastError = result.Message
	}
	return h.queue.Acknowledge(ctx, job.ID, h.workerID, status, lastError)
}

func (h *runtimeHandler) loadSubmission(ctx context.Context, submissionID [16]byte) (sqlc.Submissions, *manifest.Manifest, error) {
	submission, err := h.queries.GetSubmission(ctx, submissionID)
	if err != nil {
		return sqlc.Submissions{}, nil, labkit.WrapSystemError("load submission", err)
	}
	labRow, err := h.queries.GetLab(ctx, submission.LabID)
	if err != nil {
		return sqlc.Submissions{}, nil, labkit.WrapSystemError("load lab manifest", err)
	}

	var parsed manifest.Manifest
	if err := json.Unmarshal(labRow.Manifest, &parsed); err != nil {
		return sqlc.Submissions{}, nil, labkit.WrapSystemError("decode stored manifest", err)
	}
	if err := parsed.Validate(); err != nil {
		return sqlc.Submissions{}, nil, labkit.WrapSystemError("validate stored manifest", err)
	}
	return submission, &parsed, nil
}

func (h *runtimeHandler) evaluate(ctx context.Context, submission sqlc.Submissions, parsed *manifest.Manifest) (evaluator.Result, error) {
	archivePath := filepath.Join(h.artifactRoot, filepath.FromSlash(submission.ArtifactKey))
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		return evaluator.Result{}, labkit.WrapSystemError("read submission artifact", err)
	}

	workspace, cleanup, err := h.workspacesOrDefault().CreateWorkspace()
	if err != nil {
		return evaluator.Result{}, labkit.WrapSystemError("create evaluation workspace", err)
	}
	defer func() {
		_ = cleanup()
	}()

	submissionDir, err := runtime.EnsureSubmissionDir(workspace)
	if err != nil {
		return evaluator.Result{}, labkit.WrapSystemError("create submission workspace", err)
	}
	if err := extractArchiveToDir(archive, submissionDir); err != nil {
		return evaluator.Result{}, labkit.WrapSystemError("extract submission artifact", err)
	}

	runResult, err := h.runner.Evaluate(ctx, runtime.RunRequest{
		Image:         parsed.Eval.Image,
		SubmissionDir: submissionDir,
		Timeout:       time.Duration(parsed.Eval.Timeout) * time.Second,
		Manifest:      parsed,
	})
	if err != nil {
		return evaluator.Result{}, err
	}
	return runResult.Result, nil
}

func (h *runtimeHandler) resolveImageDigest(ctx context.Context, image string) (string, error) {
	if h == nil || h.imageInspector == nil || strings.TrimSpace(image) == "" {
		return "", nil
	}
	return h.imageInspector.Resolve(ctx, image)
}

func (h *runtimeHandler) workspacesOrDefault() runtime.TempDirManager {
	if h == nil || h.workspaces == (runtime.TempDirManager{}) {
		return runtime.TempDirManager{}
	}
	return h.workspaces
}

func (h *runtimeHandler) timestamp() time.Time {
	if h != nil && h.now != nil {
		return h.now().UTC()
	}
	return time.Now().UTC()
}

func extractArchiveToDir(archive []byte, targetDir string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("open gzip archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive entry: %w", err)
		}
		if header == nil || header.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(header.Name)
		if name == "." || path.IsAbs(name) || strings.Contains(name, "/") || strings.HasPrefix(name, "..") {
			return fmt.Errorf("invalid archive entry %q", header.Name)
		}
		targetPath := filepath.Join(targetDir, name)
		file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return fmt.Errorf("create extracted file %q: %w", name, err)
		}
		if _, err := io.Copy(file, tr); err != nil {
			file.Close()
			return fmt.Errorf("write extracted file %q: %w", name, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close extracted file %q: %w", name, err)
		}
	}
}

type dockerImageInspector struct{}

func (dockerImageInspector) Resolve(ctx context.Context, image string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{.Id}}", image).CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
