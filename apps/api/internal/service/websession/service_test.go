package websession

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestServiceCreatesFragmentRedirectURL(t *testing.T) {
	svc := NewService()
	svc.now = func() time.Time {
		return time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	}
	svc.newToken = func() (string, error) { return "ticket-123", nil }

	result, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
		UserID:       7,
		KeyID:        11,
		RedirectPath: "/auth/confirm",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if got, want := result.RedirectURL, "/auth/session#ticket=ticket-123"; got != want {
		t.Fatalf("RedirectURL = %q, want %q", got, want)
	}
}

func TestServiceRejectsUnsafeRedirectPaths(t *testing.T) {
	svc := NewService()
	cases := []string{
		"",
		"   ",
		"https://evil.example",
		"//evil.example",
		"javascript:alert(1)",
	}

	for _, redirectPath := range cases {
		t.Run(redirectPath, func(t *testing.T) {
			_, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
				UserID:       7,
				KeyID:        11,
				RedirectPath: redirectPath,
			})
			if !errors.Is(err, ErrInvalidRedirectPath) {
				t.Fatalf("CreateTicket() error = %v, want %v", err, ErrInvalidRedirectPath)
			}
		})
	}
}

func TestServiceCleansExpiredTicketsAndEnforcesLimit(t *testing.T) {
	now := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	tokens := []string{"ticket-1", "ticket-2", "ticket-3"}
	nextToken := 0

	svc := NewService(1)
	svc.ttl = time.Minute
	svc.now = func() time.Time { return now }
	svc.newToken = func() (string, error) {
		token := tokens[nextToken]
		nextToken++
		return token, nil
	}

	first, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
		UserID:       7,
		KeyID:        11,
		RedirectPath: "/auth/confirm",
	})
	if err != nil {
		t.Fatalf("CreateTicket(first) error = %v", err)
	}
	if len(svc.tickets) != 1 {
		t.Fatalf("len(tickets) = %d, want 1", len(svc.tickets))
	}

	now = now.Add(2 * time.Minute)
	if _, err := svc.ConsumeTicket(context.Background(), first.Ticket); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("ConsumeTicket(expired) error = %v, want %v", err, ErrInvalidTicket)
	}
	if len(svc.tickets) != 0 {
		t.Fatalf("len(tickets) after cleanup = %d, want 0", len(svc.tickets))
	}

	second, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
		UserID:       7,
		KeyID:        11,
		RedirectPath: "/auth/confirm",
	})
	if err != nil {
		t.Fatalf("CreateTicket(second) error = %v", err)
	}
	if len(svc.tickets) != 1 {
		t.Fatalf("len(tickets) after second create = %d, want 1", len(svc.tickets))
	}

	if _, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
		UserID:       7,
		KeyID:        11,
		RedirectPath: "/auth/confirm",
	}); !errors.Is(err, ErrSessionTicketLimitReached) {
		t.Fatalf("CreateTicket(limit) error = %v, want %v", err, ErrSessionTicketLimitReached)
	}

	consumed, err := svc.ConsumeTicket(context.Background(), second.Ticket)
	if err != nil {
		t.Fatalf("ConsumeTicket(second) error = %v", err)
	}
	if consumed.RedirectPath == "" {
		t.Fatal("ConsumeTicket(second) missing redirect path")
	}
	if _, err := svc.ConsumeTicket(context.Background(), second.Ticket); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("ConsumeTicket(replay) error = %v, want %v", err, ErrInvalidTicket)
	}
}

func TestServiceUsesConfiguredOutstandingLimit(t *testing.T) {
	svc := NewService(2)
	svc.now = func() time.Time {
		return time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	}
	svc.newToken = func() (string, error) { return "ticket-config", nil }

	for i := 0; i < 2; i++ {
		if _, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
			UserID:       7,
			KeyID:        11,
			RedirectPath: "/auth/confirm",
		}); err != nil {
			t.Fatalf("CreateTicket(%d) error = %v", i, err)
		}
		svc.newToken = func() (string, error) { return "ticket-config-2", nil }
	}

	if _, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
		UserID:       7,
		KeyID:        11,
		RedirectPath: "/auth/confirm",
	}); !errors.Is(err, ErrSessionTicketLimitReached) {
		t.Fatalf("CreateTicket(limit) error = %v, want %v", err, ErrSessionTicketLimitReached)
	}
}

