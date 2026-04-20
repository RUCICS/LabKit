package leaderboard

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"
	"strings"

	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestGetBoardDefaultsToRankByAndOrdersDescending(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	board, err := svc.GetBoard(context.Background(), "sorting", "", 0)
	if err != nil {
		t.Fatalf("GetBoard() error = %v", err)
	}

	if got := board.SelectedMetric; got != "runtime_ms" {
		t.Fatalf("selected metric = %q, want %q", got, "runtime_ms")
	}
	if len(board.Rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(board.Rows))
	}
	if got := board.Rows[0].Nickname; got != "Ada" {
		t.Fatalf("first row nickname = %q, want %q", got, "Ada")
	}
	if got := board.Rows[1].Nickname; got != "Bob" {
		t.Fatalf("second row nickname = %q, want %q", got, "Bob")
	}
	if got := board.Rows[2].Nickname; got != "Cara" {
		t.Fatalf("third row nickname = %q, want %q", got, "Cara")
	}
	if board.Rows[0].Track != "" {
		t.Fatalf("track column = %q, want empty when pick is disabled", board.Rows[0].Track)
	}
	if repo.descCalls != 1 {
		t.Fatalf("desc leaderboard calls = %d, want 1", repo.descCalls)
	}
	if repo.boardCalls != 0 {
		t.Fatalf("legacy leaderboard calls = %d, want 0", repo.boardCalls)
	}
	if repo.scoreLabCalls != 1 {
		t.Fatalf("batched score calls = %d, want 1", repo.scoreLabCalls)
	}
	if repo.scoreCalls != 0 {
		t.Fatalf("per-submission score calls = %d, want 0", repo.scoreCalls)
	}
	if repo.profileCalls != 1 {
		t.Fatalf("profile calls = %d, want 1", repo.profileCalls)
	}
}

func TestGetBoardOrdersBySelectedMetricAscending(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	board, err := svc.GetBoard(context.Background(), "sorting", "latency_ms", 0)
	if err != nil {
		t.Fatalf("GetBoard() error = %v", err)
	}

	if got := board.SelectedMetric; got != "latency_ms" {
		t.Fatalf("selected metric = %q, want %q", got, "latency_ms")
	}
	if got := board.Rows[0].Nickname; got != "Cara" {
		t.Fatalf("first row nickname = %q, want %q", got, "Cara")
	}
	if got := board.Rows[1].Nickname; got != "Bob" {
		t.Fatalf("second row nickname = %q, want %q", got, "Bob")
	}
	if got := board.Rows[2].Nickname; got != "Ada" {
		t.Fatalf("third row nickname = %q, want %q", got, "Ada")
	}
	if board.Rows[0].Scores[0].MetricID != "runtime_ms" {
		t.Fatalf("scores are not preserved in manifest order: got %q", board.Rows[0].Scores[0].MetricID)
	}
	if repo.ascCalls != 1 {
		t.Fatalf("asc leaderboard calls = %d, want 1", repo.ascCalls)
	}
	if repo.boardCalls != 0 {
		t.Fatalf("legacy leaderboard calls = %d, want 0", repo.boardCalls)
	}
	if repo.scoreLabCalls != 1 {
		t.Fatalf("batched score calls = %d, want 1", repo.scoreLabCalls)
	}
	if repo.scoreCalls != 0 {
		t.Fatalf("per-submission score calls = %d, want 0", repo.scoreCalls)
	}
}

func TestGetBoardBreaksTiesByUserID(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	board, err := svc.GetBoard(context.Background(), "tiecase", "", 0)
	if err != nil {
		t.Fatalf("GetBoard() error = %v", err)
	}

	if got := board.Rows[0].Nickname; got != "Alpha" {
		t.Fatalf("first row nickname = %q, want %q", got, "Alpha")
	}
	if got := board.Rows[1].Nickname; got != "Beta" {
		t.Fatalf("second row nickname = %q, want %q", got, "Beta")
	}
}

func TestGetBoardShowsTrackWhenPickEnabled(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	board, err := svc.GetBoard(context.Background(), "picksort", "", 0)
	if err != nil {
		t.Fatalf("GetBoard() error = %v", err)
	}

	if got := board.Rows[0].Track; got != "throughput" {
		t.Fatalf("track = %q, want %q", got, "throughput")
	}
}

