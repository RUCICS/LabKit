package grade

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeGradeRepo struct {
	rows map[string]sqlc.FinalGrades
}

func newFakeGradeRepo() *fakeGradeRepo {
	return &fakeGradeRepo{rows: make(map[string]sqlc.FinalGrades)}
}

func gradeKey(labID, studentID string) string { return labID + "|" + studentID }

func (r *fakeGradeRepo) UpsertFinalGrade(_ context.Context, arg sqlc.UpsertFinalGradeParams) (sqlc.FinalGrades, error) {
	key := gradeKey(arg.LabID, arg.StudentID)
	existing := r.rows[key]
	row := sqlc.FinalGrades{
		LabID:       arg.LabID,
		StudentID:   arg.StudentID,
		Total:       arg.Total,
		Remark:      arg.Remark,
		Items:       arg.Items,
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

func (r *fakeGradeRepo) ListPublishedFinalGradesByStudent(_ context.Context, studentID string) ([]sqlc.FinalGrades, error) {
	var out []sqlc.FinalGrades
	for _, row := range r.rows {
		if row.StudentID == studentID && row.PublishedAt.Valid {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *fakeGradeRepo) PublishFinalGrades(_ context.Context, labID string) (int64, error) {
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

func (r *fakeGradeRepo) SummarizeFinalGrades(_ context.Context, labID string) (sqlc.SummarizeFinalGradesRow, error) {
	var out sqlc.SummarizeFinalGradesRow
	for _, row := range r.rows {
		if row.LabID != labID {
			continue
		}
		out.Total++
		if row.PublishedAt.Valid {
			out.Published++
		}
	}
	return out, nil
}

func itemsOf(t *testing.T, row sqlc.FinalGrades) []GradeItem {
	t.Helper()
	var items []GradeItem
	if len(row.Items) > 0 {
		if err := json.Unmarshal(row.Items, &items); err != nil {
			t.Fatalf("unmarshal items: %v", err)
		}
	}
	return items
}

func TestImportGradesMapsTotalRemarkAndFreeFormItems(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	csv := strings.Join([]string{
		"student_id,total,赛道,性能分(85%),打榜分(15%),remark",
		"2026001,86.5,throughput,85,14,复核无误",
		"2026002,72,latency,70,10,",
	}, "\n")

	result, err := svc.ImportGrades(context.Background(), "colab-2026-p2", strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}
	if result.Imported != 2 {
		t.Fatalf("imported = %d, want 2", result.Imported)
	}

	row := repo.rows[gradeKey("colab-2026-p2", "2026001")]
	if !row.Total.Valid || row.Total.String != "86.5" {
		t.Fatalf("total = %+v, want 86.5", row.Total)
	}
	if !row.Remark.Valid || row.Remark.String != "复核无误" {
		t.Fatalf("remark = %+v, want 复核无误", row.Remark)
	}
	items := itemsOf(t, row)
	want := []GradeItem{
		{Label: "赛道", Value: "throughput"},
		{Label: "性能分(85%)", Value: "85"},
		{Label: "打榜分(15%)", Value: "14"},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %+v, want %+v", items, want)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Fatalf("items[%d] = %+v, want %+v", i, items[i], want[i])
		}
	}

	// Empty optional cells are omitted from items and from remark.
	row2 := repo.rows[gradeKey("colab-2026-p2", "2026002")]
	if row2.Remark.Valid {
		t.Fatalf("remark should be NULL for empty cell, got %+v", row2.Remark)
	}
}

func TestImportGradesTotalIsOptional(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	csv := "student_id,赛道\n2026001,throughput\n"

	result, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("imported = %d, want 1", result.Imported)
	}
	row := repo.rows[gradeKey("lab", "2026001")]
	if row.Total.Valid {
		t.Fatalf("total should be NULL when absent, got %+v", row.Total)
	}
	items := itemsOf(t, row)
	if len(items) != 1 || items[0] != (GradeItem{Label: "赛道", Value: "throughput"}) {
		t.Fatalf("items = %+v, want [赛道=throughput]", items)
	}
}

func TestImportGradesAcceptsChineseTotalAlias(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	csv := "student_id,总评,备注\n2026001,90,ok\n"

	if _, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader(csv)); err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}
	row := repo.rows[gradeKey("lab", "2026001")]
	if !row.Total.Valid || row.Total.String != "90" {
		t.Fatalf("总评 should map to total, got %+v", row.Total)
	}
	if !row.Remark.Valid || row.Remark.String != "ok" {
		t.Fatalf("备注 should map to remark, got %+v", row.Remark)
	}
	if len(itemsOf(t, row)) != 0 {
		t.Fatalf("expected no free-form items, got %+v", itemsOf(t, row))
	}
}

func TestImportGradesRejectsMissingStudentID(t *testing.T) {
	svc := NewService(newFakeGradeRepo())
	_, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader("name,total\nAda,90\n"))
	if !errors.Is(err, ErrInvalidCSV) {
		t.Fatalf("error = %v, want ErrInvalidCSV (missing student_id)", err)
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
	if _, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader("student_id,total,赛道\n2026001,88,throughput\n")); err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}

	if _, err := svc.GetGrade(context.Background(), "lab", "2026001"); !errors.Is(err, ErrGradeNotFound) {
		t.Fatalf("GetGrade() before publish error = %v, want ErrGradeNotFound", err)
	}

	if _, err := svc.PublishGrades(context.Background(), "lab"); err != nil {
		t.Fatalf("PublishGrades() error = %v", err)
	}

	grade, err := svc.GetGrade(context.Background(), "lab", "2026001")
	if err != nil {
		t.Fatalf("GetGrade() after publish error = %v", err)
	}
	if grade.Total != "88" {
		t.Fatalf("total = %q, want 88", grade.Total)
	}
	if len(grade.Items) != 1 || grade.Items[0].Label != "赛道" {
		t.Fatalf("items = %+v, want [赛道]", grade.Items)
	}
	if grade.PublishedAt == nil {
		t.Fatal("published_at should be set after publish")
	}
}

