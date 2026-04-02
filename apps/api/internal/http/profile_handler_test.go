package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	personal "labkit.local/apps/api/internal/service/personal"
	"labkit.local/packages/go/auth"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/ssh"
)

func TestHistoryHandlerListsSubmissionHistory(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	svc.SetNow(func() time.Time { return time.Date(2026, 3, 31, 13, 30, 0, 0, time.UTC) })
	handler := &HistoryHandler{Service: svc}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/history", nil)
	req.SetPathValue("labID", "sorting")
	repo.signRequest(t, req, "/api/labs/sorting/history", nil, "nonce-history", 11)

	handler.ListHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload struct {
		Submissions []personal.HistoryItem `json:"submissions"`
		Quota       map[string]any         `json:"quota"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.Submissions) != 2 {
		t.Fatalf("len(submissions) = %d, want 2", len(payload.Submissions))
	}
	if payload.Submissions[0].ID != uuid.MustParse("22222222-2222-7222-8222-222222222222") {
		t.Fatalf("first submission id = %v, want %v", payload.Submissions[0].ID, uuid.MustParse("22222222-2222-7222-8222-222222222222"))
	}
	if payload.Submissions[0].Status != "queued" {
		t.Fatalf("first submission status = %q, want %q", payload.Submissions[0].Status, "queued")
	}
	if payload.Quota == nil {
		t.Fatal("quota = nil, want summary")
	}
	if payload.Quota["left"] != float64(1) {
		t.Fatalf("quota.left = %#v, want 1", payload.Quota["left"])
	}
}

func TestHistoryHandlerFetchesSubmissionDetailWithScores(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	svc.SetNow(func() time.Time { return time.Date(2026, 3, 31, 13, 30, 0, 0, time.UTC) })
	handler := &HistoryHandler{Service: svc}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/submissions/11111111-1111-7111-8111-111111111111", nil)
	req.SetPathValue("labID", "sorting")
	req.SetPathValue("submissionID", "11111111-1111-7111-8111-111111111111")
	repo.signRequest(t, req, "/api/labs/sorting/submissions/11111111-1111-7111-8111-111111111111", nil, "nonce-detail", 11)

	handler.GetSubmissionDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload personal.SubmissionDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.ID != uuid.MustParse("11111111-1111-7111-8111-111111111111") {
		t.Fatalf("submission id = %v, want %v", payload.ID, uuid.MustParse("11111111-1111-7111-8111-111111111111"))
	}
	if len(payload.Scores) != 2 {
		t.Fatalf("len(scores) = %d, want 2", len(payload.Scores))
	}
	if payload.Scores[0].MetricID != "latency" {
		t.Fatalf("first score metric = %q, want %q", payload.Scores[0].MetricID, "latency")
	}
	if payload.Quota == nil || payload.Quota.Left != 1 {
		t.Fatalf("quota = %#v, want left=1", payload.Quota)
	}
}

func TestHistoryHandlerReturnsSubmissionDetailFieldsForWaitFlow(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &HistoryHandler{Service: svc}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/submissions/11111111-1111-7111-8111-111111111111", nil)
	req.SetPathValue("labID", "sorting")
	req.SetPathValue("submissionID", "11111111-1111-7111-8111-111111111111")
	repo.signRequest(t, req, "/api/labs/sorting/submissions/11111111-1111-7111-8111-111111111111", nil, "nonce-detail-fields", 11)

	handler.GetSubmissionDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	wantStringField(t, payload, "id", "11111111-1111-7111-8111-111111111111")
	wantStringField(t, payload, "status", "done")
	wantStringField(t, payload, "verdict", "scored")
	wantStringField(t, payload, "message", "all good")
	wantStringField(t, payload, "lab_id", "sorting")
	wantNumberField(t, payload, "key_id", 11)
	wantStringField(t, payload, "artifact_key", "sorting/7/a.tar.gz")
	wantStringField(t, payload, "content_hash", "hash-a")

	detail, ok := payload["detail"].(map[string]any)
	if !ok {
		t.Fatalf("detail = %#v, want object", payload["detail"])
	}
	if detail["format"] != "markdown" {
		t.Fatalf("detail.format = %#v, want %q", detail["format"], "markdown")
	}
	if detail["content"] != "great" {
		t.Fatalf("detail.content = %#v, want %q", detail["content"], "great")
	}

	scores, ok := payload["scores"].([]any)
	if !ok {
		t.Fatalf("scores = %#v, want array", payload["scores"])
	}
	if len(scores) != 2 {
		t.Fatalf("len(scores) = %d, want 2", len(scores))
	}
	firstScore, ok := scores[0].(map[string]any)
	if !ok {
		t.Fatalf("scores[0] = %#v, want object", scores[0])
	}
	if firstScore["metric_id"] != "latency" {
		t.Fatalf("scores[0].metric_id = %#v, want %q", firstScore["metric_id"], "latency")
	}

	finishedAt, ok := payload["finished_at"].(string)
	if !ok || finishedAt == "" {
		t.Fatalf("finished_at = %#v, want non-empty string", payload["finished_at"])
	}
}

func TestHistoryHandlerRejectsOtherUsersSubmissionDetail(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &HistoryHandler{Service: svc}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/submissions/33333333-3333-7333-8333-333333333333", nil)
	req.SetPathValue("labID", "sorting")
	req.SetPathValue("submissionID", "33333333-3333-7333-8333-333333333333")
	repo.signRequest(t, req, "/api/labs/sorting/submissions/33333333-3333-7333-8333-333333333333", nil, "nonce-other-detail", 11)

	handler.GetSubmissionDetail(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusNotFound, "", "submission_not_found", "Submission not found")
}

func TestProfileHandlerUpdatesNickname(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &ProfileHandler{Service: svc}

	body := bytes.NewBufferString(`{"nickname":" Cat "}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/labs/sorting/nickname", body)
	req.SetPathValue("labID", "sorting")
	repo.signRequest(t, req, "/api/labs/sorting/nickname", []byte(`{"nickname":" Cat "}`), "nonce-nickname", 11)

	handler.UpdateNickname(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload personal.Profile
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Nickname != "Cat" {
		t.Fatalf("nickname = %q, want %q", payload.Nickname, "Cat")
	}
	if payload.Track != "throughput" {
		t.Fatalf("track = %q, want %q", payload.Track, "throughput")
	}
}

func TestProfileHandlerRejectsTrackUpdateWhenPickDisabled(t *testing.T) {
	repo := newPersonalTestRepo(t, false)
	svc := personal.NewService(repo)
	handler := &ProfileHandler{Service: svc}

	body := bytes.NewBufferString(`{"track":"throughput"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/labs/sorting/track", body)
	req.SetPathValue("labID", "sorting")
	repo.signRequest(t, req, "/api/labs/sorting/track", []byte(`{"track":"throughput"}`), "nonce-track-disabled", 11)

	handler.UpdateTrack(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusBadRequest, "", "track_not_allowed", "Track selection is disabled")
}

func TestProfileHandlerUpdatesTrackWhenPickEnabled(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &ProfileHandler{Service: svc}

	body := bytes.NewBufferString(`{"track":"latency"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/labs/sorting/track", body)
	req.SetPathValue("labID", "sorting")
	repo.signRequest(t, req, "/api/labs/sorting/track", []byte(`{"track":"latency"}`), "nonce-track-enabled", 11)

	handler.UpdateTrack(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload personal.Profile
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Track != "latency" {
		t.Fatalf("track = %q, want %q", payload.Track, "latency")
	}
}

func TestKeysHandlerListsAndRevokesKeys(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &KeysHandler{Service: svc}

	listRR := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	repo.signRequest(t, listReq, "/api/keys", nil, "nonce-keys-list", 11)
	handler.ListKeys(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d body=%s", listRR.Code, http.StatusOK, listRR.Body.String())
	}

	var listPayload struct {
		Keys []personal.Key `json:"keys"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listPayload.Keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(listPayload.Keys))
	}

	deleteRR := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/keys/11", nil)
	deleteReq.SetPathValue("keyID", "11")
	repo.signRequest(t, deleteReq, "/api/keys/11", nil, "nonce-keys-delete", 11)
	handler.RevokeKey(deleteRR, deleteReq)

	if deleteRR.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d body=%s", deleteRR.Code, http.StatusNoContent, deleteRR.Body.String())
	}

	row, ok := repo.keys[11]
	if !ok {
		t.Fatal("revoked key row removed, want soft delete")
	}
	if !row.RevokedAt.Valid {
		t.Fatalf("revoked_at = %#v, want valid timestamp", row.RevokedAt)
	}

	listRR = httptest.NewRecorder()
	listReq = httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	sessionToken, err := issueBrowserSession(7, 12, "")
	if err != nil {
		t.Fatalf("issueBrowserSession() error = %v", err)
	}
	listReq.AddCookie(&http.Cookie{Name: browserSessionCookieName, Value: sessionToken})
	handler.ListKeys(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("list-after-revoke status = %d, want %d body=%s", listRR.Code, http.StatusOK, listRR.Body.String())
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal list-after-revoke response: %v", err)
	}
	if len(listPayload.Keys) != 1 {
		t.Fatalf("len(keys after revoke) = %d, want 1", len(listPayload.Keys))
	}
	if listPayload.Keys[0].ID != 12 {
		t.Fatalf("remaining key id = %d, want 12", listPayload.Keys[0].ID)
	}

	unauthRR := httptest.NewRecorder()
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	repo.signRequest(t, unauthReq, "/api/keys", nil, "nonce-keys-list-revoked", 11)
	handler.ListKeys(unauthRR, unauthReq)

	assertStructuredErrorResponse(t, unauthRR, http.StatusUnauthorized, "", "unauthorized", "Unauthorized")
}

func TestKeysHandlerDoesNotRevokeOtherUsersKey(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &KeysHandler{Service: svc}

	deleteRR := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/keys/21", nil)
	deleteReq.SetPathValue("keyID", "21")
	repo.signRequest(t, deleteReq, "/api/keys/21", nil, "nonce-foreign-key", 11)
	handler.RevokeKey(deleteRR, deleteReq)

	if deleteRR.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d body=%s", deleteRR.Code, http.StatusNoContent, deleteRR.Body.String())
	}

	if _, ok := repo.keys[21]; !ok {
		t.Fatalf("foreign key was deleted, want ownership boundary to preserve it")
	}
}

func TestBrowserSessionDoesNotAuthorizeMutatingPersonalRequests(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &ProfileHandler{Service: svc}

	req := httptest.NewRequest(http.MethodPut, "/api/labs/sorting/nickname", bytes.NewBufferString(`{"nickname":"Cat"}`))
	req.SetPathValue("labID", "sorting")
	req.AddCookie(&http.Cookie{Name: browserSessionCookieName, Value: "session-token"})
	rr := httptest.NewRecorder()

	handler.UpdateNickname(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusUnauthorized, "", "unauthorized", "Unauthorized")
}

func wantStringField(t *testing.T, payload map[string]any, key, want string) {
	t.Helper()
	got, ok := payload[key].(string)
	if !ok {
		t.Fatalf("%s = %#v, want %q", key, payload[key], want)
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}

func wantNumberField(t *testing.T, payload map[string]any, key string, want int64) {
	t.Helper()
	got, ok := payload[key].(float64)
	if !ok {
		t.Fatalf("%s = %#v, want number", key, payload[key])
	}
	if int64(got) != want {
		t.Fatalf("%s = %v, want %d", key, got, want)
	}
}

func TestRouterWiresAuthenticatedPersonalRoutes(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	personalSvc := personal.NewService(repo)
	router := NewRouter(WithPersonalService(personalSvc))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/history", nil)
	req.SetPathValue("labID", "sorting")
	repo.signRequest(t, req, "/api/labs/sorting/history", nil, "nonce-router-history", 11)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("router history status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	repo.signRequest(t, req, "/api/keys", nil, "nonce-router-keys", 11)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("router keys status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestProfileHandlerRejectsMissingAuthentication(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &HistoryHandler{Service: svc}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/history", nil)
	req.SetPathValue("labID", "sorting")

	handler.ListHistory(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusUnauthorized, "", "unauthorized", "Unauthorized")
}

func TestHistoryHandlerRejectsForgedKeyFingerprint(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	svc := personal.NewService(repo)
	handler := &HistoryHandler{Service: svc}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/history", nil)
	req.SetPathValue("labID", "sorting")
	repo.signRequest(t, req, "/api/labs/sorting/history", nil, "nonce-forged", 11)
	req.Header.Set("X-LabKit-Key-Fingerprint", "SHA256:forged")

	handler.ListHistory(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusUnauthorized, "", "unauthorized", "Unauthorized")
}

type personalTestRepo struct {
	t         *testing.T
	labs      map[string]sqlc.Labs
	profiles  map[string]map[int64]sqlc.LabProfiles
	submits   map[uuid.UUID]sqlc.Submissions
	scores    map[uuid.UUID][]sqlc.Scores
	keys      map[int64]sqlc.UserKeys
	nonceSeen map[string]struct{}
	private   ed25519.PrivateKey
	nextKeyID int64
}

func newPersonalTestRepo(t *testing.T, pick bool) *personalTestRepo {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	sshKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("NewPublicKey() error = %v", err)
	}

	labManifest := manifest.Manifest{
		Lab: manifest.LabSection{ID: "sorting", Name: "Sorting Lab"},
		Submit: manifest.SubmitSection{
			Files:   []string{"main.c"},
			MaxSize: "1MB",
		},
		Eval:  manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1", Timeout: 300},
		Quota: manifest.QuotaSection{Daily: 3},
		Metrics: []manifest.MetricSection{
			{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
			{ID: "latency", Name: "Latency", Sort: manifest.MetricSortAsc},
		},
		Board:    manifest.BoardSection{RankBy: "throughput", Pick: pick},
		Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
	}
	rawManifest, err := json.Marshal(labManifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	submissionA := uuid.MustParse("11111111-1111-7111-8111-111111111111")
	submissionB := uuid.MustParse("22222222-2222-7222-8222-222222222222")
	submissionC := uuid.MustParse("33333333-3333-7333-8333-333333333333")

	return &personalTestRepo{
		t: t,
		labs: map[string]sqlc.Labs{
			"sorting": {
				ID:       "sorting",
				Name:     "Sorting Lab",
				Manifest: rawManifest,
			},
		},
		profiles: map[string]map[int64]sqlc.LabProfiles{
			"sorting": {
				7: {UserID: 7, LabID: "sorting", Nickname: "Anonymous", Track: pgtype.Text{String: "throughput", Valid: true}},
			},
		},
		submits: map[uuid.UUID]sqlc.Submissions{
			submissionA: {
				ID:          submissionA,
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
				FinishedAt:  pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 12, 10, 0, 0, time.UTC), Valid: true},
				CreatedAt:   pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC), Valid: true},
			},
			submissionB: {
				ID:          submissionB,
				UserID:      7,
				LabID:       "sorting",
				Status:      "queued",
				QuotaState:  "pending",
				ContentHash: "hash-b",
				ArtifactKey: "sorting/7/b.tar.gz",
				CreatedAt:   pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 13, 0, 0, 0, time.UTC), Valid: true},
			},
			submissionC: {
				ID:          submissionC,
				UserID:      8,
				LabID:       "sorting",
				Status:      "done",
				QuotaState:  "charged",
				Verdict:     pgtype.Text{String: "scored", Valid: true},
				Message:     pgtype.Text{String: "other user", Valid: true},
				ContentHash: "hash-c",
				ArtifactKey: "sorting/8/c.tar.gz",
				FinishedAt:  pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 13, 20, 0, 0, time.UTC), Valid: true},
				CreatedAt:   pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 13, 10, 0, 0, time.UTC), Valid: true},
			},
		},
		scores: map[uuid.UUID][]sqlc.Scores{
			submissionA: {
				{SubmissionID: submissionA, MetricID: "latency", Value: 1.45},
				{SubmissionID: submissionA, MetricID: "throughput", Value: 1.82},
			},
		},
		keys: map[int64]sqlc.UserKeys{
			11: {ID: 11, UserID: 7, PublicKey: string(bytes.TrimSpace(ssh.MarshalAuthorizedKey(sshKey))), DeviceName: "laptop", CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC), Valid: true}},
			12: {ID: 12, UserID: 7, PublicKey: "ssh-ed25519 BBBB", DeviceName: "desktop", CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 3, 30, 11, 0, 0, 0, time.UTC), Valid: true}},
			21: {ID: 21, UserID: 8, PublicKey: "ssh-ed25519 CCCC", DeviceName: "tablet", CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC), Valid: true}},
		},
		nonceSeen: map[string]struct{}{},
		private:   privateKey,
		nextKeyID: 13,
	}
}

func (r *personalTestRepo) GetLab(_ context.Context, labID string) (sqlc.Labs, error) {
	row, ok := r.labs[labID]
	if !ok {
		return sqlc.Labs{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *personalTestRepo) ListSubmissionsByUserLab(_ context.Context, arg sqlc.ListSubmissionsByUserLabParams) ([]sqlc.Submissions, error) {
	rows := make([]sqlc.Submissions, 0)
	for _, row := range r.submits {
		if row.UserID == arg.UserID && row.LabID == arg.LabID {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.Time.After(rows[j].CreatedAt.Time)
	})
	return rows, nil
}

func (r *personalTestRepo) GetSubmission(_ context.Context, id uuid.UUID) (sqlc.Submissions, error) {
	row, ok := r.submits[id]
	if !ok {
		return sqlc.Submissions{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *personalTestRepo) ListScoresBySubmission(_ context.Context, submissionID uuid.UUID) ([]sqlc.Scores, error) {
	rows := append([]sqlc.Scores(nil), r.scores[submissionID]...)
	return rows, nil
}

func (r *personalTestRepo) GetUserKeyByFingerprint(_ context.Context, fingerprint string) (sqlc.UserKeys, error) {
	for _, row := range r.keys {
		if row.RevokedAt.Valid {
			continue
		}
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(row.PublicKey)))
		if err != nil {
			continue
		}
		cryptoKey, ok := publicKey.(ssh.CryptoPublicKey)
		if !ok {
			continue
		}
		edKey, ok := cryptoKey.CryptoPublicKey().(ed25519.PublicKey)
		if !ok {
			continue
		}
		if auth.PublicKeyFingerprint(edKey) == fingerprint {
			return row, nil
		}
	}
	return sqlc.UserKeys{}, pgx.ErrNoRows
}

func (r *personalTestRepo) ReserveNonce(_ context.Context, nonce string) (bool, error) {
	if _, ok := r.nonceSeen[nonce]; ok {
		return false, nil
	}
	r.nonceSeen[nonce] = struct{}{}
	return true, nil
}

func (r *personalTestRepo) UpsertLabProfileNickname(_ context.Context, arg sqlc.UpsertLabProfileNicknameParams) (sqlc.LabProfiles, error) {
	labProfiles := r.profiles[arg.LabID]
	row := labProfiles[arg.UserID]
	row.UserID = arg.UserID
	row.LabID = arg.LabID
	if arg.Nickname != "" {
		row.Nickname = arg.Nickname
	}
	labProfiles[arg.UserID] = row
	r.profiles[arg.LabID] = labProfiles
	return row, nil
}

func (r *personalTestRepo) UpsertLabProfileTrack(_ context.Context, arg sqlc.UpsertLabProfileTrackParams) (sqlc.LabProfiles, error) {
	labProfiles := r.profiles[arg.LabID]
	row := labProfiles[arg.UserID]
	row.UserID = arg.UserID
	row.LabID = arg.LabID
	if row.Nickname == "" {
		row.Nickname = "匿名"
	}
	if arg.Track.Valid {
		row.Track = arg.Track
	}
	labProfiles[arg.UserID] = row
	r.profiles[arg.LabID] = labProfiles
	return row, nil
}

func (r *personalTestRepo) ListUserKeys(_ context.Context, userID int64) ([]sqlc.UserKeys, error) {
	rows := make([]sqlc.UserKeys, 0, len(r.keys))
	for _, row := range r.keys {
		if row.UserID == userID && !row.RevokedAt.Valid {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (r *personalTestRepo) DeleteUserKey(_ context.Context, arg sqlc.DeleteUserKeyParams) error {
	row, ok := r.keys[arg.ID]
	if !ok || row.UserID != arg.UserID {
		return nil
	}
	row.RevokedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	r.keys[arg.ID] = row
	return nil
}

func (r *personalTestRepo) signRequest(t *testing.T, req *http.Request, path string, body []byte, nonce string, keyID int64) {
	t.Helper()
	now := time.Now().UTC()
	sum := sha256.Sum256(body)
	payload := auth.NewPayload(strings.ToUpper(req.Method)+" "+path, now, nonce, nil).WithContentHash(hex.EncodeToString(sum[:]))
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		t.Fatalf("SigningBytes() error = %v", err)
	}
	row, ok := r.keys[keyID]
	if !ok {
		t.Fatalf("missing key %d", keyID)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(row.PublicKey)))
	if err != nil {
		t.Fatalf("ParseAuthorizedKey() error = %v", err)
	}
	cryptoKey, ok := publicKey.(ssh.CryptoPublicKey)
	if !ok {
		t.Fatal("public key does not expose crypto key")
	}
	edKey, ok := cryptoKey.CryptoPublicKey().(ed25519.PublicKey)
	if !ok {
		t.Fatal("public key is not ed25519")
	}
	fingerprint := auth.PublicKeyFingerprint(edKey)
	req.Header.Set("X-LabKit-Key-Fingerprint", fingerprint)
	req.Header.Set("X-LabKit-Timestamp", now.Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", nonce)
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString(ed25519.Sign(r.private, signingBytes)))
}

var _ personal.Repository = (*personalTestRepo)(nil)