func TestGetBoardQuotaSummaryMarksCurrentUserRow(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	board, err := svc.GetBoard(context.Background(), "sorting", "", 2)
	if err != nil {
		t.Fatalf("GetBoard() error = %v", err)
	}

	if !board.Rows[1].CurrentUser {
		t.Fatalf("board row %#v, want current user row flagged", board.Rows[1])
	}
	if board.Rows[0].CurrentUser {
		t.Fatalf("board row %#v, want non-current user row unflagged", board.Rows[0])
	}
	if board.Quota == nil {
		t.Fatal("Quota = nil, want summary for signed viewer")
	}
	if board.Quota.Daily != 3 || board.Quota.Used != 1 || board.Quota.Left != 2 {
		t.Fatalf("Quota = %#v, want daily=3 used=1 left=2", board.Quota)
	}
}

func TestGetBoardRejectsHiddenLabBeforeVisibility(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	_, err := svc.GetBoard(context.Background(), "hidden", "", 0)
	if !errors.Is(err, ErrLabHidden) {
		t.Fatalf("GetBoard() error = %v, want ErrLabHidden", err)
	}
}

func TestGetBoardRejectsUnknownLab(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	_, err := svc.GetBoard(context.Background(), "missing", "", 0)
	if !errors.Is(err, ErrLabNotFound) {
		t.Fatalf("GetBoard() error = %v, want ErrLabNotFound", err)
	}
}

func newTestService(repo *fakeRepository, now time.Time) *Service {
	svc := NewService(repo)
	svc.now = func() time.Time { return now }
	return svc
}

