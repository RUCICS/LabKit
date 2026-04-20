package db

import (
	"context"
	"errors"

	"labkit.local/packages/go/db/sqlc"
)

// Tx is the minimal transaction surface used by the store helper.
type Tx interface {
	sqlc.DBTX
	Commit(context.Context) error
	Rollback(context.Context) error
}

// Store wraps the generated sqlc querier.
type Store struct {
	*sqlc.Queries
}

// New constructs a Store around a sqlc DB executor.
func New(db sqlc.DBTX) *Store {
	return &Store{Queries: sqlc.New(db)}
}

func (s *Store) GetUserProfileByID(ctx context.Context, userID int64) (sqlc.Users, error) {
	return s.GetUserByID(ctx, userID)
}

// WithTx runs fn within the supplied transaction and commits or rolls back.
func WithTx(ctx context.Context, tx Tx, fn func(*Store) error) error {
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r)
		}
	}()

	store := New(tx)
	if err := fn(store); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	return tx.Commit(ctx)
}
