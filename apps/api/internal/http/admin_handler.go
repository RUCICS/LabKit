package httpapi

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"labkit.local/apps/api/internal/http/middleware"
	adminsvc "labkit.local/apps/api/internal/service/admin"
)

type AdminService interface {
	ExportGrades(context.Context, string) (adminsvc.ExportGradesResult, error)
	Reevaluate(context.Context, string) (adminsvc.ReevaluateResult, error)
	QueueStatus(context.Context, string) (adminsvc.QueueStatus, error)
}

type AdminHandler struct {
	Service AdminService
}

func (h *AdminHandler) ExportGrades(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}

	result, err := h.Service.ExportGrades(r.Context(), r.PathValue("labID"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	h.writeCSVHeader(w, result.Filename)
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	_ = writer.Write(result.Columns)
	for _, row := range result.Rows {
		record := []string{
			row.StudentID,
			strconv.Itoa(row.Rank),
			row.Nickname,
			row.Track,
		}
		record = append(record, row.Scores...)
		_ = writer.Write(record)
	}
	writer.Flush()
}

func (h *AdminHandler) Reevaluate(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}

	result, err := h.Service.Reevaluate(r.Context(), r.PathValue("labID"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (h *AdminHandler) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}

	result, err := h.Service.QueueStatus(r.Context(), r.PathValue("labID"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, adminsvc.ErrLabNotFound):
		middleware.WriteError(w, r, http.StatusNotFound, "lab_not_found", "Lab not found")
	case errors.Is(err, adminsvc.ErrInvalidManifest):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_manifest", "Invalid manifest")
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}

func (h *AdminHandler) writeCSVHeader(w http.ResponseWriter, filename string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.TrimSpace(filename)+`"`)
}
