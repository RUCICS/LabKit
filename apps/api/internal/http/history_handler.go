package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"labkit.local/apps/api/internal/http/middleware"
	personal "labkit.local/apps/api/internal/service/personal"

	"github.com/google/uuid"
)

type PersonalService interface {
	Authenticate(context.Context, personal.AuthInput) (personal.AuthenticatedUser, error)
	ListSubmissionHistory(context.Context, int64, string) ([]personal.HistoryItem, error)
	GetSubmissionDetail(context.Context, int64, string, uuid.UUID) (personal.SubmissionDetail, error)
	UpdateNickname(context.Context, int64, string, string) (personal.Profile, error)
	UpdateTrack(context.Context, int64, string, string) (personal.Profile, error)
	ListKeys(context.Context, int64) ([]personal.Key, error)
	RevokeKey(context.Context, int64, int64) error
}

type HistoryHandler struct {
	Service PersonalService
}

func (h *HistoryHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	user, ok := authenticatePersonalRequest(w, r, h.Service, nil)
	if !ok {
		return
	}

	items, err := h.Service.ListSubmissionHistory(r.Context(), user.UserID, r.PathValue("labID"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"submissions": items})
}

func (h *HistoryHandler) GetSubmissionDetail(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	user, ok := authenticatePersonalRequest(w, r, h.Service, nil)
	if !ok {
		return
	}
	submissionID, err := uuid.Parse(strings.TrimSpace(r.PathValue("submissionID")))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid submission id")
		return
	}

	detail, err := h.Service.GetSubmissionDetail(r.Context(), user.UserID, r.PathValue("labID"), submissionID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *HistoryHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, personal.ErrLabNotFound):
		middleware.WriteError(w, r, http.StatusNotFound, "lab_not_found", "Lab not found")
	case errors.Is(err, personal.ErrSubmissionNotFound):
		middleware.WriteError(w, r, http.StatusNotFound, "submission_not_found", "Submission not found")
	case errors.Is(err, personal.ErrInvalidNickname):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_nickname", err.Error())
	case errors.Is(err, personal.ErrInvalidTrack):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_track", err.Error())
	case errors.Is(err, personal.ErrTrackSelectionDisabled):
		middleware.WriteError(w, r, http.StatusBadRequest, "track_not_allowed", "Track selection is disabled")
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}
