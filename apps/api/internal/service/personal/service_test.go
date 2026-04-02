package personal

import (
	"context"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestGetSubmissionDetailReturnsFinalStatusAndResult(t *testing.T) {
	submissionID := uuid.MustParse("11111111-1111-7111-8111-111111111111")
	finishedAt := time.Date(2026, 3, 31, 12, 10, 0, 0, time.UTC)
	createdAt := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	repo := &submissionDetailTestRepo{
		lab: sqlc.Labs{
			ID:       "sorting",
			Name:     "Sorting Lab",
			Manifest: []byte(`{"lab":{"id":"sorting"},"quota":{"daily":3}}`),
		},
		submission: sqlc.Submissions{
			ID:          submissionID,
			UserID:      7,
			LabID:       "sorting",
			KeyID:       11,
			Status:      "done",
			QuotaState:  "charged",
			Verdict:     pgtype.Text{String: "scored", Valid: true},
			Message:     pgtype.Text{String: "all good", Valid: true},
			Detail:      []byte(`{"format":"markdown","content":"great"}`),
			ContentHash: "hash-a",
			ArtifactKey: "sorting/7/a.tar.gz",
			FinishedAt:  pgtype.Timestamptz{Time: finishedAt, Valid: true},
			CreatedAt:   pgtype.Timestamptz{Time: createdAt, Valid: true},
		},
		scores: []sqlc.Scores{
			{SubmissionID: submissionID, MetricID: "latency", Value: 1.45},
			{SubmissionID: submissionID, MetricID: "throughput", Value: 1.82},
		},
		history: []sqlc.Submissions{
			{
				ID:         submissionID,
				UserID:     7,
				LabID:      "sorting",
				Status:     "done",
				QuotaState: "charged",
				CreatedAt:  pgtype.Timestamptz{Time: createdAt, Valid: true},
			},
		},
	}

	svc := NewService(repo)
	svc.now = func() time.Time { return createdAt }
	got, err := svc.GetSubmissionDetail(context.Background(), 7, "sorting", submissionID)
	if err != nil {
		t.Fatalf("GetSubmissionDetail() error = %v", err)
	}
	if got.ID != submissionID {
		t.Fatalf("ID = %v, want %v", got.ID, submissionID)
	}
	if got.Status != "done" {
		t.Fatalf("Status = %q, want %q", got.Status, "done")
	}
	if got.Verdict != "scored" {
		t.Fatalf("Verdict = %q, want %q", got.Verdict, "scored")
	}
	if got.Message != "all good" {
		t.Fatalf("Message = %q, want %q", got.Message, "all good")
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Fatalf("FinishedAt = %v, want %v", got.FinishedAt, finishedAt)
	}
	if got.CreatedAt != createdAt {
		t.Fatalf("CreatedAt = %v, want %v", got.CreatedAt, createdAt)
	}
	if got.LabID != "sorting" {
		t.Fatalf("LabID = %q, want %q", got.LabID, "sorting")
	}
	if got.KeyID != 11 {
		t.Fatalf("KeyID = %d, want %d", got.KeyID, 11)
	}
	if got.ArtifactKey != "sorting/7/a.tar.gz" {
		t.Fatalf("ArtifactKey = %q, want %q", got.ArtifactKey, "sorting/7/a.tar.gz")
	}
	if got.ContentHash != "hash-a" {
		t.Fatalf("ContentHash = %q, want %q", got.ContentHash, "hash-a")
	}
	if string(got.Detail) != `{"format":"markdown","content":"great"}` {
		t.Fatalf("Detail = %s, want %s", string(got.Detail), `{"format":"markdown","content":"great"}`)
	}
	if len(got.Scores) != 2 {
		t.Fatalf("len(Scores) = %d, want 2", len(got.Scores))
	}
	if got.Scores[0].MetricID != "latency" || got.Scores[1].MetricID != "throughput" {
		t.Fatalf("Scores = %#v, want latency then throughput", got.Scores)
	}
	if got.Quota == nil {
		t.Fatal("Quota = nil, want summary")
	}
	if got.Quota.Daily != 3 || got.Quota.Used != 1 || got.Quota.Left != 2 {
		t.Fatalf("Quota = %#v, want daily=3 used=1 left=2", got.Quota)
	}
}

func TestListSubmissionHistoryReturnsQuotaSummary(t *testing.T) {
	createdAt := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	repo := &submissionDetailTestRepo{
		lab: sqlc.Labs{
			ID:       "sorting",
			Name:     "Sorting Lab",
			Manifest: []byte(`{"lab":{"id":"sorting"},"quota":{"daily":3}}`),
		},
		history: []sqlc.Submissions{
			{
				ID:         uuid.MustParse("11111111-1111-7111-8111-111111111111"),
				UserID:     7,
				LabID:      "sorting",
				Status:     "done",
				QuotaState: "charged",
				CreatedAt:  pgtype.Timestamptz{Time: createdAt, Valid: true},
			},
			{
				ID:         uuid.MustParse("22222222-2222-7222-8222-222222222222"),
				UserID:     7,
				LabID:      "sorting",
				Status:     "queued",
				QuotaState: "pending",
				CreatedAt:  pgtype.Timestamptz{Time: createdAt.Add(-time.Hour), Valid: true},
			},
		},
	}

	svc := NewService(repo)
	svc.now = func() time.Time { return createdAt }
	got, err := svc.ListSubmissionHistory(context.Background(), 7, "sorting")
	if err != nil {
		t.Fatalf("ListSubmissionHistory() error = %v", err)
	}
	if got.Quota == nil {
		t.Fatal("Quota = nil, want summary")
	}
	if got.Quota.Daily != 3 || got.Quota.Used != 2 || got.Quota.Left != 1 {
		t.Fatalf("Quota = %#v, want daily=3 used=2 left=1", got.Quota)
	}
	if len(got.Submissions) != 2 {
		t.Fatalf("len(Submissions) = %d, want 2", len(got.Submissions))
	}
}

type submissionDetailTestRepo struct {
	lab        sqlc.Labs
	submission sqlc.Submissions
	history    []sqlc.Submissions
	scores     []sqlc.Scores
}

func (r *submissionDetailTestRepo) GetLab(context.Context, string) (sqlc.Labs, error) {
	return r.lab, nil
}

func (r *submissionDetailTestRepo) ListSubmissionsByUserLab(context.Context, sqlc.ListSubmissionsByUserLabParams) ([]sqlc.Submissions, error) {
	return append([]sqlc.Submissions(nil), r.history...), nil
}

func (r *submissionDetailTestRepo) GetSubmission(context.Context, uuid.UUID) (sqlc.Submissions, error) {
	if r.submission.ID == uuid.Nil {
		return sqlc.Submissions{}, pgx.ErrNoRows
	}
	return r.submission, nil
}

func (r *submissionDetailTestRepo) ListScoresBySubmission(context.Context, uuid.UUID) ([]sqlc.Scores, error) {
	return append([]sqlc.Scores(nil), r.scores...), nil
}

func (r *submissionDetailTestRepo) GetUserKeyByFingerprint(context.Context, string) (sqlc.UserKeys, error) {
	return sqlc.UserKeys{}, pgx.ErrNoRows
}

func (r *submissionDetailTestRepo) ReserveNonce(context.Context, string) (bool, error) {
	return true, nil
}

func (r *submissionDetailTestRepo) UpsertLabProfileNickname(context.Context, sqlc.UpsertLabProfileNicknameParams) (sqlc.LabProfiles, error) {
	return sqlc.LabProfiles{}, nil
}

func (r *submissionDetailTestRepo) UpsertLabProfileTrack(context.Context, sqlc.UpsertLabProfileTrackParams) (sqlc.LabProfiles, error) {
	return sqlc.LabProfiles{}, nil
}

func (r *submissionDetailTestRepo) ListUserKeys(context.Context, int64) ([]sqlc.UserKeys, error) {
	return nil, nil
}

func (r *submissionDetailTestRepo) DeleteUserKey(context.Context, sqlc.DeleteUserKeyParams) error {
	return nil
}

var _ Repository = (*submissionDetailTestRepo)(nil)
