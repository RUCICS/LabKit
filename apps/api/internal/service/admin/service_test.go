package admin

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestExportGradesUsesLatestLeaderboardState(t *testing.T) {
	repo := newAdminTestRepo(t)
	svc := NewService(repo)

	result, err := svc.ExportGrades(context.Background(), "sorting")
	if err != nil {
		t.Fatalf("ExportGrades() error = %v", err)
	}

	wantColumns := []string{"student_id", "rank", "nickname", "track", "runtime_ms", "latency_ms"}
	if !reflect.DeepEqual(result.Columns, wantColumns) {
		t.Fatalf("columns = %#v, want %#v", result.Columns, wantColumns)
	}
	if got := result.Filename; got != "sorting-grades.csv" {
		t.Fatalf("filename = %q, want %q", got, "sorting-grades.csv")
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(result.Rows))
	}
	if got := result.Rows[0].StudentID; got != "20260001" {
		t.Fatalf("first row student_id = %q, want %q", got, "20260001")
	}
	if got := result.Rows[0].Nickname; got != "Ada" {
		t.Fatalf("first row nickname = %q, want %q", got, "Ada")
	}
	if got := result.Rows[0].Rank; got != 1 {
		t.Fatalf("first row rank = %d, want 1", got)
	}
	if got := result.Rows[0].Scores; !reflect.DeepEqual(got, []string{"120", "31"}) {
		t.Fatalf("first row scores = %#v, want %#v", got, []string{"120", "31"})
	}
	if got := result.Rows[1].StudentID; got != "20260002" {
		t.Fatalf("second row student_id = %q, want %q", got, "20260002")
	}
	if got := result.Rows[1].Rank; got != 2 {
		t.Fatalf("second row rank = %d, want 2", got)
	}
}

func TestExportGradesForPickEnabledUsesDeclaredTrackMetric(t *testing.T) {
	repo := newAdminTestRepo(t)
	repo.labs["sorting"] = sqlc.Labs{
		ID:       "sorting",
		Name:     "Sorting Lab",
		Manifest: mustAdminManifestWithPick(t),
	}
	repo.scores[uuid.MustParse("22222222-2222-7222-8222-222222222222")] = []sqlc.Scores{
		{SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"), MetricID: "runtime_ms", Value: 90},
		{SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"), MetricID: "latency_ms", Value: 150},
	}

	svc := NewService(repo)

	result, err := svc.ExportGrades(context.Background(), "sorting")
	if err != nil {
		t.Fatalf("ExportGrades() error = %v", err)
	}

	if got := result.Rows[0].StudentID; got != "20260002" {
		t.Fatalf("first row student_id = %q, want %q", got, "20260002")
	}
	if got := result.Rows[0].Rank; got != 1 {
		t.Fatalf("first row rank = %d, want 1", got)
	}
	if got := result.Rows[1].StudentID; got != "20260001" {
		t.Fatalf("second row student_id = %q, want %q", got, "20260001")
	}
}

func TestReevaluateCreatesFreshJobsForCurrentLeaderboardState(t *testing.T) {
	repo := newAdminTestRepo(t)
	svc := NewService(repo)

	result, err := svc.Reevaluate(context.Background(), "sorting")
	if err != nil {
		t.Fatalf("Reevaluate() error = %v", err)
	}

	if got := result.JobsCreated; got != 2 {
		t.Fatalf("jobs created = %d, want 2", got)
	}
	if repo.tx == nil {
		t.Fatal("transaction was not started")
	}
	if repo.tx.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", repo.tx.commitCalls)
	}
	if repo.tx.rollbackCalls != 0 {
		t.Fatalf("rollback calls = %d, want 0", repo.tx.rollbackCalls)
	}
	if repo.listLeaderboardCalls != 0 {
		t.Fatalf("repository leaderboard calls = %d, want 0 before tx snapshot", repo.listLeaderboardCalls)
	}
	if repo.tx.listLeaderboardCalls != 1 {
		t.Fatalf("tx leaderboard calls = %d, want 1", repo.tx.listLeaderboardCalls)
	}
	if got := len(repo.tx.createdSubmissions); got != 2 {
		t.Fatalf("created submissions = %d, want 2", got)
	}
	if got := len(repo.tx.createdJobs); got != 2 {
		t.Fatalf("created jobs = %d, want 2", got)
	}
	if repo.tx.createdSubmissions[0].ArtifactKey != "sorting/7/11111111-1111-7111-8111-111111111111.tar.gz" {
		t.Fatalf("artifact key = %q, want %q", repo.tx.createdSubmissions[0].ArtifactKey, "sorting/7/11111111-1111-7111-8111-111111111111.tar.gz")
	}
	if repo.tx.createdSubmissions[0].Status != "queued" {
		t.Fatalf("submission status = %q, want %q", repo.tx.createdSubmissions[0].Status, "queued")
	}
}

func TestQueueStatusReturnsRecentJobs(t *testing.T) {
	repo := newAdminTestRepo(t)
	svc := NewService(repo)

	result, err := svc.QueueStatus(context.Background(), "sorting")
	if err != nil {
		t.Fatalf("QueueStatus() error = %v", err)
	}

	if got := result.LabID; got != "sorting" {
		t.Fatalf("lab_id = %q, want %q", got, "sorting")
	}
	if len(result.Jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(result.Jobs))
	}
	if got := result.Jobs[0].Status; got != "running" {
		t.Fatalf("first job status = %q, want %q", got, "running")
	}
	if got := result.Jobs[1].Status; got != "queued" {
		t.Fatalf("second job status = %q, want %q", got, "queued")
	}
	if repo.queueCalls != 1 {
		t.Fatalf("queue calls = %d, want 1", repo.queueCalls)
	}
}

