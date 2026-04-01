package evaluation

import (
	"context"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/evaluator"
	"labkit.local/packages/go/labkit"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestPersistScoredSubmissionWritesScoresAndRefreshesLeaderboard(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo)

	submission := seededSubmission("11111111-1111-7111-8111-111111111111", 7, "sorting")
	result := evaluator.Result{
		Verdict: evaluator.VerdictScored,
		Scores: map[string]float64{
			"throughput": 1.82,
			"latency":    1.45,
		},
		Detail: &evaluator.Detail{
			Format:  evaluator.DetailFormatMarkdown,
			Content: "### Breakdown\n\nok",
		},
		Message: "compiled with warnings",
	}
	now := time.Date(2026, 3, 31, 13, 14, 15, 0, time.UTC)

	if err := svc.Persist(context.Background(), PersistInput{
		Manifest:    testManifest(t),
		Submission:  submission,
		Result:      result,
		ImageDigest: "sha256:abcdef123456",
		StartedAt:   now.Add(-2 * time.Minute),
		FinishedAt:  now,
	}); err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	if repo.beginCalls != 1 {
		t.Fatalf("begin calls = %d, want 1", repo.beginCalls)
	}
	if repo.lastTx == nil || repo.lastTx.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", repo.lastTx.commitCalls)
	}
	if repo.lastTx.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", repo.lastTx.updateCalls)
	}
	if repo.lastTx.scoreCalls != 2 {
		t.Fatalf("score calls = %d, want 2", repo.lastTx.scoreCalls)
	}
	if repo.lastTx.leaderboardCalls != 1 {
		t.Fatalf("leaderboard calls = %d, want 1", repo.lastTx.leaderboardCalls)
	}

	stored := repo.submissions[submission.ID]
	if stored.Status != "done" {
		t.Fatalf("submission status = %q, want %q", stored.Status, "done")
	}
	if got := stored.Verdict.String; got != string(evaluator.VerdictScored) {
		t.Fatalf("submission verdict = %q, want %q", got, evaluator.VerdictScored)
	}
	if got := stored.Message.String; got != result.Message {
		t.Fatalf("submission message = %q, want %q", got, result.Message)
	}
	if got := string(stored.Detail); got != `{"format":"markdown","content":"### Breakdown\n\nok"}` {
		t.Fatalf("submission detail = %s", got)
	}
	if got := stored.ImageDigest.String; got != "sha256:abcdef123456" {
		t.Fatalf("submission image digest = %q, want %q", got, "sha256:abcdef123456")
	}
	if !stored.StartedAt.Valid || !stored.StartedAt.Time.Equal(now.Add(-2*time.Minute)) {
		t.Fatalf("submission started_at = %v, want %v", stored.StartedAt, now.Add(-2*time.Minute))
	}
	if !stored.FinishedAt.Valid || !stored.FinishedAt.Time.Equal(now) {
		t.Fatalf("submission finished_at = %v, want %v", stored.FinishedAt, now)
	}

	scores := repo.scores[submission.ID]
	if got := len(scores); got != 2 {
		t.Fatalf("score count = %d, want 2", got)
	}
	if got := scores["throughput"]; got != 1.82 {
		t.Fatalf("throughput score = %v, want 1.82", got)
	}
	if got := scores["latency"]; got != 1.45 {
		t.Fatalf("latency score = %v, want 1.45", got)
	}

	lb := repo.leaderboard[leaderboardKey{userID: submission.UserID, labID: submission.LabID}]
	if lb.SubmissionID != submission.ID {
		t.Fatalf("leaderboard submission id = %v, want %v", lb.SubmissionID, submission.ID)
	}
}

