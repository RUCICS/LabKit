package grade

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	// ErrGradeNotFound is returned when no published grade exists for the lookup.
	ErrGradeNotFound = errors.New("final grade not found")
	// ErrInvalidCSV signals a malformed grades CSV (no student_id column, etc).
	ErrInvalidCSV = errors.New("invalid grades csv")
	// ErrInvalidLab signals a blank lab id.
	ErrInvalidLab = errors.New("invalid lab id")
)

// The platform recognizes only a student key, an optional headline score, and
// an optional note. Everything else is a free-form breakdown column whose
// header becomes the student-facing label. The aliases let a TA name the
// headline / note column in English or Chinese; they carry no lab-specific
// grading meaning.
var (
	totalAliases  = map[string]bool{"total": true, "score": true, "grade": true, "总评": true, "总分": true, "成绩": true}
	remarkAliases = map[string]bool{"remark": true, "note": true, "comment": true, "备注": true, "说明": true}
)

// Service reads and imports externally-computed final course grades. It stores
// and displays grades without understanding any lab's grading scheme.
type Service struct {
	repo Repository
}

// NewService builds the grade service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GradeItem is one label/value pair of the free-form breakdown.
type GradeItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Grade is the student-facing view of a final course grade.
type Grade struct {
	LabID       string      `json:"lab_id"`
	StudentID   string      `json:"student_id"`
	Total       string      `json:"total,omitempty"`
	Items       []GradeItem `json:"items"`
	Remark      string      `json:"remark,omitempty"`
	PublishedAt *time.Time  `json:"published_at,omitempty"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ImportGradesResult reports how many rows were upserted.
type ImportGradesResult struct {
	LabID    string `json:"lab_id"`
	Imported int    `json:"imported"`
}

// PublishGradesResult reports how many rows became visible.
type PublishGradesResult struct {
	LabID     string `json:"lab_id"`
	Published int64  `json:"published"`
}

// DeleteGradesResult reports how many rows were removed.
type DeleteGradesResult struct {
	LabID   string `json:"lab_id"`
	Deleted int64  `json:"deleted"`
}

// GradeStatusResult summarizes the stored grades for a lab, for the admin UI.
type GradeStatusResult struct {
	LabID         string     `json:"lab_id"`
	Total         int64      `json:"total"`
	Published     int64      `json:"published"`
	Unpublished   int64      `json:"unpublished"`
	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`
}

// GetGrade returns the published grade for a (lab, student), or ErrGradeNotFound.
func (s *Service) GetGrade(ctx context.Context, labID, studentID string) (Grade, error) {
	if s == nil || s.repo == nil {
		return Grade{}, fmt.Errorf("grade service unavailable")
	}
	labID = strings.TrimSpace(labID)
	studentID = strings.TrimSpace(studentID)
	if labID == "" || studentID == "" {
		return Grade{}, ErrGradeNotFound
	}
	row, err := s.repo.GetFinalGrade(ctx, sqlc.GetFinalGradeParams{LabID: labID, StudentID: studentID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Grade{}, ErrGradeNotFound
		}
		return Grade{}, err
	}
	return gradeFromRow(row)
}

type columnRole int

const (
	roleIgnore columnRole = iota
	roleStudentID
	roleTotal
	roleRemark
	roleItem
)

type columnPlan struct {
	role  columnRole
	label string
}

// ImportGrades parses a TA-produced CSV and upserts each row into final_grades
// (left unpublished). Columns are matched by header name: student_id (required)
// is the key, total/remark (optional) are the headline score and note, and any
// other column becomes a free-form breakdown item labelled by its header.
func (s *Service) ImportGrades(ctx context.Context, labID string, r io.Reader) (ImportGradesResult, error) {
	if s == nil || s.repo == nil {
		return ImportGradesResult{}, fmt.Errorf("grade service unavailable")
	}
	labID = strings.TrimSpace(labID)
	if labID == "" {
		return ImportGradesResult{}, ErrInvalidLab
	}

	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return ImportGradesResult{}, fmt.Errorf("%w: empty csv", ErrInvalidCSV)
		}
		return ImportGradesResult{}, fmt.Errorf("%w: %v", ErrInvalidCSV, err)
	}
	plan, studentIDIdx := planColumns(header)
	if studentIDIdx < 0 {
		return ImportGradesResult{}, fmt.Errorf("%w: missing student_id column", ErrInvalidCSV)
	}

	imported := 0
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return ImportGradesResult{}, fmt.Errorf("%w: %v", ErrInvalidCSV, err)
		}
		if isBlankRecord(record) {
			continue
		}
		params, ok, err := buildGradeParams(labID, plan, studentIDIdx, record)
		if err != nil {
			return ImportGradesResult{}, err
		}
		if !ok {
			continue // blank student id → skip
		}
		if _, err := s.repo.UpsertFinalGrade(ctx, params); err != nil {
			return ImportGradesResult{}, err
		}
		imported++
	}
	return ImportGradesResult{LabID: labID, Imported: imported}, nil
}

