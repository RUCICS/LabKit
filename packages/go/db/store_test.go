package db

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestGeneratedQueriesExposeKeyHelpers(t *testing.T) {
	typ := reflect.TypeOf(&sqlc.Queries{})
	want := []string{
		"CreateLab",
		"GetLab",
		"ListLabs",
		"CreateUser",
		"GetUserByStudentID",
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
