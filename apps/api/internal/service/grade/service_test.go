package grade

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeGradeRepo struct {
	rows         map[string]sqlc.FinalGrades // key: lab|student
	upsertCalls  int
	publishCalls int
}

func newFakeGradeRepo() *fakeGradeRepo {
	return &fakeGradeRepo{rows: make(map[string]sqlc.FinalGrades)}
}

func gradeKey(labID, studentID string) string { return labID + "|" + studentID }

func (r *fakeGradeRepo) UpsertFinalGrade(_ context.Context, arg sqlc.UpsertFinalGradeParams) (sqlc.FinalGrades, error) {
	r.upsertCalls++
	key := gradeKey(arg.LabID, arg.StudentID)
	existing := r.rows[key]
	row := sqlc.FinalGrades{
		LabID:       arg.LabID,
		StudentID:   arg.StudentID,
		Total:       arg.Total,
		Track:       arg.Track,
		Ratio:       arg.Ratio,
		PerfScore:   arg.PerfScore,
		Percentile:  arg.Percentile,
		BoardScore:  arg.BoardScore,
		Remark:      arg.Remark,
		PublishedAt: existing.PublishedAt, // upsert preserves visibility
		UpdatedAt:   pgtype.Timestamptz{Time: time.Unix(1700000000, 0).UTC(), Valid: true},
	}
	r.rows[key] = row
	return row, nil
}

func (r *fakeGradeRepo) GetFinalGrade(_ context.Context, arg sqlc.GetFinalGradeParams) (sqlc.FinalGrades, error) {
	row, ok := r.rows[gradeKey(arg.LabID, arg.StudentID)]
	if !ok || !row.PublishedAt.Valid {
		return sqlc.FinalGrades{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *fakeGradeRepo) PublishFinalGrades(_ context.Context, labID string) (int64, error) {
	r.publishCalls++
	var n int64
	for key, row := range r.rows {
		if row.LabID == labID && !row.PublishedAt.Valid {
			row.PublishedAt = pgtype.Timestamptz{Time: time.Unix(1700000000, 0).UTC(), Valid: true}
			r.rows[key] = row
			n++
		}
	}
	return n, nil
}

func (r *fakeGradeRepo) DeleteFinalGradesByLab(_ context.Context, labID string) (int64, error) {
	var n int64
	for key, row := range r.rows {
		if row.LabID == labID {
			delete(r.rows, key)
			n++
		}
	}
	return n, nil
}

func TestImportGradesParsesByHeaderName(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	csv := strings.Join([]string{
		"student_id,track,ratio,perf_score,percentile,board_score,total,remark",
		"2026001,throughput,1.2,85.0,0.9,14.0,86.5,looks good",
		"2026002,latency,1.0,70,0.5,10,72,",
	}, "\n")

	result, err := svc.ImportGrades(context.Background(), "colab-2026-p2", strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("imported = %d, want 2", result.Imported)
	}
	row := repo.rows[gradeKey("colab-2026-p2", "2026001")]
	if row.Total != 86.5 {
		t.Fatalf("total = %v, want 86.5", row.Total)
	}
	if !row.Track.Valid || row.Track.String != "throughput" {
		t.Fatalf("track = %+v, want throughput", row.Track)
	}
	if !row.PerfScore.Valid || row.PerfScore.Float32 != 85.0 {
		t.Fatalf("perf_score = %+v, want 85", row.PerfScore)
	}
	if !row.Remark.Valid || row.Remark.String != "looks good" {
		t.Fatalf("remark = %+v, want 'looks good'", row.Remark)
	}
	// Empty optional cells stay NULL.
	row2 := repo.rows[gradeKey("colab-2026-p2", "2026002")]
	if row2.Remark.Valid {
		t.Fatalf("remark should be NULL for empty cell, got %+v", row2.Remark)
	}
}

func TestImportGradesColumnOrderFlexibleAndExtraColumnsIgnored(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	csv := strings.Join([]string{
		"total,student_id,extra_col",
		"91.0,2026003,ignored",
	}, "\n")

	result, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("imported = %d, want 1", result.Imported)
	}
	row := repo.rows[gradeKey("lab", "2026003")]
	if row.Total != 91.0 {
		t.Fatalf("total = %v, want 91", row.Total)
	}
}

func TestImportGradesRejectsMissingRequiredColumns(t *testing.T) {
	svc := NewService(newFakeGradeRepo())
	_, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader("student_id,track\n2026001,x\n"))
	if !errors.Is(err, ErrInvalidCSV) {
		t.Fatalf("error = %v, want ErrInvalidCSV (missing total)", err)
	}
}

func TestImportGradesRejectsBadNumber(t *testing.T) {
	svc := NewService(newFakeGradeRepo())
	_, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader("student_id,total\n2026001,notnum\n"))
	if !errors.Is(err, ErrInvalidCSV) {
		t.Fatalf("error = %v, want ErrInvalidCSV", err)
	}
	if !strings.Contains(err.Error(), "row 2") {
		t.Fatalf("error should mention row number, got %v", err)
	}
}

func TestImportGradesSkipsBlankRowsAndStudentless(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	csv := "student_id,total\n2026001,80\n\n,55\n2026002,82\n"
	result, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("imported = %d, want 2 (blank + studentless skipped)", result.Imported)
	}
}

func TestGetGradeUnpublishedThenPublished(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	if _, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader("student_id,total\n2026001,88\n")); err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}

	// Unpublished → not found.
	if _, err := svc.GetGrade(context.Background(), "lab", "2026001"); !errors.Is(err, ErrGradeNotFound) {
		t.Fatalf("GetGrade() before publish error = %v, want ErrGradeNotFound", err)
	}

	publishRes, err := svc.PublishGrades(context.Background(), "lab")
	if err != nil {
		t.Fatalf("PublishGrades() error = %v", err)
	}
	if publishRes.Published != 1 {
		t.Fatalf("published = %d, want 1", publishRes.Published)
	}

	grade, err := svc.GetGrade(context.Background(), "lab", "2026001")
	if err != nil {
		t.Fatalf("GetGrade() after publish error = %v", err)
	}
	if grade.Total != 88 {
		t.Fatalf("total = %v, want 88", grade.Total)
	}
	if grade.PublishedAt == nil {
		t.Fatal("published_at should be set after publish")
	}
}

func TestGetGradeMissingStudent(t *testing.T) {
	svc := NewService(newFakeGradeRepo())
	if _, err := svc.GetGrade(context.Background(), "lab", "nobody"); !errors.Is(err, ErrGradeNotFound) {
		t.Fatalf("GetGrade() error = %v, want ErrGradeNotFound", err)
	}
}
