package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrLabNotFound     = errors.New("lab not found")
	ErrInvalidManifest = errors.New("invalid manifest")
)

const recentQueueLimit int32 = 20

type Repository interface {
	GetLab(context.Context, string) (sqlc.Labs, error)
	GetSubmission(context.Context, uuid.UUID) (sqlc.Submissions, error)
	GetUserByID(context.Context, int64) (sqlc.Users, error)
	ListLeaderboardByLab(context.Context, string) ([]sqlc.Leaderboard, error)
	ListLabProfilesByLab(context.Context, string) ([]sqlc.LabProfiles, error)
	ListScoresByLab(context.Context, string) ([]sqlc.Scores, error)
	ListRecentEvaluationJobsByLab(context.Context, sqlc.ListRecentEvaluationJobsByLabParams) ([]sqlc.ListRecentEvaluationJobsByLabRow, error)
	BeginTx(context.Context) (Tx, error)
}

type Tx interface {
	ListLeaderboardByLab(context.Context, string) ([]sqlc.Leaderboard, error)
	CreateSubmission(context.Context, sqlc.CreateSubmissionParams) (sqlc.Submissions, error)
	CreateEvaluationJob(context.Context, uuid.UUID) (sqlc.EvaluationJobs, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

type ExportGradesResult struct {
	LabID    string            `json:"lab_id"`
	Filename string            `json:"filename"`
	Columns  []string          `json:"columns"`
	Rows     []ExportGradesRow `json:"rows"`
}

type ExportGradesRow struct {
	StudentID string   `json:"student_id"`
	Rank      int      `json:"rank"`
	Nickname  string   `json:"nickname"`
	Track     string   `json:"track"`
	Scores    []string `json:"scores"`
}

type ReevaluateResult struct {
	LabID       string `json:"lab_id"`
	JobsCreated int    `json:"jobs_created"`
}

type QueueStatus struct {
	LabID string     `json:"lab_id"`
	Jobs  []QueueJob `json:"jobs"`
}

type QueueJob struct {
	ID           string    `json:"id"`
	SubmissionID string    `json:"submission_id"`
	UserID       int64     `json:"user_id"`
	Status       string    `json:"status"`
	Attempts     int32     `json:"attempts"`
	AvailableAt  time.Time `json:"available_at"`
	WorkerID     string    `json:"worker_id,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type gradeRow struct {
	userID       int64
	submissionID uuid.UUID
	sortValue    float64
	updatedAt    time.Time
	row          ExportGradesRow
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) ExportGrades(ctx context.Context, labID string) (ExportGradesResult, error) {
	labRow, parsed, err := s.loadLab(ctx, labID)
	if err != nil {
		return ExportGradesResult{}, err
	}
	rows, profiles, scoresBySubmission, usersByID, err := s.loadGradeInputs(ctx, labRow.ID)
	if err != nil {
		return ExportGradesResult{}, err
	}
	rankMetric, err := resolveMetric(parsed, parsed.Board.RankBy)
	if err != nil {
		return ExportGradesResult{}, err
	}

	exportRows := make([]gradeRow, 0, len(rows))
	for _, row := range rows {
		scoreRows := scoresBySubmission[row.SubmissionID]
		scoreMap := make(map[string]float32, len(scoreRows))
		for _, score := range scoreRows {
			scoreMap[score.MetricID] = score.Value
		}
		rowMetric := rankMetric
		if parsed.Board.Pick {
			if profile, ok := profiles[row.UserID]; ok && profile.Track.Valid && strings.TrimSpace(profile.Track.String) != "" {
				if metric, err := resolveMetric(parsed, profile.Track.String); err == nil {
					rowMetric = metric
				}
			}
		}
		exportRow := ExportGradesRow{
			StudentID: usersByID[row.UserID].StudentID,
			Nickname:  "匿名",
			Track:     "",
			Scores:    make([]string, 0, len(parsed.Metrics)),
		}
		if profile, ok := profiles[row.UserID]; ok {
			if strings.TrimSpace(profile.Nickname) != "" {
				exportRow.Nickname = strings.TrimSpace(profile.Nickname)
			}
			if profile.Track.Valid && strings.TrimSpace(profile.Track.String) != "" {
				exportRow.Track = strings.TrimSpace(profile.Track.String)
			}
		}
		for _, metric := range parsed.Metrics {
			exportRow.Scores = append(exportRow.Scores, scoreText(scoreMap[metric.ID]))
		}
		exportRows = append(exportRows, gradeRow{
			userID:       row.UserID,
			submissionID: row.SubmissionID,
			sortValue:    normalizedScore(scoreMap[rowMetric.ID], rowMetric.Sort),
			updatedAt:    row.UpdatedAt.Time.UTC(),
			row:          exportRow,
		})
	}

	sortGradeRows(exportRows)

	result := ExportGradesResult{
		LabID:    labRow.ID,
		Filename: fmt.Sprintf("%s-grades.csv", labRow.ID),
		Columns:  gradeColumns(parsed.Metrics),
		Rows:     make([]ExportGradesRow, 0, len(exportRows)),
	}
	for i := range exportRows {
		exportRows[i].row.Rank = i + 1
		result.Rows = append(result.Rows, exportRows[i].row)
	}
	return result, nil
}

func (s *Service) Reevaluate(ctx context.Context, labID string) (ReevaluateResult, error) {
	labRow, err := s.loadLabRow(ctx, labID)
	if err != nil {
		return ReevaluateResult{}, err
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return ReevaluateResult{}, err
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback(ctx)
	}()

	rows, err := tx.ListLeaderboardByLab(ctx, labRow.ID)
	if err != nil {
		return ReevaluateResult{}, err
	}

	created := 0
	for _, row := range rows {
		submission, err := s.repo.GetSubmission(ctx, row.SubmissionID)
		if err != nil {
			return ReevaluateResult{}, err
		}
		createdSubmission, err := tx.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
			UserID:      submission.UserID,
			LabID:       submission.LabID,
			KeyID:       submission.KeyID,
			ArtifactKey: submission.ArtifactKey,
			ContentHash: submission.ContentHash,
			Status:      "queued",
			Verdict:     pgtype.Text{},
			Message:     pgtype.Text{},
			Detail:      nil,
			ImageDigest: pgtype.Text{},
			StartedAt:   pgtype.Timestamptz{},
			FinishedAt:  pgtype.Timestamptz{},
		})
		if err != nil {
			return ReevaluateResult{}, err
		}
		if _, err := tx.CreateEvaluationJob(ctx, createdSubmission.ID); err != nil {
			return ReevaluateResult{}, err
		}
		created++
	}
	if err := tx.Commit(ctx); err != nil {
		return ReevaluateResult{}, err
	}
	committed = true

	return ReevaluateResult{LabID: labRow.ID, JobsCreated: created}, nil
}

func (s *Service) QueueStatus(ctx context.Context, labID string) (QueueStatus, error) {
	labRow, err := s.loadLabRow(ctx, labID)
	if err != nil {
		return QueueStatus{}, err
	}
	rows, err := s.repo.ListRecentEvaluationJobsByLab(ctx, sqlc.ListRecentEvaluationJobsByLabParams{
		LabID: labRow.ID,
		Limit: recentQueueLimit,
	})
	if err != nil {
		return QueueStatus{}, err
	}

	result := QueueStatus{
		LabID: labRow.ID,
		Jobs:  make([]QueueJob, 0, len(rows)),
	}
	for _, row := range rows {
		result.Jobs = append(result.Jobs, QueueJob{
			ID:           row.ID.String(),
			SubmissionID: row.SubmissionID.String(),
			UserID:       row.UserID,
			Status:       row.Status,
			Attempts:     row.Attempts,
			AvailableAt:  row.AvailableAt.Time.UTC(),
			WorkerID:     textValue(row.WorkerID),
			LastError:    textValue(row.LastError),
			StartedAt:    timestamptzValue(row.StartedAt),
			FinishedAt:   timestamptzValue(row.FinishedAt),
			CreatedAt:    row.CreatedAt.Time.UTC(),
			UpdatedAt:    row.UpdatedAt.Time.UTC(),
		})
	}
	return result, nil
}

func (s *Service) loadLab(ctx context.Context, labID string) (sqlc.Labs, *manifest.Manifest, error) {
	labRow, err := s.loadLabRow(ctx, labID)
	if err != nil {
		return sqlc.Labs{}, nil, err
	}
	parsed, err := parseStoredManifest(labRow.Manifest)
	if err != nil {
		return sqlc.Labs{}, nil, err
	}
	return labRow, parsed, nil
}

func (s *Service) loadLabRow(ctx context.Context, labID string) (sqlc.Labs, error) {
	if s == nil || s.repo == nil {
		return sqlc.Labs{}, fmt.Errorf("admin service unavailable")
	}
	labRow, err := s.repo.GetLab(ctx, strings.TrimSpace(labID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Labs{}, ErrLabNotFound
		}
		return sqlc.Labs{}, err
	}
	return labRow, nil
}

func (s *Service) loadGradeInputs(ctx context.Context, labID string) ([]sqlc.Leaderboard, map[int64]sqlc.LabProfiles, map[uuid.UUID][]sqlc.Scores, map[int64]sqlc.Users, error) {
	leaderboardRows, err := s.repo.ListLeaderboardByLab(ctx, labID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	profileRows, err := s.repo.ListLabProfilesByLab(ctx, labID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	scoreRows, err := s.repo.ListScoresByLab(ctx, labID)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	profiles := make(map[int64]sqlc.LabProfiles, len(profileRows))
	for _, profile := range profileRows {
		profiles[profile.UserID] = profile
	}
	scoresBySubmission := make(map[uuid.UUID][]sqlc.Scores)
	for _, score := range scoreRows {
		scoresBySubmission[score.SubmissionID] = append(scoresBySubmission[score.SubmissionID], score)
	}
	usersByID := make(map[int64]sqlc.Users, len(leaderboardRows))
	for _, row := range leaderboardRows {
		user, err := s.repo.GetUserByID(ctx, row.UserID)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		usersByID[row.UserID] = user
	}

	return leaderboardRows, profiles, scoresBySubmission, usersByID, nil
}

func parseStoredManifest(raw json.RawMessage) (*manifest.Manifest, error) {
	var parsed manifest.Manifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	return &parsed, nil
}

func resolveMetric(m *manifest.Manifest, by string) (manifest.MetricSection, error) {
	wanted := strings.TrimSpace(by)
	if wanted == "" {
		wanted = m.Board.RankBy
	}
	if wanted == "" && len(m.Metrics) > 0 {
		return m.Metrics[0], nil
	}
	for _, metric := range m.Metrics {
		if metric.ID == wanted {
			return metric, nil
		}
	}
	return manifest.MetricSection{}, ErrInvalidManifest
}

func sortGradeRows(rows []gradeRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		if left.sortValue != right.sortValue {
			return left.sortValue > right.sortValue
		}
		if !left.updatedAt.Equal(right.updatedAt) {
			return left.updatedAt.After(right.updatedAt)
		}
		return left.userID < right.userID
	})
}

func normalizedScore(value float32, sortDir manifest.MetricSort) float64 {
	score := float64(value)
	if sortDir == manifest.MetricSortAsc {
		return -score
	}
	return score
}

func scoreText(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', -1, 32)
}

func gradeColumns(metrics []manifest.MetricSection) []string {
	columns := []string{"student_id", "rank", "nickname", "track"}
	for _, metric := range metrics {
		columns = append(columns, metric.ID)
	}
	return columns
}

func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return strings.TrimSpace(v.String)
}

func timestamptzValue(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}
