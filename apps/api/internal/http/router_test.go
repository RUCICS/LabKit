package httpapi

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"labkit.local/apps/api/internal/http/middleware"
	labsvc "labkit.local/apps/api/internal/service/labs"
	boardsvc "labkit.local/apps/api/internal/service/leaderboard"
	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestHealthEndpointReturnsOKJSON(t *testing.T) {
	handler := NewRouter()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var payload map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := payload["status"]; got != "ok" {
		t.Fatalf("status = %q, want %q", got, "ok")
	}
}

func TestRequestIDMiddlewareSetsHeaderAndContext(t *testing.T) {
	const requestID = "request-123"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := middleware.RequestIDFromContext(r.Context()); got != requestID {
			t.Fatalf("request id in context = %q, want %q", got, requestID)
		}
		if got := w.Header().Get("X-Request-Id"); got != requestID {
			t.Fatalf("request id header in handler = %q, want %q", got, requestID)
		}
		_, _ = w.Write([]byte("ok"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", requestID)

	middleware.RequestIDMiddleware(next).ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Request-Id"); got != requestID {
		t.Fatalf("request id response header = %q, want %q", got, requestID)
	}
}

func TestRequestIDMiddlewarePanicsOnNilHandler(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("RequestIDMiddleware(nil) did not panic")
		}
	}()

	_ = middleware.RequestIDMiddleware(nil)
}

func TestRequestIDMiddlewareGeneratesDistinctFallbackIDsWhenRandFails(t *testing.T) {
	originalReader := cryptorand.Reader
	cryptorand.Reader = failingReader{}
	defer func() {
		cryptorand.Reader = originalReader
	}()

	var seen []string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, middleware.RequestIDFromContext(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	})

	handler := middleware.RequestIDMiddleware(next)

	for range 2 {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rr, req)
	}

	if len(seen) != 2 {
		t.Fatalf("captured %d request ids, want 2", len(seen))
	}
	for i, requestID := range seen {
		if requestID == "" {
			t.Fatalf("request id %d was empty", i)
		}
		if requestID == "unknown" {
			t.Fatalf("request id %d = %q, want generated fallback id", i, requestID)
		}
	}
	if seen[0] == seen[1] {
		t.Fatalf("request ids = %q and %q, want distinct fallback ids", seen[0], seen[1])
	}
}

func TestWriteErrorSerializesStructuredJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(middleware.ContextWithRequestID(req.Context(), "request-abc"))

	middleware.WriteError(rr, req, http.StatusBadRequest, "invalid_request", "bad thing")

	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want %q", got, "application/json")
	}
	if got := rr.Header().Get("X-Request-Id"); got != "request-abc" {
		t.Fatalf("request id header = %q, want %q", got, "request-abc")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var payload struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := payload.Error.Code; got != "invalid_request" {
		t.Fatalf("error code = %q, want %q", got, "invalid_request")
	}
	if got := payload.Error.Message; got != "bad thing" {
		t.Fatalf("error message = %q, want %q", got, "bad thing")
	}
	if got := payload.Error.RequestID; got != "request-abc" {
		t.Fatalf("error request_id = %q, want %q", got, "request-abc")
	}
}

func TestRouterGeneratesRequestIDForMatchedRoutes(t *testing.T) {
	handler := NewRouter()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler.ServeHTTP(rr, req)

	requestID := rr.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Fatalf("request id header was empty")
	}
	if requestID == "unknown" {
		t.Fatalf("request id header = %q, want generated request id", requestID)
	}
}

