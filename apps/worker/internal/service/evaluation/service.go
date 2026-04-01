package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/evaluator"
	"labkit.local/packages/go/labkit"
	"labkit.local/packages/go/manifest"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	submissionStatusRunning = string(labkit.SubmissionStatusRunning)
	submissionStatusDone    = string(labkit.SubmissionStatusDone)
	submissionStatusError   = string(labkit.SubmissionStatusError)
)

type Repository interface {
	BeginTx(context.Context) (Tx, error)
	UpdateSubmissionRunning(context.Context, sqlc.UpdateSubmissionRunningParams) error
}

type Tx interface {
	UpdateSubmissionResult(context.Context, sqlc.UpdateSubmissionResultParams) error
	CreateScore(context.Context, sqlc.CreateScoreParams) error
	UpsertLeaderboardEntry(context.Context, sqlc.UpsertLeaderboardEntryParams) (sqlc.Leaderboard, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

type PersistInput struct {
	Manifest    *manifest.Manifest
	Submission  sqlc.Submissions
	Result      evaluator.Result
	ImageDigest string
	StartedAt   time.Time
	FinishedAt  time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

func (s *Service) MarkRunning(ctx context.Context, submissionID uuid.UUID, startedAt time.Time) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("evaluation service: repository is required")
	}
	return s.repo.UpdateSubmissionRunning(ctx, sqlc.UpdateSubmissionRunningParams{
		ID:        submissionID,
		Status:    submissionStatusRunning,
		StartedAt: timestamptzValue(startedAt),
	})
}

func (s *Service) Persist(ctx context.Context, in PersistInput) (err error) {
	if s == nil || s.repo == nil {
		return fmt.Errorf("evaluation service: repository is required")
	}

	if err := evaluator.ValidateResult(in.Manifest, in.Result); err != nil {
		return labkit.WrapEvaluatorError("invalid evaluator output", err)
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		if rbErr := tx.Rollback(ctx); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	if err := tx.UpdateSubmissionResult(ctx, sqlc.UpdateSubmissionResultParams{
		ID:          in.Submission.ID,
		Status:      submissionStatusFor(in.Result.Verdict),
		Verdict:     textValue(string(in.Result.Verdict)),
		Message:     textValue(in.Result.Message),
		Detail:      detailBytes(in.Result.Detail),
		ImageDigest: textValue(in.ImageDigest),
		StartedAt:   timestamptzValue(in.StartedAt),
		FinishedAt:  timestamptzValue(in.FinishedAt),
	}); err != nil {
		return err
	}

	if in.Result.Verdict == evaluator.VerdictScored {
		if err := persistScores(ctx, tx, in.Submission.ID, in.Result.Scores); err != nil {
			return err
		}
		if err := refreshLeaderboard(ctx, tx, in.Submission); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func persistScores(ctx context.Context, tx Tx, submissionID uuid.UUID, scores map[string]float64) error {
	for metricID, value := range scores {
		if err := tx.CreateScore(ctx, sqlc.CreateScoreParams{
			SubmissionID: submissionID,
			MetricID:     metricID,
			Value:        float32(value),
		}); err != nil {
			return err
		}
	}
	return nil
}

func submissionStatusFor(verdict evaluator.Verdict) string {
	if verdict == evaluator.VerdictError {
		return submissionStatusError
	}
	return submissionStatusDone
}

func textValue(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func timestamptzValue(v time.Time) pgtype.Timestamptz {
	if v.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: v.UTC(), Valid: true}
}

func detailBytes(detail *evaluator.Detail) []byte {
	if detail == nil {
		return nil
	}
	data, err := json.Marshal(detail)
	if err != nil {
		return nil
	}
	return data
}
