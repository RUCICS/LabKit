package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestGeneratedQueriesExposeKeyHelpers(t *testing.T) {
	typ := reflect.TypeOf(&sqlc.Queries{})
	want := []string{
		"CreateLab",
		"GetLab",
		"ListLabs",
		"CreateUser",
		"GetUserByStudentID",
		"UpdateUserNickname",
		"CreateDeviceAuthRequest",
		"GetPendingDeviceAuthRequestByUserCode",
		"CreateWebSessionTicket",
		"AcquireWebSessionTicketCreateLock",
		"CleanupExpiredWebSessionTickets",
		"CountActiveWebSessionTickets",
		"ConsumeWebSessionTicket",
		"ListUserKeys",
		"CreateSubmission",
		"GetSubmission",
		"ListSubmissionsByUserLab",
		"ListRecentSubmissionsByUser",
		"GetLatestSubmissionByUserLab",
		"CountSubmissionQuotaUsage",
		"UpdateSubmissionQuotaState",
		"ListLabProfilesByLab",
		"ListScoresByLab",
		"CreateScore",
		"UpsertLeaderboardEntry",
		"ListLeaderboardByLab",
		"ListLeaderboardByLabAndMetricAsc",
		"ListLeaderboardByLabAndMetricDesc",
		"CreateEvaluationJob",
		"GetNextQueuedEvaluationJob",
		"ListRecentEvaluationJobsByLab",
	}

	for _, name := range want {
		if _, ok := typ.MethodByName(name); !ok {
			t.Fatalf("missing generated query helper %q", name)
		}
	}
}

func TestWithTxCommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}

	err := WithTx(context.Background(), tx, func(s *Store) error {
		if s.Queries == nil {
			t.Fatalf("store queries not initialized")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTx() error = %v", err)
	}
	if !tx.committed {
		t.Fatalf("expected transaction commit")
	}
	if tx.rolledBack {
		t.Fatalf("did not expect rollback")
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	tx := &fakeTx{}

	want := errors.New("boom")
	err := WithTx(context.Background(), tx, func(s *Store) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("WithTx() error = %v, want %v", err, want)
	}
	if !tx.rolledBack {
		t.Fatalf("expected transaction rollback")
	}
	if tx.committed {
		t.Fatalf("did not expect commit")
	}
}

func TestWithTxRollsBackOnPanic(t *testing.T) {
	tx := &fakeTx{}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic")
		}
		if !tx.rolledBack {
			t.Fatalf("expected transaction rollback")
		}
		if tx.committed {
			t.Fatalf("did not expect commit")
		}
	}()

	_ = WithTx(context.Background(), tx, func(s *Store) error {
		panic("boom")
	})
}

func TestUpdateSubmissionQuotaStateRejectsInvalidValue(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t, ctx)
	resetTestDatabase(t, ctx, pool)

	store := New(pool)
	row, err := store.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		UserID:      1,
		LabID:       "lab-1",
		KeyID:       1,
		ArtifactKey: "lab-1/1/artifact.tar.gz",
		ContentHash: "hash-1",
		Status:      "queued",
	})
	if err != nil {
		t.Fatalf("CreateSubmission() error = %v", err)
	}

	err = store.UpdateSubmissionQuotaState(ctx, sqlc.UpdateSubmissionQuotaStateParams{
		ID:         row.ID,
		QuotaState: "bogus",
	})
	if err == nil {
		t.Fatal("UpdateSubmissionQuotaState() error = nil, want constraint violation")
	}
}

func TestCreateUserKeyAllowsReusingPublicKeyAfterRevoke(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t, ctx)
	resetTestDatabase(t, ctx, pool)

	store := New(pool)
	if err := store.DeleteUserKey(ctx, sqlc.DeleteUserKeyParams{
		ID:     1,
		UserID: 1,
	}); err != nil {
		t.Fatalf("DeleteUserKey() error = %v", err)
	}

	row, err := store.CreateUserKey(ctx, sqlc.CreateUserKeyParams{
		UserID:      1,
		PublicKey:   "pk-1",
		Fingerprint: pgtype.Text{String: "fp-1", Valid: true},
		DeviceName:  "device-1b",
	})
	if err != nil {
		t.Fatalf("CreateUserKey() error = %v", err)
	}
	if got, want := row.PublicKey, "pk-1"; got != want {
		t.Fatalf("PublicKey = %q, want %q", got, want)
	}
	if got, want := row.DeviceName, "device-1b"; got != want {
		t.Fatalf("DeviceName = %q, want %q", got, want)
	}
}

