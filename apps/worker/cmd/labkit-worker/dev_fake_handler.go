package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"labkit.local/apps/worker/internal/service/evaluation"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/evaluator"
	"labkit.local/packages/go/jobs"
	"labkit.local/packages/go/manifest"
)

type devFakeHandler struct {
	workerID     string
	queue        *jobs.Queue
	queries      *sqlc.Queries
	persister    *evaluation.Service
	artifactRoot string
	now          func() time.Time
}

func (h *devFakeHandler) Handle(ctx context.Context, job *sqlc.EvaluationJobs) error {
	if h == nil || h.queue == nil || h.queries == nil || h.persister == nil {
		return fmt.Errorf("dev fake handler is not configured")
	}
	if job == nil {
		return nil
	}

	startedAt := h.timestamp()
	submission, err := h.queries.GetSubmission(ctx, job.SubmissionID)
	if err != nil {
		return err
	}
	if err := h.persister.MarkRunning(ctx, submission.ID, startedAt); err != nil {
		return err
	}
	labRow, err := h.queries.GetLab(ctx, submission.LabID)
	if err != nil {
		return err
	}

	var parsed manifest.Manifest
	if err := json.Unmarshal(labRow.Manifest, &parsed); err != nil {
		return err
	}

	archive, err := os.ReadFile(filepath.Join(h.artifactRoot, filepath.FromSlash(submission.ArtifactKey)))
	if err != nil {
		return err
	}

	result, evalErr := evaluation.EvaluateDevArtifact(ctx, &parsed, archive)
	if evalErr != nil {
		result = evaluator.Result{
			Verdict: evaluator.VerdictError,
			Message: evalErr.Error(),
		}
	}

	if err := h.persister.Persist(ctx, evaluation.PersistInput{
		Manifest:   &parsed,
		Submission: submission,
		Result:     result,
		StartedAt:  startedAt,
		FinishedAt: h.timestamp(),
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

func (h *devFakeHandler) timestamp() time.Time {
	if h != nil && h.now != nil {
		return h.now().UTC()
	}
	return time.Now().UTC()
}
