package grade

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	// ErrGradeNotFound is returned when no published grade exists for the lookup.
	ErrGradeNotFound = errors.New("final grade not found")
	// ErrInvalidCSV signals a malformed grades CSV (missing required columns,
	// unparseable numbers, etc).
	ErrInvalidCSV = errors.New("invalid grades csv")
	// ErrInvalidLab signals a blank lab id.
	ErrInvalidLab = errors.New("invalid lab id")
)

// Service reads and imports externally-computed final course grades.
type Service struct {
	repo Repository
}

// NewService builds the grade service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Grade is the student-facing view of a final course grade.
type Grade struct {
	LabID       string     `json:"lab_id"`
	StudentID   string     `json:"student_id"`
	Total       float64    `json:"total"`
	Track       string     `json:"track,omitempty"`
	Ratio       *float64   `json:"ratio,omitempty"`
	PerfScore   *float64   `json:"perf_score,omitempty"`
	Percentile  *float64   `json:"percentile,omitempty"`
	BoardScore  *float64   `json:"board_score,omitempty"`
	Remark      string     `json:"remark,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
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
	return gradeFromRow(row), nil
}

// ImportGrades parses a TA-produced CSV and upserts each row into final_grades
// (left unpublished). Columns are matched by header name, so order is flexible
// and unknown columns are ignored. Required: student_id, total.
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
	index := headerIndex(header)
	if _, ok := index["student_id"]; !ok {
		return ImportGradesResult{}, fmt.Errorf("%w: missing student_id column", ErrInvalidCSV)
	}
	if _, ok := index["total"]; !ok {
		return ImportGradesResult{}, fmt.Errorf("%w: missing total column", ErrInvalidCSV)
	}

	imported := 0
	rowNum := 1 // header consumed
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return ImportGradesResult{}, fmt.Errorf("%w: %v", ErrInvalidCSV, err)
		}
		rowNum++
		if isBlankRecord(record) {
			continue
		}
		params, err := parseGradeRecord(labID, index, record)
		if err != nil {
			return ImportGradesResult{}, fmt.Errorf("%w: row %d: %v", ErrInvalidCSV, rowNum, err)
		}
		if params == nil {
			continue // blank student id → skip
		}
		if _, err := s.repo.UpsertFinalGrade(ctx, *params); err != nil {
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

func headerIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, name := range header {
		key := strings.ToLower(strings.TrimSpace(name))
		// Strip a UTF-8 BOM that spreadsheet exports often prepend.
		key = strings.TrimPrefix(key, "\ufeff")
		if key == "" {
			continue
		}
		if _, exists := index[key]; !exists {
			index[key] = i
		}
	}
	return index
}

func parseGradeRecord(labID string, index map[string]int, record []string) (*sqlc.UpsertFinalGradeParams, error) {
	get := func(col string) string {
		i, ok := index[col]
		if !ok || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}

	studentID := get("student_id")
	if studentID == "" {
		return nil, nil
	}

	totalText := get("total")
	if totalText == "" {
		return nil, errors.New("missing total")
	}
	total, err := strconv.ParseFloat(totalText, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid total %q", totalText)
	}

	ratio, err := optionalFloat4(get("ratio"))
	if err != nil {
		return nil, fmt.Errorf("invalid ratio: %w", err)
	}
	perfScore, err := optionalFloat4(get("perf_score"))
	if err != nil {
		return nil, fmt.Errorf("invalid perf_score: %w", err)
	}
	percentile, err := optionalFloat4(get("percentile"))
	if err != nil {
		return nil, fmt.Errorf("invalid percentile: %w", err)
	}
	boardScore, err := optionalFloat4(get("board_score"))
	if err != nil {
		return nil, fmt.Errorf("invalid board_score: %w", err)
	}

	return &sqlc.UpsertFinalGradeParams{
		LabID:      labID,
		StudentID:  studentID,
		Total:      float32(total),
		Track:      optionalText(get("track")),
		Ratio:      ratio,
		PerfScore:  perfScore,
		Percentile: percentile,
		BoardScore: boardScore,
		Remark:     optionalText(get("remark")),
	}, nil
}

func optionalText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func optionalFloat4(value string) (pgtype.Float4, error) {
	if value == "" {
		return pgtype.Float4{}, nil
	}
	f, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return pgtype.Float4{}, fmt.Errorf("%q", value)
	}
	return pgtype.Float4{Float32: float32(f), Valid: true}, nil
}

func isBlankRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

func gradeFromRow(row sqlc.FinalGrades) Grade {
	g := Grade{
		LabID:      row.LabID,
		StudentID:  row.StudentID,
		Total:      float64(row.Total),
		Ratio:      float4ToPtr(row.Ratio),
		PerfScore:  float4ToPtr(row.PerfScore),
		Percentile: float4ToPtr(row.Percentile),
		BoardScore: float4ToPtr(row.BoardScore),
	}
	if row.Track.Valid {
		g.Track = strings.TrimSpace(row.Track.String)
	}
	if row.Remark.Valid {
		g.Remark = strings.TrimSpace(row.Remark.String)
	}
	if row.PublishedAt.Valid {
		published := row.PublishedAt.Time.UTC()
		g.PublishedAt = &published
	}
	if row.UpdatedAt.Valid {
		g.UpdatedAt = row.UpdatedAt.Time.UTC()
	}
	return g
}

func float4ToPtr(v pgtype.Float4) *float64 {
	if !v.Valid {
		return nil
	}
	f := float64(v.Float32)
	return &f
}
