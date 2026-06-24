package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"labkit.local/apps/api/internal/http/middleware"
	gradesvc "labkit.local/apps/api/internal/service/grade"
)

// GradeService is the grade surface used by the HTTP layer (student read +
// admin import/publish).
type GradeService interface {
	GetGrade(context.Context, string, string) (gradesvc.Grade, error)
	ImportGrades(context.Context, string, io.Reader) (gradesvc.ImportGradesResult, error)
	PublishGrades(context.Context, string) (gradesvc.PublishGradesResult, error)
}

// GradeHandler serves the student-facing grade view and the admin grade import.
type GradeHandler struct {
	Service  GradeService
	Personal PersonalService
}

// maxGradeCSVBytes caps an uploaded grades CSV (defensive; class-sized files
// are tiny).
const maxGradeCSVBytes = 8 << 20 // 8 MiB

// GetMyGrade handles GET /api/labs/{labID}/grade. Browser session is preferred
// (it already carries the student id); CLI signature auth is also accepted.
func (h *GradeHandler) GetMyGrade(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	studentID, ok := h.resolveStudentID(w, r)
	if !ok {
		return
	}
	grade, err := h.Service.GetGrade(r.Context(), r.PathValue("labID"), studentID)
	if err != nil {
		switch {
		case errors.Is(err, gradesvc.ErrGradeNotFound):
			middleware.WriteError(w, r, http.StatusNotFound, "grade_not_found", "成绩尚未发布")
		default:
			middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		}
		return
	}
	writeJSON(w, http.StatusOK, grade)
}

// ImportGrades handles POST /api/admin/labs/{labID}/grades/import. The CSV may
// arrive as a multipart "file" field or as a raw text/csv body.
func (h *GradeHandler) ImportGrades(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	reader, cleanup, err := gradeCSVReader(r)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	defer cleanup()

	result, err := h.Service.ImportGrades(r.Context(), r.PathValue("labID"), reader)
	if err != nil {
		h.writeImportError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PublishGrades handles POST /api/admin/labs/{labID}/grades/publish.
func (h *GradeHandler) PublishGrades(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	result, err := h.Service.PublishGrades(r.Context(), r.PathValue("labID"))
	if err != nil {
		if errors.Is(err, gradesvc.ErrInvalidLab) {
			middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "lab id is required")
			return
		}
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *GradeHandler) writeImportError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gradesvc.ErrInvalidCSV):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_csv", err.Error())
	case errors.Is(err, gradesvc.ErrInvalidLab):
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "lab id is required")
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}

// resolveStudentID returns the requesting student's id from a browser session
// (preferred) or from CLI signature auth (reverse-lookup via profile).
func (h *GradeHandler) resolveStudentID(w http.ResponseWriter, r *http.Request) (string, bool) {
	if session, ok := browserSessionFromRequest(r); ok {
		if sid := strings.TrimSpace(session.StudentID); sid != "" {
			return sid, true
		}
		if h.Personal != nil {
			if profile, err := h.Personal.GetProfile(r.Context(), session.UserID); err == nil {
				if sid := strings.TrimSpace(profile.StudentID); sid != "" {
					return sid, true
				}
			}
		}
		middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
		return "", false
	}

	if h.Personal == nil {
		middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
		return "", false
	}
	body, err := readRequestBody(r)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
		return "", false
	}
	user, ok := authenticatePersonalRequest(w, r, h.Personal, body)
	if !ok {
		return "", false
	}
	profile, err := h.Personal.GetProfile(r.Context(), user.UserID)
	if err != nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return "", false
	}
	sid := strings.TrimSpace(profile.StudentID)
	if sid == "" {
		middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
		return "", false
	}
	return sid, true
}

// gradeCSVReader extracts the CSV body from either a multipart upload or a raw
// request body, returning a reader and a cleanup func.
func gradeCSVReader(r *http.Request) (io.Reader, func(), error) {
	noop := func() {}
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxGradeCSVBytes); err != nil {
			return nil, noop, errors.New("failed to parse multipart upload")
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, noop, errors.New("missing CSV file field \"file\"")
		}
		return io.LimitReader(file, maxGradeCSVBytes), func() { _ = file.Close() }, nil
	}
	if r.Body == nil {
		return nil, noop, errors.New("empty request body")
	}
	return io.LimitReader(r.Body, maxGradeCSVBytes), noop, nil
}
