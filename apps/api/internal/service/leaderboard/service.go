package leaderboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	submissionsvc "labkit.local/apps/api/internal/service/submissions"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrLabNotFound     = errors.New("lab not found")
	ErrLabHidden       = errors.New("lab hidden")
	ErrMetricNotFound  = errors.New("metric not found")
	ErrInvalidManifest = errors.New("invalid manifest")
)

type Repository interface {
	GetLab(context.Context, string) (sqlc.Labs, error)
	ListLeaderboardByLabAndMetricAsc(context.Context, sqlc.ListLeaderboardByLabAndMetricAscParams) ([]sqlc.ListLeaderboardByLabAndMetricAscRow, error)
	ListLeaderboardByLabAndMetricDesc(context.Context, sqlc.ListLeaderboardByLabAndMetricDescParams) ([]sqlc.ListLeaderboardByLabAndMetricDescRow, error)
	ListScoresByLab(context.Context, string) ([]sqlc.Scores, error)
	ListLabProfilesByLab(context.Context, string) ([]sqlc.LabProfiles, error)
	CountSubmissionQuotaUsage(context.Context, int64, string, time.Time, time.Time) (int64, error)
}

type Service struct {
	repo          Repository
	now           func() time.Time
	quotaLocation *time.Location
}

type Board struct {
	LabID          string                      `json:"lab_id"`
	SelectedMetric string                      `json:"selected_metric"`
	Metrics        []BoardMetric               `json:"metrics"`
	Rows           []BoardRow                  `json:"rows"`
	Quota          *submissionsvc.QuotaSummary `json:"quota,omitempty"`
}

type BoardMetric struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Sort     string `json:"sort"`
	Selected bool   `json:"selected,omitempty"`
}