// PublishGrades makes all currently-unpublished grades for a lab visible.
func (s *Service) PublishGrades(ctx context.Context, labID string) (PublishGradesResult, error) {
	if s == nil || s.repo == nil {
		return PublishGradesResult{}, fmt.Errorf("grade service unavailable")
	}
	labID = strings.TrimSpace(labID)
	if labID == "" {
		return PublishGradesResult{}, ErrInvalidLab
	}
	n, err := s.repo.PublishFinalGrades(ctx, labID)
	if err != nil {
		return PublishGradesResult{}, err
	}
	return PublishGradesResult{LabID: labID, Published: n}, nil
}

// DeleteGrades removes every grade row for a lab, supporting a clean re-import.
func (s *Service) DeleteGrades(ctx context.Context, labID string) (DeleteGradesResult, error) {
	if s == nil || s.repo == nil {
		return DeleteGradesResult{}, fmt.Errorf("grade service unavailable")
	}
	labID = strings.TrimSpace(labID)
	if labID == "" {
		return DeleteGradesResult{}, ErrInvalidLab
	}
	n, err := s.repo.DeleteFinalGradesByLab(ctx, labID)
	if err != nil {
		return DeleteGradesResult{}, err
	}
	return DeleteGradesResult{LabID: labID, Deleted: n}, nil
}

// GradeStatus returns counts of stored/published grades for a lab.
func (s *Service) GradeStatus(ctx context.Context, labID string) (GradeStatusResult, error) {
	if s == nil || s.repo == nil {
		return GradeStatusResult{}, fmt.Errorf("grade service unavailable")
	}
	labID = strings.TrimSpace(labID)
	if labID == "" {
		return GradeStatusResult{}, ErrInvalidLab
	}
	row, err := s.repo.SummarizeFinalGrades(ctx, labID)
	if err != nil {
		return GradeStatusResult{}, err
	}
	result := GradeStatusResult{
		LabID:       labID,
		Total:       row.Total,
		Published:   row.Published,
		Unpublished: row.Total - row.Published,
	}
	if row.LastUpdated.Valid {
		updated := row.LastUpdated.Time.UTC()
		result.LastUpdatedAt = &updated
	}
	return result, nil
}

// planColumns classifies each header column. Returns the per-column plan and the
// student_id column index (-1 if absent). The first matching column wins for
// student_id / total / remark; later matches fall through to free-form items.
func planColumns(header []string) ([]columnPlan, int) {
	plan := make([]columnPlan, len(header))
	studentIDIdx := -1
	totalTaken := false
	remarkTaken := false
	for i, raw := range header {
		label := strings.TrimPrefix(strings.TrimSpace(raw), "\ufeff")
		key := strings.ToLower(label)
		switch {
		case key == "":
			plan[i] = columnPlan{role: roleIgnore}
		case key == "student_id" && studentIDIdx < 0:
			studentIDIdx = i
			plan[i] = columnPlan{role: roleStudentID, label: label}
		case !totalTaken && totalAliases[key]:
			totalTaken = true
			plan[i] = columnPlan{role: roleTotal, label: label}
		case !remarkTaken && remarkAliases[key]:
			remarkTaken = true
			plan[i] = columnPlan{role: roleRemark, label: label}
		default:
			plan[i] = columnPlan{role: roleItem, label: label}
		}
	}
	return plan, studentIDIdx
}

func buildGradeParams(labID string, plan []columnPlan, studentIDIdx int, record []string) (sqlc.UpsertFinalGradeParams, bool, error) {
	cell := func(i int) string {
		if i < 0 || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}

	studentID := cell(studentIDIdx)
	if studentID == "" {
		return sqlc.UpsertFinalGradeParams{}, false, nil
	}

	var total, remark string
	items := make([]GradeItem, 0, len(plan))
	for i, p := range plan {
		value := cell(i)
		switch p.role {
		case roleTotal:
			total = value
		case roleRemark:
			remark = value
		case roleItem:
			if value != "" {
				items = append(items, GradeItem{Label: p.label, Value: value})
			}
		}
	}

	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return sqlc.UpsertFinalGradeParams{}, false, err
	}
	return sqlc.UpsertFinalGradeParams{
		LabID:     labID,
		StudentID: studentID,
		Total:     optionalText(total),
		Remark:    optionalText(remark),
		Items:     itemsJSON,
	}, true, nil
}

func optionalText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func isBlankRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

func gradeFromRow(row sqlc.FinalGrades) (Grade, error) {
	g := Grade{
		LabID:     row.LabID,
		StudentID: row.StudentID,
		Items:     []GradeItem{},
	}
	if row.Total.Valid {
		g.Total = strings.TrimSpace(row.Total.String)
	}
	if row.Remark.Valid {
		g.Remark = strings.TrimSpace(row.Remark.String)
	}
	if len(row.Items) > 0 {
		if err := json.Unmarshal(row.Items, &g.Items); err != nil {
			return Grade{}, err
		}
		if g.Items == nil {
			g.Items = []GradeItem{}
		}
	}
	if row.PublishedAt.Valid {
		published := row.PublishedAt.Time.UTC()
		g.PublishedAt = &published
	}
	if row.UpdatedAt.Valid {
		g.UpdatedAt = row.UpdatedAt.Time.UTC()
	}
	return g, nil
}
