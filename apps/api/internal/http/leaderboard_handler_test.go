package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	boardsvc "labkit.local/apps/api/internal/service/leaderboard"
)

func TestLeaderboardHandlerReturnsBoardJSON(t *testing.T) {
	svc := newLeaderboardHTTPTestService()
	handler := &LeaderboardHandler{Service: svc}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/board?by=latency_ms", nil)
	req.SetPathValue("labID", "sorting")

	handler.GetBoard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}

	var payload boardsvc.Board
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.SelectedMetric != "latency_ms" {
		t.Fatalf("selected metric = %q, want %q", payload.SelectedMetric, "latency_ms")
	}
	if len(payload.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(payload.Rows))
	}
	if payload.Rows[0].Rank != 1 {
		t.Fatalf("first rank = %d, want 1", payload.Rows[0].Rank)
	}
}

func TestLeaderboardHandlerReturnsNotFoundBeforeVisible(t *testing.T) {
	svc := newLeaderboardHTTPTestService()
	handler := &LeaderboardHandler{Service: svc}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/hidden/board", nil)
	req.SetPathValue("labID", "hidden")

	handler.GetBoard(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusNotFound, "", "lab_hidden", "Leaderboard is hidden")
}

func TestLeaderboardHandlerReturnsNotFoundForUnknownLab(t *testing.T) {
	handler := &LeaderboardHandler{Service: leaderboardHTTPErrorService{err: boardsvc.ErrLabNotFound}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/missing/board", nil)
	req.SetPathValue("labID", "missing")

	handler.GetBoard(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusNotFound, "", "lab_not_found", "Lab not found")
}

func TestLeaderboardHandlerReturnsInvalidMetric(t *testing.T) {
	handler := &LeaderboardHandler{Service: leaderboardHTTPErrorService{err: boardsvc.ErrMetricNotFound}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/board?by=missing", nil)
	req.SetPathValue("labID", "sorting")

	handler.GetBoard(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusBadRequest, "", "invalid_metric", "Invalid metric")
}

func TestLeaderboardHandlerReturnsInvalidManifest(t *testing.T) {
	handler := &LeaderboardHandler{Service: leaderboardHTTPErrorService{err: boardsvc.ErrInvalidManifest}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/board", nil)
	req.SetPathValue("labID", "sorting")

	handler.GetBoard(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusBadRequest, "", "invalid_manifest", "Invalid manifest")
}

func TestLeaderboardHandlerUsesBrowserSessionViewer(t *testing.T) {
	svc := &capturingLeaderboardHTTPService{}
	handler := &LeaderboardHandler{Service: svc}
	sessionToken, err := issueBrowserSession(7, 12, "")
	if err != nil {
		t.Fatalf("issueBrowserSession() error = %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/board", nil)
	req.SetPathValue("labID", "sorting")
	req.AddCookie(&http.Cookie{Name: browserSessionCookieName, Value: sessionToken})

	handler.GetBoard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.viewerUserID != 7 {
		t.Fatalf("viewerUserID = %d, want 7", svc.viewerUserID)
	}
}

type leaderboardHTTPTestService struct{}

func newLeaderboardHTTPTestService() *leaderboardHTTPTestService {
	return &leaderboardHTTPTestService{}
}

type leaderboardHTTPErrorService struct {
	err error
}

type capturingLeaderboardHTTPService struct {
	viewerUserID int64
}

func (s leaderboardHTTPErrorService) GetBoard(_ context.Context, _, _ string, _ int64) (boardsvc.Board, error) {
	return boardsvc.Board{}, s.err
}

func (s *capturingLeaderboardHTTPService) GetBoard(_ context.Context, _, _ string, viewerUserID int64) (boardsvc.Board, error) {
	s.viewerUserID = viewerUserID
	return boardsvc.Board{}, nil
}

func (s *leaderboardHTTPTestService) GetBoard(_ context.Context, labID, by string, _ int64) (boardsvc.Board, error) {
	switch labID {
	case "sorting":
		return boardsvc.Board{
			LabID:          "sorting",
			SelectedMetric: by,
			Metrics: []boardsvc.BoardMetric{
				{ID: "runtime_ms", Name: "Runtime", Sort: "desc", Selected: by == "" || by == "runtime_ms"},
				{ID: "latency_ms", Name: "Latency", Sort: "asc", Selected: by == "latency_ms"},
			},
			Rows: []boardsvc.BoardRow{
				{Rank: 1, Nickname: "Bob", Track: "latency", Scores: []boardsvc.BoardScore{{MetricID: "runtime_ms", Value: 88}, {MetricID: "latency_ms", Value: 35}}, UpdatedAt: time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)},
				{Rank: 2, Nickname: "Ada", Track: "throughput", Scores: []boardsvc.BoardScore{{MetricID: "runtime_ms", Value: 92}, {MetricID: "latency_ms", Value: 50}}, UpdatedAt: time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC)},
			},
		}, nil
	case "hidden":
		return boardsvc.Board{}, boardsvc.ErrLabHidden
	default:
		return boardsvc.Board{}, errors.New("unexpected lab")
	}
}
