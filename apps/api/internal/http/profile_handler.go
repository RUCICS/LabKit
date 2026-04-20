package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"labkit.local/apps/api/internal/http/middleware"
	personal "labkit.local/apps/api/internal/service/personal"
)

type ProfileHandler struct {
	Service PersonalService
}

type updateNicknameRequest struct {
	Nickname string `json:"nickname"`
}

type updateProfileRequest struct {
	Nickname string `json:"nickname"`
}

type updateTrackRequest struct {
	Track string `json:"track"`
}

func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	user, ok := authenticateBrowserSessionOrPersonalRequest(w, r, h.Service, nil)
	if !ok {
		return
	}
	profile, err := h.Service.GetProfile(r.Context(), user.UserID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	body, err := readRequestBody(r)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	user, ok := authenticateBrowserSessionOrPersonalRequest(w, r, h.Service, body)
	if !ok {
		return
	}
	var req updateProfileRequest
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	profile, err := h.Service.UpdateUserProfile(r.Context(), user.UserID, strings.TrimSpace(req.Nickname))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) UpdateNickname(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	body, err := readRequestBody(r)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	user, ok := authenticateBrowserSessionOrPersonalRequest(w, r, h.Service, body)
	if !ok {
		return
	}
	var req updateNicknameRequest
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	profile, err := h.Service.UpdateNickname(r.Context(), user.UserID, r.PathValue("labID"), strings.TrimSpace(req.Nickname))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) UpdateTrack(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	body, err := readRequestBody(r)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	user, ok := authenticateBrowserSessionOrPersonalRequest(w, r, h.Service, body)
	if !ok {
		return
	}
	var req updateTrackRequest
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	profile, err := h.Service.UpdateTrack(r.Context(), user.UserID, r.PathValue("labID"), strings.TrimSpace(req.Track))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, personal.ErrLabNotFound):
		middleware.WriteError(w, r, http.StatusNotFound, "lab_not_found", "Lab not found")
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
