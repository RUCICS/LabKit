package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"labkit.local/apps/api/internal/http/middleware"
	personal "labkit.local/apps/api/internal/service/personal"
)

type KeysHandler struct {
	Service PersonalService
}

func (h *KeysHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	user, ok := authenticateBrowserSessionRequest(r)
	if !ok {
		user, ok = authenticatePersonalRequest(w, r, h.Service, nil)
	}
	if !ok {
		return
	}

	keys, err := h.Service.ListKeys(r.Context(), user.UserID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (h *KeysHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	user, ok := authenticatePersonalRequest(w, r, h.Service, nil)
	if !ok {
		return
	}
	keyID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("keyID")), 10, 64)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid key id")
		return
	}

	if err := h.Service.RevokeKey(r.Context(), user.UserID, keyID); err != nil {
		switch {
		case errors.Is(err, personal.ErrLabNotFound):
			middleware.WriteError(w, r, http.StatusNotFound, "lab_not_found", "Lab not found")
		default:
			middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *KeysHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, personal.ErrLabNotFound):
		middleware.WriteError(w, r, http.StatusNotFound, "lab_not_found", "Lab not found")
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}
