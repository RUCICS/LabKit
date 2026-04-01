package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"labkit.local/apps/api/internal/http/middleware"
	labsvc "labkit.local/apps/api/internal/service/labs"
)

type LabsService interface {
	RegisterLab(context.Context, []byte) (labsvc.Lab, error)
	UpdateLab(context.Context, string, []byte) (labsvc.Lab, error)
	ListPublicLabs(context.Context) ([]labsvc.Lab, error)
	GetPublicLab(context.Context, string) (labsvc.Lab, error)
}

type LabsHandler struct {
	Service LabsService
}

func (h *LabsHandler) RegisterLab(w http.ResponseWriter, r *http.Request) {
	rawManifest, ok := h.readManifest(w, r)
	if !ok {
		return
	}
	lab, err := h.Service.RegisterLab(r.Context(), rawManifest)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, lab)
}

func (h *LabsHandler) UpdateLab(w http.ResponseWriter, r *http.Request) {
	rawManifest, ok := h.readManifest(w, r)
	if !ok {
		return
	}

	lab, err := h.Service.UpdateLab(r.Context(), r.PathValue("labID"), rawManifest)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, lab)
}

func (h *LabsHandler) ListLabs(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}

	labs, err := h.Service.ListPublicLabs(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"labs": labs})
}

func (h *LabsHandler) GetLab(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}

	lab, err := h.Service.GetPublicLab(r.Context(), r.PathValue("labID"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, lab)
}

func (h *LabsHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, labsvc.ErrLabNotFound):
		middleware.WriteError(w, r, http.StatusNotFound, "lab_not_found", "Lab not found")
	case errors.Is(err, labsvc.ErrLabAlreadyExists):
		middleware.WriteError(w, r, http.StatusConflict, "lab_already_exists", "Lab already exists")
	case errors.Is(err, labsvc.ErrInvalidManifest):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_manifest", err.Error())
	case errors.Is(err, labsvc.ErrStructuralUpdate):
		middleware.WriteError(w, r, http.StatusConflict, "structural_update_rejected", "Structural lab updates are not allowed")
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}

func (h *LabsHandler) readManifest(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return nil, false
	}

	rawManifest, err := io.ReadAll(r.Body)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_manifest", "invalid manifest")
		return nil, false
	}
	return rawManifest, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