func TestPersistBuildFailedPreservesNoScores(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo)

	submission := seededSubmission("22222222-2222-7222-8222-222222222222", 7, "sorting")
	result := evaluator.Result{
		Verdict: evaluator.VerdictBuildFailed,
		Message: "build failed",
	}

	if err := svc.Persist(context.Background(), PersistInput{
		Manifest:   testManifest(t),
		Submission: submission,
		Result:     result,
		FinishedAt: time.Date(2026, 3, 31, 13, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	if repo.beginCalls != 1 {
		t.Fatalf("begin calls = %d, want 1", repo.beginCalls)
	}
	if repo.lastTx.scoreCalls != 0 {
		t.Fatalf("score calls = %d, want 0", repo.lastTx.scoreCalls)
	}
	if repo.lastTx.leaderboardCalls != 0 {
		t.Fatalf("leaderboard calls = %d, want 0", repo.lastTx.leaderboardCalls)
	}
	if _, ok := repo.scores[submission.ID]; ok {
		t.Fatalf("scores were written for build_failed verdict")
	}
}

func TestPersistInvalidEvaluatorOutputReturnsEvaluatorErrorWithoutTransaction(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo)

	submission := seededSubmission("33333333-3333-7333-8333-333333333333", 7, "sorting")
	result := evaluator.Result{
		Verdict: evaluator.VerdictScored,
		Scores: map[string]float64{
			"latency": 1.45,
		},
	}

	err := svc.Persist(context.Background(), PersistInput{
		Manifest:   testManifest(t),
		Submission: submission,
		Result:     result,
	})
	if err == nil {
		t.Fatal("Persist() error = nil, want evaluator error")
	}
	if got := labkit.ClassifyError(err); got != labkit.ErrorClassEvaluator {
		t.Fatalf("ClassifyError() = %q, want %q", got, labkit.ErrorClassEvaluator)
	}
	if repo.beginCalls != 0 {
		t.Fatalf("begin calls = %d, want 0", repo.beginCalls)
	}
	if repo.lastTx != nil {
		t.Fatalf("transaction was started unexpectedly")
	}
}

func TestPersistLatestScoredSubmissionReplacesLeaderboardPointer(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo)
	manifest := testManifest(t)

	first := seededSubmission("44444444-4444-7444-8444-444444444444", 7, "sorting")
	second := seededSubmission("55555555-5555-7555-8555-555555555555", 7, "sorting")

	for _, submission := range []sqlc.Submissions{first, second} {
		if err := svc.Persist(context.Background(), PersistInput{
			Manifest:   manifest,
			Submission: submission,
			Result: evaluator.Result{
				Verdict: evaluator.VerdictScored,
				Scores: map[string]float64{
					"throughput": 1.82,
					"latency":    1.45,
				},
			},
			FinishedAt: time.Date(2026, 3, 31, 13, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("Persist() error = %v", err)
		}
	}

	lb := repo.leaderboard[leaderboardKey{userID: 7, labID: "sorting"}]
	if lb.SubmissionID != second.ID {
		t.Fatalf("leaderboard submission id = %v, want latest %v", lb.SubmissionID, second.ID)
	}
}

func newTestService(repo *fakeRepository) *Service {
	svc := NewService(repo)
	svc.now = func() time.Time { return repo.now }
	return svc
}

func testManifest(t *testing.T) *manifest.Manifest {
	t.Helper()

	m := &manifest.Manifest{
		Lab: manifest.LabSection{
			ID:   "sorting",
			Name: "Sorting Lab",
		},
		Submit: manifest.SubmitSection{
			Files: []string{"main.c", "README.md"},
		},
		Eval: manifest.EvalSection{
			Image: "ghcr.io/labkit/sorting:latest",
		},
		Quota: manifest.QuotaSection{
			Daily: 3,
		},
		Metrics: []manifest.MetricSection{
			{ID: "throughput", Sort: manifest.MetricSortDesc},
			{ID: "latency", Sort: manifest.MetricSortAsc},
		},
		Board: manifest.BoardSection{
			RankBy: "throughput",
			Pick:   true,
		},
		Schedule: manifest.ScheduleSection{
			Open:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("manifest.Validate() error = %v", err)
	}
	return m
}

func seededSubmission(rawID string, userID int64, labID string) sqlc.Submissions {
	return sqlc.Submissions{
		ID:          uuid.MustParse(rawID),
		UserID:      userID,
		LabID:       labID,
		KeyID:       11,
		ArtifactKey: "artifact-key",
		ContentHash: "content-hash",
		Status:      "running",
	}
}

type leaderboardKey struct {
	userID int64
	labID  string
}

type fakeRepository struct {
	now         time.Time
	beginCalls  int
	lastTx      *fakeTx
	submissions map[uuid.UUID]sqlc.Submissions
	scores      map[uuid.UUID]map[string]float32
	leaderboard map[leaderboardKey]sqlc.Leaderboard
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		now:         time.Date(2026, 3, 31, 13, 14, 15, 0, time.UTC),
		submissions: map[uuid.UUID]sqlc.Submissions{},
		scores:      map[uuid.UUID]map[string]float32{},
		leaderboard: map[leaderboardKey]sqlc.Leaderboard{},
	}
}

func (r *fakeRepository) BeginTx(context.Context) (Tx, error) {
	r.beginCalls++
	tx := &fakeTx{repo: r}
	r.lastTx = tx
	return tx, nil
}

type fakeTx struct {
	repo             *fakeRepository
	updateCalls      int
	scoreCalls       int
	leaderboardCalls int
	commitCalls      int
	rollbackCalls    int
	stagedScores     map[uuid.UUID]map[string]float32
}

func (tx *fakeTx) UpdateSubmissionResult(_ context.Context, arg sqlc.UpdateSubmissionResultParams) error {
	tx.updateCalls++

	row := tx.repo.submissions[arg.ID]
	row.Status = arg.Status
	row.Verdict = arg.Verdict
	row.Message = arg.Message
	row.Detail = append([]byte(nil), arg.Detail...)
	row.ImageDigest = arg.ImageDigest
	row.StartedAt = arg.StartedAt
	row.FinishedAt = arg.FinishedAt
	tx.repo.submissions[arg.ID] = row
	return nil
}

func (tx *fakeTx) CreateScore(_ context.Context, arg sqlc.CreateScoreParams) error {
	tx.scoreCalls++
	if tx.stagedScores == nil {
		tx.stagedScores = map[uuid.UUID]map[string]float32{}
	}
	if tx.stagedScores[arg.SubmissionID] == nil {
		tx.stagedScores[arg.SubmissionID] = map[string]float32{}
	}
	tx.stagedScores[arg.SubmissionID][arg.MetricID] = arg.Value
	return nil
}

func (tx *fakeTx) UpsertLeaderboardEntry(_ context.Context, arg sqlc.UpsertLeaderboardEntryParams) (sqlc.Leaderboard, error) {
	tx.leaderboardCalls++
	entry := sqlc.Leaderboard{
		UserID:       arg.UserID,
		LabID:        arg.LabID,
		SubmissionID: arg.SubmissionID,
		UpdatedAt:    pgtype.Timestamptz{Time: tx.repo.now, Valid: true},
	}
	tx.repo.leaderboard[leaderboardKey{userID: arg.UserID, labID: arg.LabID}] = entry
	return entry, nil
}

func (tx *fakeTx) Commit(context.Context) error {
	tx.commitCalls++
	for submissionID, scores := range tx.stagedScores {
		if tx.repo.scores[submissionID] == nil {
			tx.repo.scores[submissionID] = map[string]float32{}
		}
		for metricID, value := range scores {
			tx.repo.scores[submissionID][metricID] = value
		}
	}
	return nil
}

func (tx *fakeTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}

var _ interface {
	UpdateSubmissionResult(context.Context, sqlc.UpdateSubmissionResultParams) error
	CreateScore(context.Context, sqlc.CreateScoreParams) error
	UpsertLeaderboardEntry(context.Context, sqlc.UpsertLeaderboardEntryParams) (sqlc.Leaderboard, error)
	Commit(context.Context) error
	Rollback(context.Context) error
} = (*fakeTx)(nil)

var _ interface {
	BeginTx(context.Context) (Tx, error)
} = (*fakeRepository)(nil)