func TestRouterWiresLeaderboardRoute(t *testing.T) {
	handler := NewRouter(WithLeaderboardService(failingLeaderboardService{}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/board", nil)
	req.SetPathValue("labID", "sorting")

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestRouterReturnsStructuredJSONForNotFound(t *testing.T) {
	handler := NewRouter()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)

	handler.ServeHTTP(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusNotFound, "", "not_found", "Not Found")
}

func TestRouterReturnsStructuredJSONForMethodNotAllowed(t *testing.T) {
	handler := NewRouter()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("allow header = %q, want %q", got, "GET, HEAD")
	}

	assertStructuredErrorResponse(t, rr, http.StatusMethodNotAllowed, "", "method_not_allowed", "Method Not Allowed")
}

func TestRouterPreservesCanonicalPathRedirects(t *testing.T) {
	handler := NewRouter()

	testCases := []struct {
		name string
		path string
	}{
		{name: "double slash", path: "//healthz"},
		{name: "dot dot segment", path: "/../healthz"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusTemporaryRedirect {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusTemporaryRedirect)
			}
			if got := rr.Header().Get("Location"); got != "/healthz" {
				t.Fatalf("location = %q, want %q", got, "/healthz")
			}
			if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Fatalf("content type = %q, want %q", got, "text/html; charset=utf-8")
			}
		})
	}
}

