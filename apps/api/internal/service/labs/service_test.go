package labs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRegisterLabCreatesLabFromValidManifest(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	lab, err := svc.RegisterLab(context.Background(), []byte(validManifest(`
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`)))
	if err != nil {
		t.Fatalf("RegisterLab() error = %v", err)
	}

	if lab.ID != "sorting" {
		t.Fatalf("lab id = %q, want %q", lab.ID, "sorting")
	}
	if lab.Name != "Sorting Lab" {
		t.Fatalf("lab name = %q, want %q", lab.Name, "Sorting Lab")
	}
	if repo.createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", repo.createCalls)
	}
	if repo.labs["sorting"].ManifestUpdatedAt.Time.IsZero() {
		t.Fatalf("manifest updated at was not set")
	}
}

func TestRegisterLabRejectsExistingLab(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RegisterLab(context.Background(), []byte(validManifest(`
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`))); err != nil {
		t.Fatalf("initial RegisterLab() error = %v", err)
	}

	_, err := svc.RegisterLab(context.Background(), []byte(nonStructuralUpdateManifest()))
	if !errors.Is(err, ErrLabAlreadyExists) {
		t.Fatalf("RegisterLab() error = %v, want ErrLabAlreadyExists", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("update calls = %d, want 0", repo.updateCalls)
	}
}

func TestRegisterLabTranslatesUniqueViolationToAlreadyExists(t *testing.T) {
	repo := newFakeRepository()
	repo.createErr = &pgconn.PgError{Code: "23505"}
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	_, err := svc.RegisterLab(context.Background(), []byte(validManifest(`
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`)))
	if !errors.Is(err, ErrLabAlreadyExists) {
		t.Fatalf("RegisterLab() error = %v, want ErrLabAlreadyExists", err)
	}
}

func TestUpdateLabRejectsStructuralUpdates(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RegisterLab(context.Background(), []byte(validManifest(`
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`))); err != nil {
		t.Fatalf("initial RegisterLab() error = %v", err)
	}

	_, err := svc.UpdateLab(context.Background(), "sorting", []byte(structuralUpdateManifest()))
	if !errors.Is(err, ErrStructuralUpdate) {
		t.Fatalf("UpdateLab() error = %v, want ErrStructuralUpdate", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("update calls = %d, want 0", repo.updateCalls)
	}
}

func TestUpdateLabAllowsNonStructuralUpdates(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RegisterLab(context.Background(), []byte(validManifest(`
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`))); err != nil {
		t.Fatalf("initial RegisterLab() error = %v", err)
	}

	lab, err := svc.UpdateLab(context.Background(), "sorting", []byte(nonStructuralUpdateManifest()))
	if err != nil {
		t.Fatalf("UpdateLab() update error = %v", err)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", repo.updateCalls)
	}
	if lab.Name != "Sorting Lab Renamed" {
		t.Fatalf("lab name = %q, want %q", lab.Name, "Sorting Lab Renamed")
	}
	if lab.Manifest.Board.Pick != true {
		t.Fatalf("board.pick = %v, want true", lab.Manifest.Board.Pick)
	}
}

func TestUpdateLabRejectsSubmitChanges(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RegisterLab(context.Background(), []byte(validManifest(`
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`))); err != nil {
		t.Fatalf("initial RegisterLab() error = %v", err)
	}

	_, err := svc.UpdateLab(context.Background(), "sorting", []byte(submitUpdateManifest()))
	if !errors.Is(err, ErrStructuralUpdate) {
		t.Fatalf("UpdateLab() error = %v, want ErrStructuralUpdate", err)
	}
}

func TestListPublicLabsReturnsPublicManifestOnly(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RegisterLab(context.Background(), []byte(validManifest(`
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`))); err != nil {
		t.Fatalf("RegisterLab() error = %v", err)
	}

	labs, err := svc.ListPublicLabs(context.Background())
	if err != nil {
		t.Fatalf("ListPublicLabs() error = %v", err)
	}
	if len(labs) != 1 {
		t.Fatalf("len(labs) = %d, want 1", len(labs))
	}
	if labs[0].Manifest.Eval.Image != "" {
		t.Fatalf("public eval image = %q, want empty", labs[0].Manifest.Eval.Image)
	}
	if labs[0].Manifest.Lab.ID != "sorting" {
		t.Fatalf("public manifest lab id = %q, want %q", labs[0].Manifest.Lab.ID, "sorting")
	}

	var stored manifest.Manifest
	if err := json.Unmarshal(repo.labs["sorting"].Manifest, &stored); err != nil {
		t.Fatalf("unmarshal stored manifest: %v", err)
	}
	if got := stored.Eval.Image; got == "" {
		t.Fatalf("stored manifest image = %q, want non-empty", got)
	}
}

func TestPublicLabsHonorVisibilitySchedule(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RegisterLab(context.Background(), []byte(validManifest(`
visible = 2026-04-01T00:00:00Z
open = 2026-04-02T00:00:00Z
close = 2026-04-30T00:00:00Z
`))); err != nil {
		t.Fatalf("RegisterLab() error = %v", err)
	}

	labs, err := svc.ListPublicLabs(context.Background())
	if err != nil {
		t.Fatalf("ListPublicLabs() error = %v", err)
	}
	if len(labs) != 0 {
		t.Fatalf("len(labs) before visible = %d, want 0", len(labs))
	}

	if _, err := svc.GetPublicLab(context.Background(), "sorting"); !errors.Is(err, ErrLabNotFound) {
		t.Fatalf("GetPublicLab() error = %v, want ErrLabNotFound", err)
	}

	svc.now = func() time.Time { return time.Date(2026, 4, 1, 0, 0, 1, 0, time.UTC) }

	lab, err := svc.GetPublicLab(context.Background(), "sorting")
	if err != nil {
		t.Fatalf("GetPublicLab() after visible error = %v", err)
	}
	if lab.ID != "sorting" {
		t.Fatalf("lab id = %q, want %q", lab.ID, "sorting")
	}
}

func newTestService(repo *fakeRepository, now time.Time) *Service {
	svc := NewService(repo)
	svc.now = func() time.Time { return now }
	return svc
}

type fakeRepository struct {
	labs        map[string]sqlc.Labs
	createCalls int
	updateCalls int
	createErr   error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		labs: make(map[string]sqlc.Labs),
	}
}

func (r *fakeRepository) CreateLab(_ context.Context, arg sqlc.CreateLabParams) (sqlc.Labs, error) {
	r.createCalls++
	if r.createErr != nil {
		return sqlc.Labs{}, r.createErr
	}
	row := sqlc.Labs{
		ID:                arg.ID,
		Name:              arg.Name,
		Manifest:          append([]byte(nil), arg.Manifest...),
		ManifestUpdatedAt: pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC), Valid: true},
	}
	r.labs[arg.ID] = row
	return row, nil
}

