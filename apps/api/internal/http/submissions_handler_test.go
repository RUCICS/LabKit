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
	err      error
}

func (s *capturingSubmissionsService) Intake(_ context.Context, input submissionsvc.SubmitInput) (submissionsvc.Submission, error) {
	s.input = input
	return s.response, s.err
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

var _ SubmissionsService = (*capturingSubmissionsService)(nil)
var _ = json.Valid
