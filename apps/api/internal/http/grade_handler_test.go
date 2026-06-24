package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gradesvc "labkit.local/apps/api/internal/service/grade"
)

type fakeGradeService struct {
	grade         gradesvc.Grade
	getErr        error
	importRes     gradesvc.ImportGradesResult
	importErr     error
	publishRes    gradesvc.PublishGradesResult
	deleteRes     gradesvc.DeleteGradesResult
	statusRes     gradesvc.GradeStatusResult
	lastLabID     string
	lastStudentID string
	lastImportCSV string
}

func (f *fakeGradeService) GetGrade(_ context.Context, labID, studentID string) (gradesvc.Grade, error) {
	f.lastLabID = labID
	f.lastStudentID = studentID
	if f.getErr != nil {
		return gradesvc.Grade{}, f.getErr
	}
	return f.grade, nil
}

func (f *fakeGradeService) ImportGrades(_ context.Context, labID string, r io.Reader) (gradesvc.ImportGradesResult, error) {
	body, _ := io.ReadAll(r)
	f.lastImportCSV = string(body)
	f.lastLabID = labID
	if f.importErr != nil {
		return gradesvc.ImportGradesResult{}, f.importErr
	}
	return f.importRes, nil
}

func (f *fakeGradeService) PublishGrades(_ context.Context, labID string) (gradesvc.PublishGradesResult, error) {
	f.lastLabID = labID
	return f.publishRes, nil
}

func (f *fakeGradeService) DeleteGrades(_ context.Context, labID string) (gradesvc.DeleteGradesResult, error) {
	f.lastLabID = labID
	return f.deleteRes, nil
}

func (f *fakeGradeService) GradeStatus(_ context.Context, labID string) (gradesvc.GradeStatusResult, error) {
	f.lastLabID = labID
	return f.statusRes, nil
}

func TestGradeRouteReturnsGradeForBrowserSession(t *testing.T) {
	svc := &fakeGradeService{grade: gradesvc.Grade{LabID: "colab-2026-p2", StudentID: "2026001", Total: 86.5}}
	router := NewRouter(WithGradeService(svc))

	token, err := issueWebBrowserSession(7, "2026001")
	if err != nil {
		t.Fatalf("issueWebBrowserSession() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/labs/colab-2026-p2/grade", nil)
	req.AddCookie(&http.Cookie{Name: browserSessionCookieName, Value: token})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.lastStudentID != "2026001" {
		t.Fatalf("queried student id = %q, want %q", svc.lastStudentID, "2026001")
	}
	if svc.lastLabID != "colab-2026-p2" {
		t.Fatalf("queried lab id = %q, want %q", svc.lastLabID, "colab-2026-p2")
	}
	var payload gradesvc.Grade
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal grade: %v", err)
	}
	if payload.Total != 86.5 {
		t.Fatalf("total = %v, want 86.5", payload.Total)
	}
}

func TestGradeRouteReturns404WhenUnpublished(t *testing.T) {
	svc := &fakeGradeService{getErr: gradesvc.ErrGradeNotFound}
	router := NewRouter(WithGradeService(svc))

	token, _ := issueWebBrowserSession(7, "2026001")
	req := httptest.NewRequest(http.MethodGet, "/api/labs/colab-2026-p2/grade", nil)
	req.AddCookie(&http.Cookie{Name: browserSessionCookieName, Value: token})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "grade_not_found") {
		t.Fatalf("body = %s, want grade_not_found", rr.Body.String())
	}
}

func TestGradeRouteRequiresAuth(t *testing.T) {
	svc := &fakeGradeService{}
	router := NewRouter(WithGradeService(svc))

	req := httptest.NewRequest(http.MethodGet, "/api/labs/colab-2026-p2/grade", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestAdminGradeImportRoute(t *testing.T) {
	svc := &fakeGradeService{importRes: gradesvc.ImportGradesResult{LabID: "colab-2026-p2", Imported: 3}}
	router := NewRouter(WithGradeService(svc), WithAdminToken("secret"))

	csv := "student_id,total\n2026001,80\n2026002,90\n2026003,70\n"
	req := httptest.NewRequest(http.MethodPost, "/api/admin/labs/colab-2026-p2/grades/import", strings.NewReader(csv))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "text/csv")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.lastImportCSV != csv {
		t.Fatalf("imported csv = %q, want %q", svc.lastImportCSV, csv)
	}
	var payload gradesvc.ImportGradesResult
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Imported != 3 {
		t.Fatalf("imported = %d, want 3", payload.Imported)
	}
}

func TestAdminGradeImportRequiresAdminToken(t *testing.T) {
	svc := &fakeGradeService{}
	router := NewRouter(WithGradeService(svc), WithAdminToken("secret"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/labs/colab-2026-p2/grades/import", strings.NewReader("student_id,total\n"))
	req.Header.Set("Content-Type", "text/csv")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestAdminGradeStatusRoute(t *testing.T) {
	svc := &fakeGradeService{statusRes: gradesvc.GradeStatusResult{LabID: "colab-2026-p2", Total: 30, Published: 12, Unpublished: 18}}
	router := NewRouter(WithGradeService(svc), WithAdminToken("secret"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/labs/colab-2026-p2/grades/status", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var payload gradesvc.GradeStatusResult
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Total != 30 || payload.Published != 12 || payload.Unpublished != 18 {
		t.Fatalf("payload = %+v, want total 30 / published 12 / unpublished 18", payload)
	}
}

func TestAdminGradeStatusRequiresAdminToken(t *testing.T) {
	router := NewRouter(WithGradeService(&fakeGradeService{}), WithAdminToken("secret"))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/labs/colab-2026-p2/grades/status", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestAdminGradeDeleteRoute(t *testing.T) {
	svc := &fakeGradeService{deleteRes: gradesvc.DeleteGradesResult{LabID: "colab-2026-p2", Deleted: 4}}
	router := NewRouter(WithGradeService(svc), WithAdminToken("secret"))

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/labs/colab-2026-p2/grades", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.lastLabID != "colab-2026-p2" {
		t.Fatalf("deleted lab id = %q, want %q", svc.lastLabID, "colab-2026-p2")
	}
	var payload gradesvc.DeleteGradesResult
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Deleted != 4 {
		t.Fatalf("deleted = %d, want 4", payload.Deleted)
	}
}

func TestAdminGradeDeleteRequiresAdminToken(t *testing.T) {
	svc := &fakeGradeService{}
	router := NewRouter(WithGradeService(svc), WithAdminToken("secret"))

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/labs/colab-2026-p2/grades", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestAdminGradePublishRoute(t *testing.T) {
	svc := &fakeGradeService{publishRes: gradesvc.PublishGradesResult{LabID: "colab-2026-p2", Published: 5}}
	router := NewRouter(WithGradeService(svc), WithAdminToken("secret"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/labs/colab-2026-p2/grades/publish", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.lastLabID != "colab-2026-p2" {
		t.Fatalf("published lab id = %q, want %q", svc.lastLabID, "colab-2026-p2")
	}
	var payload gradesvc.PublishGradesResult
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Published != 5 {
		t.Fatalf("published = %d, want 5", payload.Published)
	}
}
