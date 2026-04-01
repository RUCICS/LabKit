package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"labkit.local/apps/api/internal/http/middleware"
	submissionsvc "labkit.local/apps/api/internal/service/submissions"
	"labkit.local/packages/go/auth"
)

type SubmissionsService interface {
	Intake(context.Context, submissionsvc.SubmitInput) (submissionsvc.Submission, error)
}

type SubmissionsHandler struct {
	Service SubmissionsService
}

func (h *SubmissionsHandler) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_submission", "invalid multipart request")
		return
	}

	keyFingerprint := strings.TrimSpace(r.Header.Get("X-LabKit-Key-Fingerprint"))
	if keyFingerprint == "" {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_submission", "invalid key fingerprint")
		return
	}
	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(r.Header.Get("X-LabKit-Timestamp")))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_submission", "invalid timestamp")
		return
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(r.Header.Get("X-LabKit-Signature")))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_submission", "invalid signature encoding")
		return
	}

	files := make([]submissionsvc.UploadFile, 0, len(r.MultipartForm.File))
	for _, entries := range r.MultipartForm.File {
		for _, entry := range entries {
			handle, err := entry.Open()
			if err != nil {
				middleware.WriteError(w, r, http.StatusBadRequest, "invalid_submission", "invalid uploaded file")
				return
			}
			content, readErr := io.ReadAll(handle)
			_ = handle.Close()
			if readErr != nil {
				middleware.WriteError(w, r, http.StatusBadRequest, "invalid_submission", "invalid uploaded file")
				return
			}
			files = append(files, submissionsvc.UploadFile{
				Name:    entry.Filename,
				Content: content,
			})
		}
	}

	submission, err := h.Service.Intake(r.Context(), submissionsvc.SubmitInput{
		LabID:          r.PathValue("labID"),
		KeyFingerprint: keyFingerprint,
		Timestamp:      timestamp,
		Nonce:          strings.TrimSpace(r.Header.Get("X-LabKit-Nonce")),
		Signature:      signature,
		Files:          files,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, submission)
}

func (h *SubmissionsHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, submissionsvc.ErrMissingFiles), errors.Is(err, submissionsvc.ErrInvalidSubmission), errors.Is(err, submissionsvc.ErrSubmissionTooLarge):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_submission", err.Error())
	case errors.Is(err, submissionsvc.ErrInvalidSignature), errors.Is(err, submissionsvc.ErrSubmissionKey), errors.Is(err, auth.ErrNonceReplayed):
		middleware.WriteError(w, r, http.StatusUnauthorized, "invalid_signature", "Invalid signature")
	case errors.Is(err, submissionsvc.ErrLabClosed):
		middleware.WriteError(w, r, http.StatusConflict, "lab_closed", "Lab is closed")
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}