func TestQueueStatusOrdersByRecentActivity(t *testing.T) {
	repo := newAdminTestRepo(t)
	repo.queueJobs["sorting"] = []sqlc.ListRecentEvaluationJobsByLabRow{
		{
			ID:           uuid.MustParse("bbbbbbbb-bbbb-7bbb-8bbb-bbbbbbbbbbbb"),
			SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"),
			UserID:       8,
			LabID:        "sorting",
			Status:       "queued",
			Attempts:     1,
			AvailableAt:  timestamptz("2026-03-31T12:10:00Z"),
			CreatedAt:    timestamptz("2026-03-31T12:00:00Z"),
			UpdatedAt:    timestamptz("2026-03-31T12:30:00Z"),
		},
		{
			ID:           uuid.MustParse("aaaaaaaa-aaaa-7aaa-8aaa-aaaaaaaaaaaa"),
			SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"),
			UserID:       7,
			LabID:        "sorting",
			Status:       "running",
			Attempts:     1,
			AvailableAt:  timestamptz("2026-03-31T12:00:00Z"),
			StartedAt:    timestamptz("2026-03-31T12:01:00Z"),
			CreatedAt:    timestamptz("2026-03-31T12:00:00Z"),
			UpdatedAt:    timestamptz("2026-03-31T12:05:00Z"),
		},
	}
	svc := NewService(repo)

	result, err := svc.QueueStatus(context.Background(), "sorting")
	if err != nil {
		t.Fatalf("QueueStatus() error = %v", err)
	}

	if got := result.Jobs[0].ID; got != "bbbbbbbb-bbbb-7bbb-8bbb-bbbbbbbbbbbb" {
		t.Fatalf("first job id = %q, want %q", got, "bbbbbbbb-bbbb-7bbb-8bbb-bbbbbbbbbbbb")
	}
	if got := result.Jobs[1].ID; got != "aaaaaaaa-aaaa-7aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("second job id = %q, want %q", got, "aaaaaaaa-aaaa-7aaa-8aaa-aaaaaaaaaaaa")
	}
}

func TestReevaluateIgnoresMalformedManifest(t *testing.T) {
	repo := newAdminTestRepo(t)
	repo.labs["sorting"] = sqlc.Labs{
		ID:       "sorting",
		Name:     "Sorting Lab",
		Manifest: json.RawMessage(`{"broken":`),
	}
	svc := NewService(repo)

	result, err := svc.Reevaluate(context.Background(), "sorting")
	if err != nil {
		t.Fatalf("Reevaluate() error = %v", err)
	}
	if result.JobsCreated != 2 {
		t.Fatalf("jobs created = %d, want 2", result.JobsCreated)
	}
}

func TestQueueStatusIgnoresMalformedManifest(t *testing.T) {
	repo := newAdminTestRepo(t)
	repo.labs["sorting"] = sqlc.Labs{
		ID:       "sorting",
		Name:     "Sorting Lab",
		Manifest: json.RawMessage(`{"broken":`),
	}
	svc := NewService(repo)

	result, err := svc.QueueStatus(context.Background(), "sorting")
	if err != nil {
		t.Fatalf("QueueStatus() error = %v", err)
	}
	if result.LabID != "sorting" {
		t.Fatalf("lab_id = %q, want %q", result.LabID, "sorting")
	}
}

type adminTestRepo struct {
	labs                 map[string]sqlc.Labs
	leaderboard          map[string][]sqlc.Leaderboard
	submissions          map[uuid.UUID]sqlc.Submissions
	scores               map[uuid.UUID][]sqlc.Scores
	profiles             map[string][]sqlc.LabProfiles
	users                map[int64]sqlc.Users
	queueJobs            map[string][]sqlc.ListRecentEvaluationJobsByLabRow
	tx                   *adminTestTx
	queueCalls           int
	listLeaderboardCalls int
}

