package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	personal "labkit.local/apps/api/internal/service/personal"
	submissionsvc "labkit.local/apps/api/internal/service/submissions"

	"github.com/google/uuid"
)

func TestSubmissionsHandlerCreateSubmissionParsesMultipartRequest(t *testing.T) {
	service := &capturingSubmissionsService{
		response: submissionsvc.Submission{
			ID:          uuid.MustParse("11111111-1111-7111-8111-111111111111"),
			Status:      "queued",
			ArtifactKey: "sorting/7/test.tar.gz",
			ContentHash: "hash",
		},
	}
	handler := &SubmissionsHandler{Service: service}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", "main.c")
	if err != nil {
		t.Fatalf("CreateFormFile(main.c): %v", err)
	}
	if _, err := part.Write([]byte("int main(void) { return 0; }\n")); err != nil {
		t.Fatalf("Write(main.c): %v", err)
	}
	part, err = writer.CreateFormFile("files", "README.md")
	if err != nil {
		t.Fatalf("CreateFormFile(README.md): %v", err)
	}
	if _, err := part.Write([]byte("# sorting\n")); err != nil {
		t.Fatalf("Write(README.md): %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/labs/sorting/submit", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-LabKit-Timestamp", "2026-03-31T12:00:00Z")
	req.Header.Set("X-LabKit-Nonce", "nonce-http")
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString([]byte("signature")))
	req.Header.Set("X-LabKit-Key-Fingerprint", "SHA256:test-fingerprint")
	req.SetPathValue("labID", "sorting")

	rr := httptest.NewRecorder()
	handler.CreateSubmission(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if service.input.LabID != "sorting" {
		t.Fatalf("LabID = %q, want sorting", service.input.LabID)
	}
	if service.input.KeyFingerprint != "SHA256:test-fingerprint" {
		t.Fatalf("KeyFingerprint = %q, want test fingerprint", service.input.KeyFingerprint)
	}
	if service.input.Nonce != "nonce-http" {
		t.Fatalf("Nonce = %q, want nonce-http", service.input.Nonce)
	}
	if len(service.input.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(service.input.Files))
	}
	if service.input.Files[0].Name != "main.c" {
		t.Fatalf("first file name = %q, want main.c", service.input.Files[0].Name)
	}
	if string(service.input.Files[1].Content) != "# sorting\n" {
		t.Fatalf("second file content = %q, want %q", string(service.input.Files[1].Content), "# sorting\n")
	}
}

func TestSubmissionsHandlerMapsOversizeSubmissionToBadRequest(t *testing.T) {
	handler := &SubmissionsHandler{}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/labs/sorting/submit", nil)

	handler.writeError(rr, req, submissionsvc.ErrSubmissionTooLarge)

	assertStructuredErrorResponse(t, rr, http.StatusBadRequest, "", "invalid_submission", submissionsvc.ErrSubmissionTooLarge.Error())
}

type capturingSubmissionsService struct {
	input    submissionsvc.SubmitInput
	response submissionsvc.Submission
	precheck submissionsvc.SubmitPrecheck
	err      error
}

func (s *capturingSubmissionsService) Intake(_ context.Context, input submissionsvc.SubmitInput) (submissionsvc.Submission, error) {
	s.input = input
	return s.response, s.err
}

func (s *capturingSubmissionsService) GetSubmitPrecheck(_ context.Context, _ int64, _ string) (submissionsvc.SubmitPrecheck, error) {
	return s.precheck, s.err
}

func TestSubmissionsHandlerCreateSubmissionMapsServiceErrors(t *testing.T) {
	service := &capturingSubmissionsService{err: submissionsvc.ErrSubmissionTooLarge}
	handler := &SubmissionsHandler{Service: service}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", "main.c")
	if err != nil {
		t.Fatalf("CreateFormFile(main.c): %v", err)
	}
	if _, err := part.Write([]byte("int main(void) { return 0; }\n")); err != nil {
		t.Fatalf("Write(main.c): %v", err)
	}
	part, err = writer.CreateFormFile("files", "README.md")
	if err != nil {
		t.Fatalf("CreateFormFile(README.md): %v", err)
	}
	if _, err := part.Write([]byte("# sorting\n")); err != nil {
		t.Fatalf("Write(README.md): %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/labs/sorting/submit", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-LabKit-Timestamp", time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", "nonce-http-error")
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString([]byte("signature")))
	req.Header.Set("X-LabKit-Key-Fingerprint", "SHA256:test-fingerprint")
	req.SetPathValue("labID", "sorting")

	rr := httptest.NewRecorder()
	handler.CreateSubmission(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusBadRequest, "", "invalid_submission", submissionsvc.ErrSubmissionTooLarge.Error())
}

func TestSubmissionsHandlerCreateSubmissionMapsDailyQuotaExceeded(t *testing.T) {
	service := &capturingSubmissionsService{
		err: submissionsvc.NewDailyQuotaExceededError(3, 3, mustLoadHTTPTestLocation(t, "Asia/Shanghai")),
	}
	handler := &SubmissionsHandler{Service: service}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", "main.c")
	if err != nil {
		t.Fatalf("CreateFormFile(main.c): %v", err)
	}
	if _, err := part.Write([]byte("int main(void) { return 0; }\n")); err != nil {
		t.Fatalf("Write(main.c): %v", err)
	}
	part, err = writer.CreateFormFile("files", "README.md")
	if err != nil {
		t.Fatalf("CreateFormFile(README.md): %v", err)
	}
	if _, err := part.Write([]byte("# sorting\n")); err != nil {
		t.Fatalf("Write(README.md): %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/labs/sorting/submit", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-LabKit-Timestamp", time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", "nonce-http-quota")
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString([]byte("signature")))
	req.Header.Set("X-LabKit-Key-Fingerprint", "SHA256:test-fingerprint")
	req.SetPathValue("labID", "sorting")

	rr := httptest.NewRecorder()
	handler.CreateSubmission(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusTooManyRequests, "", "daily_quota_exceeded", "Daily submission limit reached (3/3). Quota resets at 00:00 Asia/Shanghai.")
}

func TestSubmissionsHandlerReturnsSubmitPrecheck(t *testing.T) {
	service := &capturingSubmissionsService{
		precheck: submissionsvc.SubmitPrecheck{
			Quota: &submissionsvc.QuotaSummary{Daily: 3, Used: 1, Left: 2, ResetHint: "00:00 Asia/Shanghai"},
			LatestSubmission: &submissionsvc.LatestSubmissionHint{
				ContentHash: "hash-latest",
				CreatedAt:   time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC),
			},
		},
	}
	handler := &SubmissionsHandler{
		Service:  service,
		Personal: submitPrecheckPersonalService{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/labs/sorting/submit/precheck", nil)
	req.SetPathValue("labID", "sorting")
	req.Header.Set("X-LabKit-Key-Fingerprint", "SHA256:test-fingerprint")
	req.Header.Set("X-LabKit-Timestamp", time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", "nonce-precheck")
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString([]byte("signature")))
	rr := httptest.NewRecorder()

	handler.GetSubmitPrecheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var payload submissionsvc.SubmitPrecheck
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Quota == nil || payload.Quota.Left != 2 {
		t.Fatalf("quota = %#v, want left=2", payload.Quota)
	}
	if payload.LatestSubmission == nil || payload.LatestSubmission.ContentHash != "hash-latest" {
		t.Fatalf("latest submission = %#v, want hash-latest", payload.LatestSubmission)
	}
}

func TestSubmissionsHandlerMapsSubmitPrecheckLabNotFound(t *testing.T) {
	service := &capturingSubmissionsService{err: submissionsvc.ErrLabNotFound}
	handler := &SubmissionsHandler{
		Service:  service,
		Personal: submitPrecheckPersonalService{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/labs/missing/submit/precheck", nil)
	req.SetPathValue("labID", "missing")
	req.Header.Set("X-LabKit-Key-Fingerprint", "SHA256:test-fingerprint")
	req.Header.Set("X-LabKit-Timestamp", time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", "nonce-precheck-missing")
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString([]byte("signature")))

	rr := httptest.NewRecorder()
	handler.GetSubmitPrecheck(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusNotFound, "", "lab_not_found", "Lab not found")
}

var _ SubmissionsService = (*capturingSubmissionsService)(nil)
var _ = json.Valid

func mustLoadHTTPTestLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q) error = %v", name, err)
	}
	return location
}

type submitPrecheckPersonalService struct{}

func (submitPrecheckPersonalService) Authenticate(context.Context, personal.AuthInput) (personal.AuthenticatedUser, error) {
	return personal.AuthenticatedUser{UserID: 7, KeyID: 11}, nil
}

func (submitPrecheckPersonalService) GetProfile(context.Context, int64) (personal.UserProfile, error) {
	return personal.UserProfile{}, nil
}

func (submitPrecheckPersonalService) UpdateUserProfile(context.Context, int64, string) (personal.UserProfile, error) {
	return personal.UserProfile{}, nil
}

func (submitPrecheckPersonalService) ListSubmissionHistory(context.Context, int64, string) (personal.HistoryResponse, error) {
	return personal.HistoryResponse{}, nil
}

func (submitPrecheckPersonalService) GetSubmissionDetail(context.Context, int64, string, uuid.UUID) (personal.SubmissionDetail, error) {
	return personal.SubmissionDetail{}, nil
}

func (submitPrecheckPersonalService) UpdateNickname(context.Context, int64, string, string) (personal.Profile, error) {
	return personal.Profile{}, nil
}

func (submitPrecheckPersonalService) UpdateTrack(context.Context, int64, string, string) (personal.Profile, error) {
	return personal.Profile{}, nil
}

func (submitPrecheckPersonalService) ListKeys(context.Context, int64) ([]personal.Key, error) {
	return nil, nil
}

func (submitPrecheckPersonalService) RevokeKey(context.Context, int64, int64) error {
	return nil
}