type BoardRow struct {
	Rank        int          `json:"rank"`
	Nickname    string       `json:"nickname"`
	Track       string       `json:"track,omitempty"`
	Scores      []BoardScore `json:"scores"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CurrentUser bool         `json:"current_user,omitempty"`
}

type BoardScore struct {
	MetricID string  `json:"metric_id"`
	Value    float32 `json:"value"`
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, quotaLocation: submissionsvc.DefaultQuotaLocation()}
}

func (s *Service) SetQuotaLocation(location *time.Location) {
	if location != nil {
		s.quotaLocation = location
	}
}

func (s *Service) GetBoard(ctx context.Context, labID, by string, viewerUserID int64) (Board, error) {
	if s == nil || s.repo == nil {
		return Board{}, fmt.Errorf("leaderboard service unavailable")
	}
	labRow, err := s.repo.GetLab(ctx, strings.TrimSpace(labID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Board{}, ErrLabNotFound
		}
		return Board{}, err
	}
	parsed, err := parseManifest(labRow.Manifest)
	if err != nil {
		return Board{}, err
	}
	if !isVisible(parsed.Schedule, s.nowUTC()) {
		return Board{}, ErrLabHidden
	}
	selectedMetric, err := resolveMetric(parsed, by)
	if err != nil {
		return Board{}, err
	}

	leaderboardRows, err := s.listLeaderboardRows(ctx, labRow.ID, selectedMetric)
	if err != nil {
		return Board{}, err
	}
	scoresBySubmission, err := s.listScoresByLab(ctx, labRow.ID)
	if err != nil {
		return Board{}, err
	}

	profiles, err := s.repo.ListLabProfilesByLab(ctx, labRow.ID)
	if err != nil {
		return Board{}, err
	}
	profileByUser := make(map[int64]sqlc.LabProfiles, len(profiles))
	for _, profile := range profiles {
		profileByUser[profile.UserID] = profile
	}

	rows := make([]boardEntry, 0, len(leaderboardRows))
	for _, row := range leaderboardRows {
		scoreRows := scoresBySubmission[row.submissionID]
		rowScores := scoresForMetrics(parsed.Metrics, scoreRows)
		selectedValue := metricValueFor(rowScores, selectedMetric.ID)
		nickname := "匿名"
		if profile, ok := profileByUser[row.userID]; ok && strings.TrimSpace(profile.Nickname) != "" {
			nickname = strings.TrimSpace(profile.Nickname)
		}
		boardRow := BoardRow{
			Nickname:    nickname,
			Scores:      rowScores,
			UpdatedAt:   row.updatedAt,
			CurrentUser: viewerUserID != 0 && row.userID == viewerUserID,
		}
		if parsed.Board.Pick {
			if profile, ok := profileByUser[row.userID]; ok && profile.Track.Valid && strings.TrimSpace(profile.Track.String) != "" {
				boardRow.Track = strings.TrimSpace(profile.Track.String)
			}
		}
		rows = append(rows, boardEntry{
			userID:        row.userID,
			selectedValue: selectedValue,
			updatedAt:     row.updatedAt,
			row:           boardRow,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		switch selectedMetric.Sort {
		case manifest.MetricSortAsc:
			if left.selectedValue != right.selectedValue {
				return left.selectedValue < right.selectedValue
			}
			if !left.updatedAt.Equal(right.updatedAt) {
				return left.updatedAt.Before(right.updatedAt)
			}
		default:
			if left.selectedValue != right.selectedValue {
				return left.selectedValue > right.selectedValue
			}
			if !left.updatedAt.Equal(right.updatedAt) {
				return left.updatedAt.After(right.updatedAt)
			}
		}
		return left.userID < right.userID
	})

	board := Board{
		LabID:          labRow.ID,
		SelectedMetric: selectedMetric.ID,
		Metrics:        make([]BoardMetric, 0, len(parsed.Metrics)),
		Rows:           make([]BoardRow, 0, len(rows)),
	}
	for _, metric := range parsed.Metrics {
		board.Metrics = append(board.Metrics, BoardMetric{
			ID:       metric.ID,
			Name:     metric.Name,
			Sort:     string(metric.Sort),
			Selected: metric.ID == selectedMetric.ID,
		})
	}
	for i := range rows {
		rows[i].row.Rank = i + 1
		board.Rows = append(board.Rows, rows[i].row)
	}
	if viewerUserID != 0 {
		windowStart, windowEnd := submissionsvc.QuotaWindowForTime(s.nowUTC(), s.quotaLocationOrDefault())
		used, err := s.repo.CountSubmissionQuotaUsage(ctx, viewerUserID, labRow.ID, windowStart, windowEnd)
		if err != nil {
			return Board{}, err
		}
		board.Quota = submissionsvc.BuildQuotaSummary(parsed, int(used), s.quotaLocationOrDefault())
	}
	return board, nil
}

func (s *Service) nowUTC() time.Time {
	now := s.now
	if now == nil {
		now = time.Now
	}
	return now().UTC()
}

func (s *Service) quotaLocationOrDefault() *time.Location {
	if s != nil && s.quotaLocation != nil {
		return s.quotaLocation
	}
	return submissionsvc.DefaultQuotaLocation()
}

func parseManifest(raw json.RawMessage) (*manifest.Manifest, error) {
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
	for _, metric := range m.Metrics {
		if metric.ID == wanted {
			return metric, nil
		}
	}
	return manifest.MetricSection{}, ErrMetricNotFound
}

func isVisible(schedule manifest.ScheduleSection, now time.Time) bool {
	if schedule.Visible.IsZero() {
		return true
	}
	return !now.Before(schedule.Visible)
}

func (s *Service) listLeaderboardRows(ctx context.Context, labID string, metric manifest.MetricSection) ([]leaderboardRow, error) {
	switch metric.Sort {
	case manifest.MetricSortAsc:
		rows, err := s.repo.ListLeaderboardByLabAndMetricAsc(ctx, sqlc.ListLeaderboardByLabAndMetricAscParams{
			LabID:    labID,
			MetricID: metric.ID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]leaderboardRow, 0, len(rows))
		for _, row := range rows {
			out = append(out, leaderboardRow{
				userID:       row.UserID,
				submissionID: row.SubmissionID,
				updatedAt:    row.UpdatedAt.Time.UTC(),
			})
		}
		return out, nil
	case manifest.MetricSortDesc:
		rows, err := s.repo.ListLeaderboardByLabAndMetricDesc(ctx, sqlc.ListLeaderboardByLabAndMetricDescParams{
			LabID:    labID,
			MetricID: metric.ID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]leaderboardRow, 0, len(rows))
		for _, row := range rows {
			out = append(out, leaderboardRow{
				userID:       row.UserID,
				submissionID: row.SubmissionID,
				updatedAt:    row.UpdatedAt.Time.UTC(),
			})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported metric sort: %s", metric.Sort)
	}
}

func (s *Service) listScoresByLab(ctx context.Context, labID string) (map[uuid.UUID][]sqlc.Scores, error) {
	rows, err := s.repo.ListScoresByLab(ctx, labID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]sqlc.Scores)
	for _, row := range rows {
		score := sqlc.Scores{
			SubmissionID: row.SubmissionID,
			MetricID:     row.MetricID,
			Value:        row.Value,
		}
		out[row.SubmissionID] = append(out[row.SubmissionID], score)
	}
	return out, nil
}

type leaderboardRow struct {
	userID       int64
	submissionID uuid.UUID
	updatedAt    time.Time
}

type boardEntry struct {
	userID        int64
	selectedValue float32
	updatedAt     time.Time
	row           BoardRow
}

func scoresForMetrics(metrics []manifest.MetricSection, scores []sqlc.Scores) []BoardScore {
	scoreByMetric := make(map[string]float32, len(scores))
	for _, score := range scores {
		scoreByMetric[score.MetricID] = score.Value
	}
	items := make([]BoardScore, 0, len(metrics))
	for _, metric := range metrics {
		items = append(items, BoardScore{
			MetricID: metric.ID,
			Value:    scoreByMetric[metric.ID],
		})
	}
	return items
}

func metricValueFor(scores []BoardScore, metricID string) float32 {
	for _, score := range scores {
		if score.MetricID == metricID {
			return score.Value
		}
	}
	return 0
}