func TestPersistentServiceDoesNotOversubscribeOutstandingLimitUnderConcurrency(t *testing.T) {
	repo := &blockingTicketRepo{
		now:         time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
		createGate:  make(chan struct{}),
		createEnter: make(chan struct{}, 1),
		tickets:     make(map[string]sqlc.WebSessionTickets),
	}
	svc := NewPersistentService(repo, 1)
	svc.ttl = time.Minute
	svc.now = func() time.Time { return repo.now }
	tokens := []string{"ticket-1", "ticket-2"}
	var tokenMu sync.Mutex
	svc.newToken = func() (string, error) {
		tokenMu.Lock()
		defer tokenMu.Unlock()
		token := tokens[0]
		tokens = tokens[1:]
		return token, nil
	}

	firstErrCh := make(chan error, 1)
	secondErrCh := make(chan error, 1)
	go func() {
		_, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
			UserID:       7,
			KeyID:        11,
			RedirectPath: "/auth/confirm",
		})
		firstErrCh <- err
	}()

	waitForCreateEnter(t, repo)

	go func() {
		_, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
			UserID:       7,
			KeyID:        11,
			RedirectPath: "/auth/confirm",
		})
		secondErrCh <- err
	}()

	close(repo.createGate)

	if err := <-firstErrCh; err != nil {
		t.Fatalf("CreateTicket(first) error = %v", err)
	}
	if err := <-secondErrCh; !errors.Is(err, ErrSessionTicketLimitReached) {
		t.Fatalf("CreateTicket(second) error = %v, want %v", err, ErrSessionTicketLimitReached)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.tickets) != 1 {
		t.Fatalf("persisted tickets = %d, want 1", len(repo.tickets))
	}
}

func TestPersistentServiceTTLBeginsAfterCreateLockWait(t *testing.T) {
	repo := &blockingTicketRepo{
		now:         time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
		acquireGate: make(chan struct{}),
		tickets:     make(map[string]sqlc.WebSessionTickets),
	}
	svc := NewPersistentService(repo, 1)
	svc.ttl = time.Minute
	svc.now = time.Now
	svc.newToken = func() (string, error) { return "ticket-ttl", nil }

	resultCh := make(chan CreateSessionTicketResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := svc.CreateTicket(context.Background(), CreateSessionTicketInput{
			UserID:       7,
			KeyID:        11,
			RedirectPath: "/auth/confirm",
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	time.Sleep(200 * time.Millisecond)
	repo.releaseAcquire()

	result := waitForTicketResult(t, resultCh, errCh)

	repo.mu.Lock()
	createdAt := repo.createdAt
	repo.mu.Unlock()
	if result.ExpiresAt.Before(createdAt.Add(svc.ttl - 100*time.Millisecond)) {
		t.Fatalf("ExpiresAt = %v, want roughly %v after create time %v", result.ExpiresAt, svc.ttl, createdAt)
	}
}

type blockingTicketRepo struct {
	mu          sync.Mutex
	now         time.Time
	acquireGate chan struct{}
	createGate  chan struct{}
	createEnter chan struct{}
	createdAt   time.Time
	tickets     map[string]sqlc.WebSessionTickets
}

func (r *blockingTicketRepo) WithTx(_ context.Context, fn func(TicketTx) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return fn(&blockingTicketTx{repo: r})
}

type blockingTicketTx struct {
	repo *blockingTicketRepo
}

func (tx *blockingTicketTx) AcquireWebSessionTicketCreateLock(context.Context) error {
	if tx.repo.acquireGate != nil {
		<-tx.repo.acquireGate
	}
	return nil
}

func (tx *blockingTicketTx) CleanupExpiredWebSessionTickets(context.Context) error {
	return nil
}

func (tx *blockingTicketTx) CountActiveWebSessionTickets(context.Context) (int64, error) {
	var count int64
	for _, row := range tx.repo.tickets {
		if row.ExpiresAt.Valid && row.ExpiresAt.Time.After(tx.repo.now) {
			count++
		}
	}
	return count, nil
}

func (tx *blockingTicketTx) CreateWebSessionTicket(_ context.Context, arg sqlc.CreateWebSessionTicketParams) (sqlc.WebSessionTickets, error) {
	if tx.repo.createEnter != nil {
		tx.repo.createEnter <- struct{}{}
	}
	if tx.repo.createGate != nil {
		<-tx.repo.createGate
	}
	tx.repo.createdAt = time.Now().UTC()
	row := sqlc.WebSessionTickets{
		TicketHash:   arg.TicketHash,
		UserID:       arg.UserID,
		KeyID:        arg.KeyID,
		RedirectPath: arg.RedirectPath,
		ExpiresAt:    pgtype.Timestamptz{Time: arg.ExpiresAt.Time, Valid: true},
	}
	tx.repo.tickets[arg.TicketHash] = row
	return row, nil
}

func (tx *blockingTicketTx) ConsumeWebSessionTicket(_ context.Context, ticketHash string) (sqlc.WebSessionTickets, error) {
	row, ok := tx.repo.tickets[ticketHash]
	if !ok {
		return sqlc.WebSessionTickets{}, errors.New("missing")
	}
	delete(tx.repo.tickets, ticketHash)
	return row, nil
}

func waitForCreateEnter(t *testing.T, repo *blockingTicketRepo) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-repo.createEnter:
			return
		case <-deadline:
			t.Fatal("timed out waiting for create entry")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (r *blockingTicketRepo) releaseAcquire() {
	close(r.acquireGate)
}

func waitForTicketResult(t *testing.T, resultCh <-chan CreateSessionTicketResult, errCh <-chan error) CreateSessionTicketResult {
	t.Helper()
	select {
	case err := <-errCh:
		t.Fatalf("CreateTicket() error = %v", err)
	case result := <-resultCh:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ticket result")
	}
	return CreateSessionTicketResult{}
}