func (r *fakeRepository) GetLab(_ context.Context, id string) (sqlc.Labs, error) {
	row, ok := r.labs[id]
	if !ok {
		return sqlc.Labs{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *fakeRepository) ListLabs(_ context.Context) ([]sqlc.Labs, error) {
	items := make([]sqlc.Labs, 0, len(r.labs))
	for _, lab := range r.labs {
		items = append(items, lab)
	}
	return items, nil
}

func (r *fakeRepository) UpdateLabManifest(_ context.Context, arg sqlc.UpdateLabManifestParams) error {
	row, ok := r.labs[arg.ID]
	if !ok {
		return pgx.ErrNoRows
	}
	r.updateCalls++
	row.Name = arg.Name
	row.Manifest = append([]byte(nil), arg.Manifest...)
	row.ManifestUpdatedAt = pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 13, 0, 0, 0, time.UTC), Valid: true}
	r.labs[arg.ID] = row
	return nil
}

func validManifest(schedule string) string {
	return `
[lab]
id = "sorting"
name = "Sorting Lab"

[submit]
files = ["main.py"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/sorting:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
name = "Runtime"
sort = "asc"
unit = "ms"

[board]
rank_by = "runtime_ms"
pick = false

[schedule]
` + schedule
}

func structuralUpdateManifest() string {
	return `
[lab]
id = "sorting"
name = "Sorting Lab v2"

[submit]
files = ["main.py", "README.md"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/sorting:2"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
name = "Runtime"
sort = "desc"
unit = "ms"

[board]
rank_by = "runtime_ms"
pick = false

[schedule]
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`
}

func nonStructuralUpdateManifest() string {
	return `
[lab]
id = "sorting"
name = "Sorting Lab Renamed"

[submit]
files = ["main.py"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/sorting:2"
timeout = 120

[quota]
daily = 10
free = ["build_failed", "rejected"]

[[metric]]
id = "runtime_ms"
name = "Runtime v2"
sort = "asc"
unit = "milliseconds"

[board]
rank_by = "runtime_ms"
pick = true

[schedule]
visible = 2026-03-29T00:00:00Z
open = 2026-03-31T12:00:00Z
close = 2026-05-01T00:00:00Z
`
}

func submitUpdateManifest() string {
	return `
[lab]
id = "sorting"
name = "Sorting Lab"

[submit]
files = ["main.py"]
max_size = "5MB"

[eval]
image = "ghcr.io/labkit/sorting:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
name = "Runtime"
sort = "asc"
unit = "ms"

[board]
rank_by = "runtime_ms"
pick = false

[schedule]
visible = 2026-03-30T00:00:00Z
open = 2026-03-31T00:00:00Z
close = 2026-04-30T00:00:00Z
`
}
