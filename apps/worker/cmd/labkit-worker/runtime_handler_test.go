package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"labkit.local/apps/worker/internal/runtime"
	"labkit.local/apps/worker/internal/service/evaluation"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/evaluator"
	"labkit.local/packages/go/jobs"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRuntimeHandlerPersistsScoredResultAndAcknowledgesDone(t *testing.T) {
	tctx := context.Background()
	artifactRoot := t.TempDir()
	submissionID := uuid.MustParse("11111111-1111-7111-8111-111111111111")
	jobID := uuid.MustParse("22222222-2222-7222-8222-222222222222")

	if err := os.MkdirAll(filepath.Join(artifactRoot, "lab/1"), 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "lab/1/artifact.tar.gz"), mustArchive(t, map[string]string{
		"answer.txt": "hello world\n",
	}), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	manifestData, err := json.Marshal(testRuntimeManifest())
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	queries := &fakeRuntimeQueries{
		submission: sqlc.Submissions{
			ID:          submissionID,
			UserID:      1,
			LabID:       "runtime-lab",
			ArtifactKey: "lab/1/artifact.tar.gz",
		},
		lab: sqlc.Labs{
			ID:       "runtime-lab",
			Name:     "Runtime Lab",
			Manifest: manifestData,
		},
	}
	queue := &fakeRuntimeQueue{}
	persister := &fakeRuntimePersister{}
	runner := &fakeRuntimeRunner{
		result: runtime.RunResult{
			Result: evaluator.Result{
				Verdict: evaluator.VerdictScored,
				Scores: map[string]float64{
					"score": 97.5,
				},
				Message: "ok",
			},
		},
	}

	handler := &runtimeHandler{
		workerID:     "worker-test",
		queue:        queue,
		queries:      queries,
		persister:    persister,
		artifactRoot: artifactRoot,
		now: func() time.Time {
			return time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
		},
		runner:         runner,
		imageInspector: fakeImageInspector{digest: "sha256:test"},
	}

	if err := handler.Handle(tctx, &sqlc.EvaluationJobs{ID: jobID, SubmissionID: submissionID}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if queue.status != jobs.StatusDone {
		t.Fatalf("ack status = %q, want %q", queue.status, jobs.StatusDone)
	}
	if queue.lastError != "" {
		t.Fatalf("ack last error = %q, want empty", queue.lastError)
	}
	if persister.input.ImageDigest != "sha256:test" {
		t.Fatalf("image digest = %q, want %q", persister.input.ImageDigest, "sha256:test")
	}
	if got := persister.input.Result.Verdict; got != evaluator.VerdictScored {
		t.Fatalf("verdict = %q, want %q", got, evaluator.VerdictScored)
	}
	if runner.req.Image != "registry.example.edu/runtime-eval:latest" {
		t.Fatalf("runner image = %q", runner.req.Image)
	}
	if string(runner.answerContent) != "hello world\n" {
		t.Fatalf("submission content = %q", string(runner.answerContent))
	}
}

func TestRuntimeHandlerMapsRunnerErrorToSubmissionErrorAndAcknowledgesError(t *testing.T) {
	tctx := context.Background()
	artifactRoot := t.TempDir()
	submissionID := uuid.MustParse("33333333-3333-7333-8333-333333333333")
	jobID := uuid.MustParse("44444444-4444-7444-8444-444444444444")

	if err := os.MkdirAll(filepath.Join(artifactRoot, "lab/1"), 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "lab/1/artifact.tar.gz"), mustArchive(t, map[string]string{
		"answer.txt": "boom\n",
	}), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	manifestData, err := json.Marshal(testRuntimeManifest())
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	handler := &runtimeHandler{
		workerID: "worker-test",
		queue:    &fakeRuntimeQueue{},
		queries: &fakeRuntimeQueries{
			submission: sqlc.Submissions{
				ID:          submissionID,
				UserID:      1,
				LabID:       "runtime-lab",
				ArtifactKey: "lab/1/artifact.tar.gz",
			},
			lab: sqlc.Labs{
				ID:       "runtime-lab",
				Name:     "Runtime Lab",
				Manifest: manifestData,
			},
		},
		persister:    &fakeRuntimePersister{},
		artifactRoot: artifactRoot,
		now: func() time.Time {
			return time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
		},
		runner:         &fakeRuntimeRunner{err: errors.New("docker create failed")},
		imageInspector: fakeImageInspector{},
	}

	queue := handler.queue.(*fakeRuntimeQueue)
	persister := handler.persister.(*fakeRuntimePersister)

	if err := handler.Handle(tctx, &sqlc.EvaluationJobs{ID: jobID, SubmissionID: submissionID}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if queue.status != jobs.StatusError {
		t.Fatalf("ack status = %q, want %q", queue.status, jobs.StatusError)
	}
	if got := persister.input.Result.Verdict; got != evaluator.VerdictError {
		t.Fatalf("verdict = %q, want %q", got, evaluator.VerdictError)
	}
	if persister.input.Result.Message == "" {
		t.Fatal("error verdict message is empty")
	}
}

func testRuntimeManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Lab: manifest.LabSection{
			ID:   "runtime-lab",
			Name: "Runtime Lab",
		},
		Submit: manifest.SubmitSection{
			Files:   []string{"answer.txt"},
			MaxSize: "1MB",
		},
		Eval: manifest.EvalSection{
			Image:   "registry.example.edu/runtime-eval:latest",
			Timeout: 30,
		},
		Quota: manifest.QuotaSection{
			Daily: 3,
		},
		Metrics: []manifest.MetricSection{
			{ID: "score", Name: "Score", Sort: manifest.MetricSortDesc},
		},
		Board: manifest.BoardSection{
			RankBy: "score",
		},
		Schedule: manifest.ScheduleSection{
			Open:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Close: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
}

func mustArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

type fakeRuntimeQueries struct {
	submission sqlc.Submissions
	lab        sqlc.Labs
}

func (q *fakeRuntimeQueries) GetSubmission(context.Context, uuid.UUID) (sqlc.Submissions, error) {
	return q.submission, nil
}

func (q *fakeRuntimeQueries) GetLab(context.Context, string) (sqlc.Labs, error) {
	return q.lab, nil
}

type fakeRuntimeQueue struct {
	status    string
	lastError string
}

func (q *fakeRuntimeQueue) Acknowledge(_ context.Context, _ uuid.UUID, _ string, status, lastError string) error {
	q.status = status
	q.lastError = lastError
	return nil
}

type fakeRuntimePersister struct {
	input evaluation.PersistInput
}

func (p *fakeRuntimePersister) Persist(_ context.Context, input evaluation.PersistInput) error {
	p.input = input
	return nil
}

type fakeRuntimeRunner struct {
	req           runtime.RunRequest
	result        runtime.RunResult
	err           error
	answerContent []byte
}

func (r *fakeRuntimeRunner) Evaluate(_ context.Context, req runtime.RunRequest) (runtime.RunResult, error) {
	r.req = req
	if data, err := os.ReadFile(filepath.Join(req.SubmissionDir, "answer.txt")); err == nil {
		r.answerContent = append([]byte(nil), data...)
	}
	if r.err != nil {
		return runtime.RunResult{}, r.err
	}
	return r.result, nil
}

type fakeImageInspector struct {
	digest string
	err    error
}

func (i fakeImageInspector) Resolve(context.Context, string) (string, error) {
	return i.digest, i.err
}

var _ queryStore = (*fakeRuntimeQueries)(nil)
var _ jobAcknowledger = (*fakeRuntimeQueue)(nil)
var _ resultPersister = (*fakeRuntimePersister)(nil)
var _ evaluationRunner = (*fakeRuntimeRunner)(nil)
var _ imageInspector = fakeImageInspector{}
var _ = pgtype.Text{}