func TestRouterLabsEndpointsAreReachableWithProvidedService(t *testing.T) {
	repo := newHTTPFakeLabRepository()
	service := labsvc.NewService(repo)
	router := NewRouter(WithLabsService(service), WithAdminToken("secret-token"))

	manifest := []byte(validLabManifestForHTTP(`
visible = 2000-03-30T00:00:00Z
open = 2000-03-31T00:00:00Z
close = 2099-04-30T00:00:00Z
`))

	registerResponse := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(http.MethodPost, "/api/admin/labs", bytes.NewReader(manifest))
	registerRequest.Header.Set("Authorization", "Bearer secret-token")
	router.ServeHTTP(registerResponse, registerRequest)

	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/admin/labs status = %d, want %d body=%s", registerResponse.Code, http.StatusCreated, registerResponse.Body.String())
	}

	secondRegisterResponse := httptest.NewRecorder()
	secondRegisterRequest := httptest.NewRequest(http.MethodPost, "/api/admin/labs", bytes.NewReader([]byte(nonStructuralLabManifestForHTTP(`
visible = 2000-03-30T00:00:00Z
open = 2000-03-31T00:00:00Z
close = 2099-04-30T00:00:00Z
`))))
	secondRegisterRequest.Header.Set("Authorization", "Bearer secret-token")
	router.ServeHTTP(secondRegisterResponse, secondRegisterRequest)

	assertStructuredErrorResponse(t, secondRegisterResponse, http.StatusConflict, "", "lab_already_exists", "Lab already exists")

	listResponse := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/labs", nil)
	router.ServeHTTP(listResponse, listRequest)

	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/labs status = %d, want %d body=%s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}

	var listPayload struct {
		Labs []struct {
			ID       string `json:"id"`
			Manifest struct {
				Eval struct {
					Image string `json:"image"`
				} `json:"eval"`
			} `json:"manifest"`
		} `json:"labs"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal GET /api/labs response: %v", err)
	}
	if len(listPayload.Labs) != 1 {
		t.Fatalf("len(labs) = %d, want 1", len(listPayload.Labs))
	}
	if listPayload.Labs[0].ID != "sorting" {
		t.Fatalf("lab id = %q, want %q", listPayload.Labs[0].ID, "sorting")
	}
	if listPayload.Labs[0].Manifest.Eval.Image != "" {
		t.Fatalf("public eval image = %q, want empty", listPayload.Labs[0].Manifest.Eval.Image)
	}

	detailResponse := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/labs/sorting", nil)
	router.ServeHTTP(detailResponse, detailRequest)

	if detailResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/labs/sorting status = %d, want %d body=%s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}

	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/labs/sorting", bytes.NewReader([]byte(nonStructuralLabManifestForHTTP(`
visible = 2000-03-30T00:00:00Z
open = 2000-03-31T00:00:00Z
close = 2099-04-30T00:00:00Z
`))))
	updateRequest.Header.Set("Authorization", "Bearer secret-token")
	router.ServeHTTP(updateResponse, updateRequest)

	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/admin/labs/sorting status = %d, want %d body=%s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}
}

func TestRouterAdminLabEndpointsRequireBearerToken(t *testing.T) {
	repo := newHTTPFakeLabRepository()
	service := labsvc.NewService(repo)
	router := NewRouter(WithLabsService(service), WithAdminToken("secret-token"))

	for _, tc := range []struct {
		name   string
		method string
		path   string
		token  string
	}{
		{name: "post missing token", method: http.MethodPost, path: "/api/admin/labs"},
		{name: "post wrong token", method: http.MethodPost, path: "/api/admin/labs", token: "Bearer wrong-token"},
		{name: "put missing token", method: http.MethodPut, path: "/api/admin/labs/sorting"},
		{name: "put wrong token", method: http.MethodPut, path: "/api/admin/labs/sorting", token: "Bearer wrong-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte(validLabManifestForHTTP(`
visible = 2000-03-30T00:00:00Z
open = 2000-03-31T00:00:00Z
close = 2099-04-30T00:00:00Z
`))))
			if tc.token != "" {
				req.Header.Set("Authorization", tc.token)
			}

			router.ServeHTTP(rr, req)

			assertStructuredErrorResponse(t, rr, http.StatusUnauthorized, "", "unauthorized", "Unauthorized")
		})
	}
}

func TestRouterAdminLabEndpointsAcceptCaseInsensitiveBearerScheme(t *testing.T) {
	repo := newHTTPFakeLabRepository()
	service := labsvc.NewService(repo)
	router := NewRouter(WithLabsService(service), WithAdminToken("secret-token"))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/labs", bytes.NewReader([]byte(validLabManifestForHTTP(`
visible = 2000-03-30T00:00:00Z
open = 2000-03-31T00:00:00Z
close = 2099-04-30T00:00:00Z
`))))
	req.Header.Set("Authorization", "bearer secret-token")

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/admin/labs status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestRouterWithoutLabsServiceReturnsStructuredErrorsForLabEndpoints(t *testing.T) {
	router := NewRouter()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs", nil)
	router.ServeHTTP(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusInternalServerError, "", "internal_server_error", "Internal Server Error")
}

func TestRouterReturnsStructuredInternalServerErrorForLabServiceFailures(t *testing.T) {
	router := NewRouter(WithLabsService(failingLabsService{err: errors.New("db offline")}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs", nil)
	router.ServeHTTP(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusInternalServerError, "", "internal_server_error", "Internal Server Error")
}

func TestRouterReturnsBadRequestForInvalidManifest(t *testing.T) {
	repo := newHTTPFakeLabRepository()
	service := labsvc.NewService(repo)
	router := NewRouter(WithLabsService(service), WithAdminToken("secret-token"))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/labs", bytes.NewReader([]byte("not toml")))
	req.Header.Set("Authorization", "Bearer secret-token")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal invalid manifest response: %v", err)
	}
	if payload.Error.Code != "invalid_manifest" {
		t.Fatalf("error code = %q, want %q", payload.Error.Code, "invalid_manifest")
	}
	if !strings.Contains(payload.Error.Message, "invalid manifest: parse manifest:") {
		t.Fatalf("error message = %q, want parse prefix", payload.Error.Message)
	}
}

func assertStructuredErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantRequestID, wantCode, wantMessage string) {
	t.Helper()

	if rr.Code != wantStatus {
		t.Fatalf("status = %d, want %d", rr.Code, wantStatus)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want %q", got, "application/json")
	}

	headerRequestID := rr.Header().Get("X-Request-Id")
	if wantRequestID == "" {
		if headerRequestID == "" {
			t.Fatalf("request id header was empty")
		}
		if headerRequestID == "unknown" {
			t.Fatalf("request id header = %q, want generated request id", headerRequestID)
		}
	} else if headerRequestID != wantRequestID {
		t.Fatalf("request id header = %q, want %q", headerRequestID, wantRequestID)
	}

	var payload struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := payload.Error.Code; got != wantCode {
		t.Fatalf("error code = %q, want %q", got, wantCode)
	}
	if got := payload.Error.Message; got != wantMessage {
		t.Fatalf("error message = %q, want %q", got, wantMessage)
	}
	if got := payload.Error.RequestID; got != headerRequestID {
		t.Fatalf("error request_id = %q, want %q", got, headerRequestID)
	}
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("boom")
}

var _ io.Reader = failingReader{}

type failingLabsService struct {
	err error
}

func (s failingLabsService) RegisterLab(context.Context, []byte) (labsvc.Lab, error) {
	return labsvc.Lab{}, s.err
}

func (s failingLabsService) UpdateLab(context.Context, string, []byte) (labsvc.Lab, error) {
	return labsvc.Lab{}, s.err
}

func (s failingLabsService) ListPublicLabs(context.Context) ([]labsvc.Lab, error) {
	return nil, s.err
}

func (s failingLabsService) GetPublicLab(context.Context, string) (labsvc.Lab, error) {
	return labsvc.Lab{}, s.err
}

type failingLeaderboardService struct{}

func (failingLeaderboardService) GetBoard(_ context.Context, labID, by string, _ int64) (boardsvc.Board, error) {
	return boardsvc.Board{
		LabID:          labID,
		SelectedMetric: by,
	}, nil
}

type httpFakeLabRepository struct {
	labs map[string]sqlc.Labs
}

func newHTTPFakeLabRepository() *httpFakeLabRepository {
	return &httpFakeLabRepository{labs: map[string]sqlc.Labs{}}
}

func (r *httpFakeLabRepository) CreateLab(_ context.Context, arg sqlc.CreateLabParams) (sqlc.Labs, error) {
	row := sqlc.Labs{
		ID:                arg.ID,
		Name:              arg.Name,
		Manifest:          append([]byte(nil), arg.Manifest...),
		ManifestUpdatedAt: pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC), Valid: true},
	}
	r.labs[arg.ID] = row
	return row, nil
}

func (r *httpFakeLabRepository) GetLab(_ context.Context, id string) (sqlc.Labs, error) {
	row, ok := r.labs[id]
	if !ok {
		return sqlc.Labs{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *httpFakeLabRepository) ListLabs(_ context.Context) ([]sqlc.Labs, error) {
	items := make([]sqlc.Labs, 0, len(r.labs))
	for _, lab := range r.labs {
		items = append(items, lab)
	}
	return items, nil
}

func (r *httpFakeLabRepository) UpdateLabManifest(_ context.Context, arg sqlc.UpdateLabManifestParams) error {
	row, ok := r.labs[arg.ID]
	if !ok {
		return pgx.ErrNoRows
	}
	row.Name = arg.Name
	row.Manifest = append([]byte(nil), arg.Manifest...)
	row.ManifestUpdatedAt = pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 13, 0, 0, 0, time.UTC), Valid: true}
	r.labs[arg.ID] = row
	return nil
}

func validLabManifestForHTTP(schedule string) string {
	return `
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
sort = "asc"
unit = "ms"

[board]
rank_by = "runtime_ms"
pick = false

[schedule]
` + schedule
}

func nonStructuralLabManifestForHTTP(schedule string) string {
	return `
[lab]
id = "sorting"
name = "Sorting Lab Renamed"

[submit]
files = ["main.py"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/sorting:2"
timeout = 120

[quota]
daily = 10
free = ["build_failed", "rejected"]

[[metric]]
id = "runtime_ms"
name = "Runtime v2"
sort = "asc"
unit = "milliseconds"

[board]
rank_by = "runtime_ms"
pick = true

[schedule]
` + schedule
}
