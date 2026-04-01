package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"labkit.local/apps/api/internal/http/middleware"
	boardsvc "labkit.local/apps/api/internal/service/leaderboard"
)

type LeaderboardService interface {
	GetBoard(context.Context, string, string, int64) (boardsvc.Board, error)
}

type LeaderboardHandler struct {
	Service  LeaderboardService
	Personal PersonalService
}

func (h *LeaderboardHandler) GetBoard(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}

	var viewerUserID int64
	if strings.TrimSpace(r.Header.Get("X-LabKit-Key-Fingerprint")) != "" && h.Personal != nil {
		user, ok := authenticatePersonalRequest(w, r, h.Personal, nil)
		if !ok {
			return
		}
		viewerUserID = user.UserID
	}

	board, err := h.Service.GetBoard(r.Context(), r.PathValue("labID"), r.URL.Query().Get("by"), viewerUserID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, board)
}

func (h *LeaderboardHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, boardsvc.ErrLabNotFound):
		middleware.WriteError(w, r, http.StatusNotFound, "lab_not_found", "Lab not found")
	case errors.Is(err, boardsvc.ErrLabHidden):
		middleware.WriteError(w, r, http.StatusNotFound, "lab_hidden", "Leaderboard is hidden")
	case errors.Is(err, boardsvc.ErrMetricNotFound):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_metric", "Invalid metric")
	case errors.Is(err, boardsvc.ErrInvalidManifest):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_manifest", "Invalid manifest")
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}