func TestUpdateUserNicknamePersistsValueAndRecentSubmissionsOrder(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t, ctx)
	resetTestDatabase(t, ctx, pool)

	store := New(pool)
	user, err := store.UpdateUserNickname(ctx, sqlc.UpdateUserNicknameParams{
		ID:       1,
		Nickname: "alice",
	})
	if err != nil {
		t.Fatalf("UpdateUserNickname() error = %v", err)
	}
	if got, want := user.Nickname, "alice"; got != want {
		t.Fatalf("Nickname = %q, want %q", got, want)
	}

	profile, err := store.GetUserByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if got, want := profile.Nickname, "alice"; got != want {
		t.Fatalf("Nickname = %q, want %q", got, want)
	}

	firstID := uuidMustParse(t, "11111111-1111-7111-8111-111111111111")
	secondID := uuidMustParse(t, "22222222-2222-7222-8222-222222222222")

	for _, submission := range []struct {
		ID        uuid.UUID
		Hash      string
		Status    string
		CreatedAt time.Time
	}{
		{
			ID:        firstID,
			Hash:      "hash-1",
			Status:    "done",
			CreatedAt: mustParseTime(t, "2026-04-09T08:00:00Z"),
		},
		{
			ID:        secondID,
			Hash:      "hash-2",
			Status:    "queued",
			CreatedAt: mustParseTime(t, "2026-04-09T09:00:00Z"),
		},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO submissions (
				id, user_id, lab_id, key_id, artifact_key, content_hash, status, created_at, quota_state
			)
			VALUES ($1, 1, 'lab-1', 1, $2, $3, $4, $5, 'free')
		`, submission.ID, "lab-1/1/"+submission.Hash+".tar.gz", submission.Hash, submission.Status, submission.CreatedAt); err != nil {
			t.Fatalf("insert submission error = %v", err)
		}
	}

	rows, err := store.ListRecentSubmissionsByUser(ctx, sqlc.ListRecentSubmissionsByUserParams{
		UserID: 1,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("ListRecentSubmissionsByUser() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].ID != secondID || rows[1].ID != firstID {
		t.Fatalf("rows = %#v, want newest submission first", rows)
	}
}

func TestUserProfileNicknameMigrationBackfillsFromLabProfiles(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDatabaseURL)
	if err != nil {
		t.Fatalf("pgx.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close(ctx)
	})

	if _, err := conn.Exec(ctx, `
		CREATE TEMP TABLE users (
			id BIGINT PRIMARY KEY,
			student_id TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TEMP TABLE lab_profiles (
			user_id BIGINT NOT NULL,
			lab_id TEXT NOT NULL,
			nickname TEXT NOT NULL DEFAULT '匿名',
			track TEXT,
			PRIMARY KEY (user_id, lab_id)
		);
	`); err != nil {
		t.Fatalf("create temp legacy tables error = %v", err)
	}

	if _, err := conn.Exec(ctx, `
		INSERT INTO users (id, student_id) VALUES
			(1, 's0000001'),
			(2, 's0000002'),
			(3, 's0000003'),
			(4, 's0000004');
		INSERT INTO lab_profiles (user_id, lab_id, nickname) VALUES
			(1, 'lab-a', 'Alpha'),
			(1, 'lab-z', 'Alpha'),
			(2, 'lab-b', ''),
			(2, 'lab-a', '   '),
			(4, 'lab-a', 'Alpha'),
			(4, 'lab-z', 'Zulu');
	`); err != nil {
		t.Fatalf("seed legacy rows error = %v", err)
	}

	body, err := os.ReadFile(filepath.Join(repoRoot(), "db", "migrations", "0008_user_profile_nickname.up.sql"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if _, err := conn.Exec(ctx, string(body)); err != nil {
		t.Fatalf("apply nickname migration error = %v", err)
	}

	rows, err := conn.Query(ctx, `SELECT id, nickname FROM users ORDER BY id`)
	if err != nil {
		t.Fatalf("query migrated users error = %v", err)
	}
	defer rows.Close()

	type userRow struct {
		id       int64
		nickname string
	}
	var got []userRow
	for rows.Next() {
		var row userRow
		if err := rows.Scan(&row.id, &row.nickname); err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}

	want := []userRow{
		{id: 1, nickname: "Alpha"},
		{id: 2, nickname: "匿名"},
		{id: 3, nickname: "匿名"},
		{id: 4, nickname: "匿名"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated users = %#v, want %#v", got, want)
	}
}

func uuidMustParse(t *testing.T, value string) uuid.UUID {
	t.Helper()

	id, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("uuid.Parse(%q) error = %v", value, err)
	}
	return id
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", value, err)
	}
	return parsed
}

type fakeTx struct {
	committed  bool
	rolledBack bool
}

func (f *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func (f *fakeTx) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}