func TestListGradesReturnsOnlyPublishedForStudentAcrossLabs(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	if _, err := svc.ImportGrades(context.Background(), "lab-a", strings.NewReader("student_id,total\n2026001,80\n2026002,70\n")); err != nil {
		t.Fatalf("import lab-a: %v", err)
	}
	if _, err := svc.ImportGrades(context.Background(), "lab-b", strings.NewReader("student_id,total\n2026001,90\n")); err != nil {
		t.Fatalf("import lab-b: %v", err)
	}

	// Nothing published yet.
	if grades, err := svc.ListGrades(context.Background(), "2026001"); err != nil || len(grades) != 0 {
		t.Fatalf("ListGrades before publish = (%v, %v), want empty", grades, err)
	}

	if _, err := svc.PublishGrades(context.Background(), "lab-a"); err != nil {
		t.Fatalf("publish lab-a: %v", err)
	}

	grades, err := svc.ListGrades(context.Background(), "2026001")
	if err != nil {
		t.Fatalf("ListGrades() error = %v", err)
	}
	// lab-a published (student 2026001) → 1; lab-b still unpublished → excluded.
	if len(grades) != 1 || grades[0].LabID != "lab-a" {
		t.Fatalf("grades = %+v, want only lab-a", grades)
	}
}

func TestGetGradeMissingStudent(t *testing.T) {
	svc := NewService(newFakeGradeRepo())
	if _, err := svc.GetGrade(context.Background(), "lab", "nobody"); !errors.Is(err, ErrGradeNotFound) {
		t.Fatalf("GetGrade() error = %v, want ErrGradeNotFound", err)
	}
}

func TestDeleteGradesClearsLabRows(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	if _, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader("student_id,total\n2026001,80\n2026002,90\n")); err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}
	result, err := svc.DeleteGrades(context.Background(), "lab")
	if err != nil {
		t.Fatalf("DeleteGrades() error = %v", err)
	}
	if result.Deleted != 2 {
		t.Fatalf("deleted = %d, want 2", result.Deleted)
	}
	if len(repo.rows) != 0 {
		t.Fatalf("rows remaining = %d, want 0", len(repo.rows))
	}
}

func TestDeleteGradesRejectsBlankLab(t *testing.T) {
	svc := NewService(newFakeGradeRepo())
	if _, err := svc.DeleteGrades(context.Background(), "  "); !errors.Is(err, ErrInvalidLab) {
		t.Fatalf("DeleteGrades() error = %v, want ErrInvalidLab", err)
	}
}

func TestGradeStatusCountsTotalAndPublished(t *testing.T) {
	repo := newFakeGradeRepo()
	svc := NewService(repo)
	if _, err := svc.ImportGrades(context.Background(), "lab", strings.NewReader("student_id,total\n2026001,80\n2026002,90\n2026003,70\n")); err != nil {
		t.Fatalf("ImportGrades() error = %v", err)
	}

	status, err := svc.GradeStatus(context.Background(), "lab")
	if err != nil {
		t.Fatalf("GradeStatus() error = %v", err)
	}
	if status.Total != 3 || status.Published != 0 || status.Unpublished != 3 {
		t.Fatalf("status = %+v, want total 3 / published 0 / unpublished 3", status)
	}

	if _, err := svc.PublishGrades(context.Background(), "lab"); err != nil {
		t.Fatalf("PublishGrades() error = %v", err)
	}
	status, err = svc.GradeStatus(context.Background(), "lab")
	if err != nil {
		t.Fatalf("GradeStatus() error = %v", err)
	}
	if status.Total != 3 || status.Published != 3 || status.Unpublished != 0 {
		t.Fatalf("status = %+v, want total 3 / published 3 / unpublished 0", status)
	}
}
