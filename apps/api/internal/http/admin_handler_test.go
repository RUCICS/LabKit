package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminsvc "labkit.local/apps/api/internal/service/admin"
)

func TestAdminRoutesExportGradesAsCSV(t *testing.T) {
	router := NewRouter(
		WithAdminToken("secret"),
		WithAdminService(&fakeAdminService{
			export: adminsvc.ExportGradesResult{
				LabID:    "sorting",
				Filename: "sorting-grades.csv",
				Columns:  []string{"student_id", "rank", "nickname", "track", "runtime_ms", "latency_ms"},
				Rows: []adminsvc.ExportGradesRow{
					{
						StudentID: "20260001",
						Rank:      1,
						Nickname:  "Ada",
						Track:     "throughput",
						Scores:    []string{"120", "31"},
					},
					{
						StudentID: "20260002",
						Rank:      2,
						Nickname:  "Bob",
						Track:     "latency",
						Scores:    []string{"90", "44"},
					},
				},
			},
		}),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/labs/sorting/grades", nil)
	req.SetPathValue("labID", "sorting")
	req.Header.Set("Authorization", "Bearer secret")

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("content type = %q, want %q", got, "text/csv; charset=utf-8")
	}
	if got := rr.Header().Get("Content-Disposition"); !strings.Contains(got, `attachment; filename="sorting-grades.csv"`) {
		t.Fatalf("content disposition = %q, want attachment filename", got)
	}
	want := "student_id,rank,nickname,track,runtime_ms,latency_ms\n20260001,1,Ada,throughput,120,31\n20260002,2,Bob,latency,90,44\n"
	if got := rr.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestAdminRoutesReevaluateAndQueueStatus(t *testing.T) {
	router := NewRouter(
		WithAdminToken("secret"),
		WithAdminService(&fakeAdminService{
			reeval: adminsvc.ReevaluateResult{LabID: "sorting", JobsCreated: 2},
			queue: adminsvc.QueueStatus{
				LabID: "sorting",
				Jobs: []adminsvc.QueueJob{
					{ID: "job-1", SubmissionID: "sub-1", UserID: 7, Status: "running", Attempts: 1, CreatedAt: time.Now().UTC()},
					{ID: "job-2", SubmissionID: "sub-2", UserID: 8, Status: "queued", Attempts: 0, CreatedAt: time.Now().UTC()},
				},
			},
		}),
	)

	reevalRR := httptest.NewRecorder()
	reevalReq := httptest.NewRequest(http.MethodPost, "/api/admin/labs/sorting/reeval", nil)
	reevalReq.SetPathValue("labID", "sorting")
	reevalReq.Header.Set("Authorization", "Bearer secret")
	router.ServeHTTP(reevalRR, reevalReq)

	if reevalRR.Code != http.StatusAccepted {
		t.Fatalf("reeval status = %d, want %d body=%s", reevalRR.Code, http.StatusAccepted, reevalRR.Body.String())
	}

	var reevalPayload adminsvc.ReevaluateResult
	if err := json.Unmarshal(reevalRR.Body.Bytes(), &reevalPayload); err != nil {
		t.Fatalf("unmarshal reeval response: %v", err)
	}
	if reevalPayload.JobsCreated != 2 {
		t.Fatalf("jobs_created = %d, want 2", reevalPayload.JobsCreated)
	}

	queueRR := httptest.NewRecorder()
	queueReq := httptest.NewRequest(http.MethodGet, "/api/admin/labs/sorting/queue", nil)
	queueReq.SetPathValue("labID", "sorting")
	queueReq.Header.Set("Authorization", "Bearer secret")
	router.ServeHTTP(queueRR, queueReq)

	if queueRR.Code != http.StatusOK {
		t.Fatalf("queue status = %d, want %d body=%s", queueRR.Code, http.StatusOK, queueRR.Body.String())
	}

	var queuePayload adminsvc.QueueStatus
	if err := json.Unmarshal(queueRR.Body.Bytes(), &queuePayload); err != nil {
		t.Fatalf("unmarshal queue response: %v", err)
	}
	if len(queuePayload.Jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(queuePayload.Jobs))
	}
}

func TestAdminRoutesRejectMissingAuth(t *testing.T) {
	router := NewRouter(WithAdminToken("secret"), WithAdminService(&fakeAdminService{}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/labs/sorting/grades", nil)
	req.SetPathValue("labID", "sorting")

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

type fakeAdminService struct {
	export adminsvc.ExportGradesResult
	reeval adminsvc.ReevaluateResult
	queue  adminsvc.QueueStatus
}

func (f *fakeAdminService) ExportGrades(context.Context, string) (adminsvc.ExportGradesResult, error) {
	return f.export, nil
}

func (f *fakeAdminService) Reevaluate(context.Context, string) (adminsvc.ReevaluateResult, error) {
	return f.reeval, nil
}

func (f *fakeAdminService) QueueStatus(context.Context, string) (adminsvc.QueueStatus, error) {
	return f.queue, nil
}