func newAdminTestRepo(t *testing.T) *adminTestRepo {
	t.Helper()

	rawManifest := mustAdminManifest(t)
	return &adminTestRepo{
		labs: map[string]sqlc.Labs{
			"sorting": {
				ID:       "sorting",
				Name:     "Sorting Lab",
				Manifest: rawManifest,
			},
		},
		leaderboard: map[string][]sqlc.Leaderboard{
			"sorting": {
				{
					UserID:       7,
					LabID:        "sorting",
					SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"),
					UpdatedAt:    timestamptz("2026-03-31T12:00:00Z"),
				},
				{
					UserID:       8,
					LabID:        "sorting",
					SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"),
					UpdatedAt:    timestamptz("2026-03-31T12:05:00Z"),
				},
			},
		},
		submissions: map[uuid.UUID]sqlc.Submissions{
			uuid.MustParse("11111111-1111-7111-8111-111111111111"): {
				ID:          uuid.MustParse("11111111-1111-7111-8111-111111111111"),
				UserID:      7,
				LabID:       "sorting",
				KeyID:       11,
				ArtifactKey: "sorting/7/11111111-1111-7111-8111-111111111111.tar.gz",
				ContentHash: "hash-1",
				Status:      "done",
				CreatedAt:   timestamptz("2026-03-31T11:00:00Z"),
			},
			uuid.MustParse("22222222-2222-7222-8222-222222222222"): {
				ID:          uuid.MustParse("22222222-2222-7222-8222-222222222222"),
				UserID:      8,
				LabID:       "sorting",
				KeyID:       12,
				ArtifactKey: "sorting/8/22222222-2222-7222-8222-222222222222.tar.gz",
				ContentHash: "hash-2",
				Status:      "done",
				CreatedAt:   timestamptz("2026-03-31T11:05:00Z"),
			},
		},
		scores: map[uuid.UUID][]sqlc.Scores{
			uuid.MustParse("11111111-1111-7111-8111-111111111111"): {
				{SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"), MetricID: "runtime_ms", Value: 120},
				{SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"), MetricID: "latency_ms", Value: 31},
			},
			uuid.MustParse("22222222-2222-7222-8222-222222222222"): {
				{SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"), MetricID: "runtime_ms", Value: 90},
				{SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"), MetricID: "latency_ms", Value: 44},
			},
		},
		profiles: map[string][]sqlc.LabProfiles{
			"sorting": {
				{UserID: 7, LabID: "sorting", Nickname: "Ada", Track: pgtype.Text{String: "runtime_ms", Valid: true}},
				{UserID: 8, LabID: "sorting", Nickname: "Bob", Track: pgtype.Text{String: "latency_ms", Valid: true}},
			},
		},
		users: map[int64]sqlc.Users{
			7: {ID: 7, StudentID: "20260001", CreatedAt: timestamptz("2026-03-01T00:00:00Z")},
			8: {ID: 8, StudentID: "20260002", CreatedAt: timestamptz("2026-03-01T00:00:00Z")},
		},
		queueJobs: map[string][]sqlc.ListRecentEvaluationJobsByLabRow{
			"sorting": {
				{
					ID:           uuid.MustParse("aaaaaaaa-aaaa-7aaa-8aaa-aaaaaaaaaaaa"),
					SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"),
					UserID:       7,
					LabID:        "sorting",
					Status:       "running",
					Attempts:     1,
					AvailableAt:  timestamptz("2026-03-31T12:00:00Z"),
					StartedAt:    timestamptz("2026-03-31T12:01:00Z"),
					CreatedAt:    timestamptz("2026-03-31T12:00:00Z"),
					UpdatedAt:    timestamptz("2026-03-31T12:01:00Z"),
				},
				{
					ID:           uuid.MustParse("bbbbbbbb-bbbb-7bbb-8bbb-bbbbbbbbbbbb"),
					SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"),
					UserID:       8,
					LabID:        "sorting",
					Status:       "queued",
					Attempts:     0,
					AvailableAt:  timestamptz("2026-03-31T12:10:00Z"),
					CreatedAt:    timestamptz("2026-03-31T12:10:00Z"),
					UpdatedAt:    timestamptz("2026-03-31T12:10:00Z"),
				},
			},
		},
	}
}

func mustAdminManifest(t *testing.T) []byte {
	t.Helper()

	parsed, err := manifest.Parse([]byte(`
[lab]
id = "sorting"
name = "Sorting Lab"

[submit]
files = ["main.c"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/sorting:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
sort = "desc"

[[metric]]
id = "latency_ms"
sort = "asc"

[board]
rank_by = "runtime_ms"
pick = true

[schedule]
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`))
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	raw, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return raw
}

func mustAdminManifestWithPick(t *testing.T) []byte {
	t.Helper()

	parsed, err := manifest.Parse([]byte(`
[lab]
id = "sorting"
name = "Sorting Lab"

[submit]
files = ["main.c"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/sorting:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
sort = "desc"

[[metric]]
id = "latency_ms"
sort = "desc"

[board]
rank_by = "runtime_ms"
pick = true

[schedule]
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`))
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	raw, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return raw
}

func timestamptz(iso string) pgtype.Timestamptz {
	ts, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		panic(err)
	}
	return pgtype.Timestamptz{Time: ts, Valid: true}
}

type adminTestTx struct {
	createdSubmissions   []sqlc.CreateSubmissionParams
	createdJobs          []uuid.UUID
	commitCalls          int
	rollbackCalls        int
	nextSubmissionID     uuid.UUID
	listLeaderboardCalls int
}

func (tx *adminTestTx) ListLeaderboardByLab(_ context.Context, labID string) ([]sqlc.Leaderboard, error) {
	tx.listLeaderboardCalls++
	return []sqlc.Leaderboard{
		{
			UserID:       7,
			LabID:        labID,
			SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"),
			UpdatedAt:    timestamptz("2026-03-31T12:00:00Z"),
		},
		{
			UserID:       8,
			LabID:        labID,
			SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"),
			UpdatedAt:    timestamptz("2026-03-31T12:05:00Z"),
		},
	}, nil
}

func (r *adminTestRepo) GetLab(_ context.Context, labID string) (sqlc.Labs, error) {
	lab, ok := r.labs[labID]
	if !ok {
		return sqlc.Labs{}, pgx.ErrNoRows
	}
	return lab, nil
}

func (r *adminTestRepo) GetSubmission(_ context.Context, id uuid.UUID) (sqlc.Submissions, error) {
	submission, ok := r.submissions[id]
	if !ok {
		return sqlc.Submissions{}, pgx.ErrNoRows
	}
	return submission, nil
}

func (r *adminTestRepo) ListLeaderboardByLab(_ context.Context, labID string) ([]sqlc.Leaderboard, error) {
	r.listLeaderboardCalls++
	return append([]sqlc.Leaderboard(nil), r.leaderboard[labID]...), nil
}

func (r *adminTestRepo) ListLabProfilesByLab(_ context.Context, labID string) ([]sqlc.LabProfiles, error) {
	return append([]sqlc.LabProfiles(nil), r.profiles[labID]...), nil
}

func (r *adminTestRepo) ListScoresByLab(_ context.Context, labID string) ([]sqlc.Scores, error) {
	var out []sqlc.Scores
	for _, row := range r.leaderboard[labID] {
		out = append(out, r.scores[row.SubmissionID]...)
	}
	return out, nil
}

func (r *adminTestRepo) GetUserByID(_ context.Context, userID int64) (sqlc.Users, error) {
	user, ok := r.users[userID]
	if !ok {
		return sqlc.Users{}, pgx.ErrNoRows
	}
	return user, nil
}

func (r *adminTestRepo) ListRecentEvaluationJobsByLab(_ context.Context, _ sqlc.ListRecentEvaluationJobsByLabParams) ([]sqlc.ListRecentEvaluationJobsByLabRow, error) {
	r.queueCalls++
	return append([]sqlc.ListRecentEvaluationJobsByLabRow(nil), r.queueJobs["sorting"]...), nil
}

func (r *adminTestRepo) BeginTx(context.Context) (Tx, error) {
	r.tx = &adminTestTx{}
	return r.tx, nil
}

func (tx *adminTestTx) CreateSubmission(_ context.Context, arg sqlc.CreateSubmissionParams) (sqlc.Submissions, error) {
	tx.createdSubmissions = append(tx.createdSubmissions, arg)
	id := tx.nextSubmissionID
	if id == uuid.Nil {
		id = uuid.MustParse("cccccccc-cccc-7ccc-8ccc-cccccccccccc")
	}
	return sqlc.Submissions{
		ID:          id,
		UserID:      arg.UserID,
		LabID:       arg.LabID,
		KeyID:       arg.KeyID,
		ArtifactKey: arg.ArtifactKey,
		ContentHash: arg.ContentHash,
		Status:      arg.Status,
	}, nil
}

func (tx *adminTestTx) CreateEvaluationJob(_ context.Context, submissionID uuid.UUID) (sqlc.EvaluationJobs, error) {
	tx.createdJobs = append(tx.createdJobs, submissionID)
	return sqlc.EvaluationJobs{SubmissionID: submissionID}, nil
}

func (tx *adminTestTx) Commit(context.Context) error {
	tx.commitCalls++
	return nil
}

func (tx *adminTestTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}
