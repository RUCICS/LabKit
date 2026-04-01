package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrLabNotFound      = errors.New("lab not found")
	ErrLabAlreadyExists = errors.New("lab already exists")
	ErrInvalidManifest  = errors.New("invalid manifest")
	ErrStructuralUpdate = errors.New("structural lab update rejected")
)

type Repository interface {
	CreateLab(context.Context, sqlc.CreateLabParams) (sqlc.Labs, error)
	GetLab(context.Context, string) (sqlc.Labs, error)
	ListLabs(context.Context) ([]sqlc.Labs, error)
	UpdateLabManifest(context.Context, sqlc.UpdateLabManifestParams) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

type Lab struct {
	ID                string                  `json:"id"`
	Name              string                  `json:"name"`
	Manifest          manifest.PublicManifest `json:"manifest"`
	ManifestUpdatedAt time.Time               `json:"manifest_updated_at,omitempty"`
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

func (s *Service) RegisterLab(ctx context.Context, rawManifest []byte) (Lab, error) {
	parsed, storedManifest, err := parseAndMarshalManifest(rawManifest)
	if err != nil {
		return Lab{}, err
	}

	_, err = s.repo.GetLab(ctx, parsed.Lab.ID)
	if err == nil {
		return Lab{}, ErrLabAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Lab{}, err
	}

	created, err := s.repo.CreateLab(ctx, sqlc.CreateLabParams{
		ID:       parsed.Lab.ID,
		Name:     parsed.Lab.Name,
		Manifest: storedManifest,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Lab{}, ErrLabAlreadyExists
		}
		return Lab{}, err
	}
	return publicLabFromRow(created)
}

func (s *Service) UpdateLab(ctx context.Context, id string, rawManifest []byte) (Lab, error) {
	parsed, storedManifest, err := parseAndMarshalManifest(rawManifest)
	if err != nil {
		return Lab{}, err
	}
	if parsed.Lab.ID != id {
		return Lab{}, ErrStructuralUpdate
	}
	row, err := s.repo.GetLab(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Lab{}, ErrLabNotFound
		}
		return Lab{}, err
	}

	currentManifest, err := parseStoredManifest(row.Manifest)
	if err != nil {
		return Lab{}, err
	}
	if !sameStructure(currentManifest, parsed) {
		return Lab{}, ErrStructuralUpdate
	}

	if err := s.repo.UpdateLabManifest(ctx, sqlc.UpdateLabManifestParams{
		ID:       id,
		Name:     parsed.Lab.Name,
		Manifest: storedManifest,
	}); err != nil {
		return Lab{}, err
	}

	updated, err := s.repo.GetLab(ctx, id)
	if err != nil {
		return Lab{}, err
	}
	return publicLabFromRow(updated)
}

func (s *Service) ListPublicLabs(ctx context.Context) ([]Lab, error) {
	rows, err := s.repo.ListLabs(ctx)
	if err != nil {
		return nil, err
	}

	labs := make([]Lab, 0, len(rows))
	for _, row := range rows {
		lab, err := publicLabFromRow(row)
		if err != nil {
			return nil, err
		}
		if isVisible(lab.Manifest.Schedule, s.nowUTC()) {
			labs = append(labs, lab)
		}
	}
	return labs, nil
}

func (s *Service) GetPublicLab(ctx context.Context, id string) (Lab, error) {
	row, err := s.repo.GetLab(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Lab{}, ErrLabNotFound
		}
		return Lab{}, err
	}

	lab, err := publicLabFromRow(row)
	if err != nil {
		return Lab{}, err
	}
	if !isVisible(lab.Manifest.Schedule, s.nowUTC()) {
		return Lab{}, ErrLabNotFound
	}
	return lab, nil
}

func (s *Service) nowUTC() time.Time {
	now := s.now
	if now == nil {
		now = time.Now
	}
	return now().UTC()
}

func publicLabFromRow(row sqlc.Labs) (Lab, error) {
	parsed, err := parseStoredManifest(row.Manifest)
	if err != nil {
		return Lab{}, err
	}
	return Lab{
		ID:                row.ID,
		Name:              row.Name,
		Manifest:          parsed.Public(),
		ManifestUpdatedAt: row.ManifestUpdatedAt.Time,
	}, nil
}

func parseAndMarshalManifest(rawManifest []byte) (*manifest.Manifest, []byte, error) {
	parsed, err := manifest.Parse(rawManifest)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	storedManifest, err := json.Marshal(parsed)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal manifest: %w", err)
	}
	return parsed, storedManifest, nil
}

func parseStoredManifest(rawManifest []byte) (*manifest.Manifest, error) {
	var parsed manifest.Manifest
	if err := json.Unmarshal(rawManifest, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func isVisible(schedule manifest.ScheduleSection, now time.Time) bool {
	if schedule.Visible.IsZero() {
		return true
	}
	return !now.Before(schedule.Visible)
}

func sameStructure(current, next *manifest.Manifest) bool {
	if current.Submit.MaxSize != next.Submit.MaxSize {
		return false
	}
	if len(current.Submit.Files) != len(next.Submit.Files) {
		return false
	}
	for i := range current.Submit.Files {
		if current.Submit.Files[i] != next.Submit.Files[i] {
			return false
		}
	}
	if len(current.Metrics) != len(next.Metrics) {
		return false
	}
	if current.Board.RankBy != next.Board.RankBy {
		return false
	}
	for i := range current.Metrics {
		if current.Metrics[i].ID != next.Metrics[i].ID {
			return false
		}
		if current.Metrics[i].Sort != next.Metrics[i].Sort {
			return false
		}
	}
	return true
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return false
}