type fakeRepository struct {
	labs          map[string]sqlc.Labs
	board         map[string][]sqlc.Leaderboard
	scores        map[uuid.UUID][]sqlc.Scores
	profiles      map[string][]sqlc.LabProfiles
	labQueries    int
	boardCalls    int
	ascCalls      int
	descCalls     int
	scoreCalls    int
	scoreLabCalls int
	profileCalls  int
	quotaUsage    int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		labs: map[string]sqlc.Labs{
			"sorting": mustLabRow(`
[lab]
id = "sorting"
name = "Sorting Lab"

[submit]
files = ["main.py"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/sorting:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
name = "Runtime"
sort = "desc"
unit = "ms"

[[metric]]
id = "latency_ms"
name = "Latency"
sort = "asc"
unit = "ms"

[board]
rank_by = "runtime_ms"
pick = false

[schedule]
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)),
			"picksort": mustLabRow(`
[lab]
id = "picksort"
name = "Pick Sort"

[submit]
files = ["main.py"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/picksort:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "throughput"
name = "Throughput"
sort = "desc"
unit = "x"

[[metric]]
id = "latency"
name = "Latency"
sort = "asc"
unit = "ms"

[board]
rank_by = "throughput"
pick = true

[schedule]
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)),
			"hidden": mustLabRow(`
[lab]
id = "hidden"
name = "Hidden Lab"

[submit]
files = ["main.py"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/hidden:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
name = "Runtime"
sort = "desc"
unit = "ms"

[board]
rank_by = "runtime_ms"
pick = false

[schedule]
visible = 2026-04-01T00:00:00Z
open = 2026-04-02T00:00:00Z
close = 2026-04-30T00:00:00Z
`, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)),
			"tiecase": mustLabRow(`
[lab]
id = "tiecase"
name = "Tie Case"

[submit]
files = ["main.py"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/tiecase:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
name = "Runtime"
sort = "desc"
unit = "ms"

[board]
rank_by = "runtime_ms"
pick = false

[schedule]
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)),
		},
		board: map[string][]sqlc.Leaderboard{
			"sorting": {
				{UserID: 1, LabID: "sorting", SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"), UpdatedAt: timestamptz(time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC))},
				{UserID: 2, LabID: "sorting", SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"), UpdatedAt: timestamptz(time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC))},
				{UserID: 3, LabID: "sorting", SubmissionID: uuid.MustParse("33333333-3333-7333-8333-333333333333"), UpdatedAt: timestamptz(time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC))},
			},
			"picksort": {
				{UserID: 4, LabID: "picksort", SubmissionID: uuid.MustParse("44444444-4444-7444-8444-444444444444"), UpdatedAt: timestamptz(time.Date(2026, 3, 31, 11, 30, 0, 0, time.UTC))},
				{UserID: 5, LabID: "picksort", SubmissionID: uuid.MustParse("55555555-5555-7555-8555-555555555555"), UpdatedAt: timestamptz(time.Date(2026, 3, 31, 10, 30, 0, 0, time.UTC))},
			},
			"tiecase": {
				{UserID: 7, LabID: "tiecase", SubmissionID: uuid.MustParse("77777777-7777-7777-8777-777777777777"), UpdatedAt: timestamptz(time.Date(2026, 3, 31, 11, 15, 0, 0, time.UTC))},
				{UserID: 6, LabID: "tiecase", SubmissionID: uuid.MustParse("66666666-6666-7666-8666-666666666666"), UpdatedAt: timestamptz(time.Date(2026, 3, 31, 11, 15, 0, 0, time.UTC))},
			},
		},
		scores: map[uuid.UUID][]sqlc.Scores{
			uuid.MustParse("11111111-1111-7111-8111-111111111111"): {
				{SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"), MetricID: "runtime_ms", Value: 92},
				{SubmissionID: uuid.MustParse("11111111-1111-7111-8111-111111111111"), MetricID: "latency_ms", Value: 50},
			},
			uuid.MustParse("22222222-2222-7222-8222-222222222222"): {
				{SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"), MetricID: "runtime_ms", Value: 88},
				{SubmissionID: uuid.MustParse("22222222-2222-7222-8222-222222222222"), MetricID: "latency_ms", Value: 40},
			},
			uuid.MustParse("33333333-3333-7333-8333-333333333333"): {
				{SubmissionID: uuid.MustParse("33333333-3333-7333-8333-333333333333"), MetricID: "runtime_ms", Value: 88},
				{SubmissionID: uuid.MustParse("33333333-3333-7333-8333-333333333333"), MetricID: "latency_ms", Value: 35},
			},
			uuid.MustParse("44444444-4444-7444-8444-444444444444"): {
				{SubmissionID: uuid.MustParse("44444444-4444-7444-8444-444444444444"), MetricID: "throughput", Value: 2.0},
				{SubmissionID: uuid.MustParse("44444444-4444-7444-8444-444444444444"), MetricID: "latency", Value: 1.2},
			},
			uuid.MustParse("55555555-5555-7555-8555-555555555555"): {
				{SubmissionID: uuid.MustParse("55555555-5555-7555-8555-555555555555"), MetricID: "throughput", Value: 1.8},
				{SubmissionID: uuid.MustParse("55555555-5555-7555-8555-555555555555"), MetricID: "latency", Value: 1.0},
			},
			uuid.MustParse("77777777-7777-7777-8777-777777777777"): {
				{SubmissionID: uuid.MustParse("77777777-7777-7777-8777-777777777777"), MetricID: "runtime_ms", Value: 90},
			},
			uuid.MustParse("66666666-6666-7666-8666-666666666666"): {
				{SubmissionID: uuid.MustParse("66666666-6666-7666-8666-666666666666"), MetricID: "runtime_ms", Value: 90},
			},
		},
		profiles: map[string][]sqlc.LabProfiles{
			"sorting": {
				{UserID: 1, LabID: "sorting", Nickname: "Ada (lab)"},
				{UserID: 2, LabID: "sorting", Nickname: "Bob (lab)"},
				{UserID: 3, LabID: "sorting", Nickname: "Cara (lab)"},
			},
			"picksort": {
				{UserID: 4, LabID: "picksort", Nickname: "Ada (lab)", Track: text("throughput")},
				{UserID: 5, LabID: "picksort", Nickname: "Bob (lab)", Track: text("latency")},
			},
			"tiecase": {
				{UserID: 7, LabID: "tiecase", Nickname: "Beta (lab)"},
				{UserID: 6, LabID: "tiecase", Nickname: "Alpha (lab)"},
			},
		},
		quotaUsage: 1,
	}
}

func (r *fakeRepository) CountSubmissionQuotaUsage(_ context.Context, _ int64, _ string, _ time.Time, _ time.Time) (int64, error) {
	return r.quotaUsage, nil
}

func (r *fakeRepository) GetLab(_ context.Context, labID string) (sqlc.Labs, error) {
	r.labQueries++
	row, ok := r.labs[labID]
	if !ok {
		return sqlc.Labs{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *fakeRepository) ListLeaderboardByLabAndMetricAsc(_ context.Context, arg sqlc.ListLeaderboardByLabAndMetricAscParams) ([]sqlc.ListLeaderboardByLabAndMetricAscRow, error) {
	r.ascCalls++
	items := r.boardRowsForMetric(arg.LabID, arg.MetricID, false)
	return items, nil
}

func (r *fakeRepository) ListLeaderboardByLabAndMetricDesc(_ context.Context, arg sqlc.ListLeaderboardByLabAndMetricDescParams) ([]sqlc.ListLeaderboardByLabAndMetricDescRow, error) {
	r.descCalls++
	items := r.boardRowsForMetric(arg.LabID, arg.MetricID, true)
	out := make([]sqlc.ListLeaderboardByLabAndMetricDescRow, 0, len(items))
	for _, item := range items {
		out = append(out, sqlc.ListLeaderboardByLabAndMetricDescRow{
			UserID:       item.UserID,
			Nickname:     item.Nickname,
			LabID:        item.LabID,
			SubmissionID: item.SubmissionID,
			UpdatedAt:    item.UpdatedAt,
			MetricID:     item.MetricID,
			Value:        item.Value,
		})
	}
	return out, nil
}

func (r *fakeRepository) ListLeaderboardByLab(_ context.Context, labID string) ([]sqlc.Leaderboard, error) {
	r.boardCalls++
	return append([]sqlc.Leaderboard(nil), r.board[labID]...), nil
}

func (r *fakeRepository) ListScoresByLab(_ context.Context, labID string) ([]sqlc.Scores, error) {
	r.scoreLabCalls++
	var items []sqlc.Scores
	for _, row := range r.board[labID] {
		items = append(items, r.scores[row.SubmissionID]...)
	}
	return items, nil
}

func (r *fakeRepository) ListScoresBySubmission(_ context.Context, submissionID uuid.UUID) ([]sqlc.Scores, error) {
	r.scoreCalls++
	return append([]sqlc.Scores(nil), r.scores[submissionID]...), nil
}

func (r *fakeRepository) ListLabProfilesByLab(_ context.Context, labID string) ([]sqlc.LabProfiles, error) {
	r.profileCalls++
	return append([]sqlc.LabProfiles(nil), r.profiles[labID]...), nil
}

func mustLabRow(body string, updatedAt time.Time) sqlc.Labs {
	parsed, err := manifest.Parse([]byte(body))
	if err != nil {
		panic(err)
	}
	raw, err := json.Marshal(parsed)
	if err != nil {
		panic(err)
	}
	return sqlc.Labs{
		ID:                parsed.Lab.ID,
		Name:              parsed.Lab.Name,
		Manifest:          raw,
		ManifestUpdatedAt: timestamptz(updatedAt),
	}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func text(v string) pgtype.Text {
	return pgtype.Text{String: v, Valid: true}
}

func (r *fakeRepository) boardRowsForMetric(labID, metricID string, desc bool) []sqlc.ListLeaderboardByLabAndMetricAscRow {
	rows := r.board[labID]
	items := make([]sqlc.ListLeaderboardByLabAndMetricAscRow, 0, len(rows))
	for _, row := range rows {
		score := scoreForMetric(r.scores[row.SubmissionID], metricID)
		nickname := ""
		for _, profile := range r.profiles[labID] {
			if profile.UserID == row.UserID {
				nickname = strings.TrimSuffix(profile.Nickname, " (lab)")
				break
			}
		}
		items = append(items, sqlc.ListLeaderboardByLabAndMetricAscRow{
			UserID:       row.UserID,
			Nickname:     nickname,
			LabID:        row.LabID,
			SubmissionID: row.SubmissionID,
			UpdatedAt:    row.UpdatedAt,
			MetricID:     metricID,
			Value:        score,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		leftScore := items[i].Value
		rightScore := items[j].Value
		if leftScore == rightScore {
			if desc {
				return items[i].UpdatedAt.Time.After(items[j].UpdatedAt.Time)
			}
			return items[i].UpdatedAt.Time.Before(items[j].UpdatedAt.Time)
		}
		if desc {
			return leftScore > rightScore
		}
		return leftScore < rightScore
	})
	return items
}

func scoreForMetric(scores []sqlc.Scores, metricID string) float32 {
	for _, score := range scores {
		if score.MetricID == metricID {
			return score.Value
		}
	}
	return 0
}
