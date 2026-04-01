package websession

import (
	"context"
	"errors"

	dbpkg "labkit.local/packages/go/db"
	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	WithTx(context.Context, func(TicketTx) error) error
}

type TicketTx interface {
	AcquireWebSessionTicketCreateLock(context.Context) error
	CleanupExpiredWebSessionTickets(context.Context) error
	CountActiveWebSessionTickets(context.Context) (int64, error)
	CreateWebSessionTicket(context.Context, sqlc.CreateWebSessionTicketParams) (sqlc.WebSessionTickets, error)
	ConsumeWebSessionTicket(context.Context, string) (sqlc.WebSessionTickets, error)
}

type repo struct {
	store *dbpkg.Store
	pool  *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repo{
		store: dbpkg.New(pool),
		pool:  pool,
	}
}

func (r *repo) WithTx(ctx context.Context, fn func(TicketTx) error) error {
	if r == nil || r.pool == nil {
		return errors.New("web session repository unavailable")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	return dbpkg.WithTx(ctx, tx, func(store *dbpkg.Store) error {
		return fn(&ticketTx{Queries: store.Queries})
	})
}

type ticketTx struct {
	*sqlc.Queries
}

func (tx *ticketTx) AcquireWebSessionTicketCreateLock(ctx context.Context) error {
	return tx.Queries.AcquireWebSessionTicketCreateLock(ctx)
}

func (tx *ticketTx) CleanupExpiredWebSessionTickets(ctx context.Context) error {
	return tx.Queries.CleanupExpiredWebSessionTickets(ctx)
}

func (tx *ticketTx) CountActiveWebSessionTickets(ctx context.Context) (int64, error) {
	return tx.Queries.CountActiveWebSessionTickets(ctx)
}

func (tx *ticketTx) CreateWebSessionTicket(ctx context.Context, arg sqlc.CreateWebSessionTicketParams) (sqlc.WebSessionTickets, error) {
	return tx.Queries.CreateWebSessionTicket(ctx, arg)
}

func (tx *ticketTx) ConsumeWebSessionTicket(ctx context.Context, ticketHash string) (sqlc.WebSessionTickets, error) {
	return tx.Queries.ConsumeWebSessionTicket(ctx, ticketHash)
}
