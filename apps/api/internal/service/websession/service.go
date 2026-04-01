package websession

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidTicket             = errors.New("invalid session ticket")
	ErrInvalidRedirectPath       = errors.New("invalid redirect path")
	ErrSessionTicketLimitReached = errors.New("session ticket limit reached")
)

const defaultMaxOutstandingTickets = 64

type Service struct {
	mu             sync.Mutex
	now            func() time.Time
	ttl            time.Duration
	maxOutstanding int
	tickets        map[string]sessionTicket
	newToken       func() (string, error)
	repo           Repository
}

type CreateSessionTicketInput struct {
	UserID       int64
	KeyID        int64
	RedirectPath string
}

type CreateSessionTicketResult struct {
	Ticket      string    `json:"ticket"`
	RedirectURL string    `json:"redirect_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type ConsumeSessionTicketResult struct {
	UserID       int64
	KeyID        int64
	RedirectPath string
}

type sessionTicket struct {
	UserID       int64
	KeyID        int64
	RedirectPath string
	ExpiresAt    time.Time
}

func NewService(maxOutstanding ...int) *Service {
	limit := defaultMaxOutstandingTickets
	if len(maxOutstanding) > 0 && maxOutstanding[0] > 0 {
		limit = maxOutstanding[0]
	}
	return &Service{
		now:            time.Now,
		ttl:            time.Minute,
		maxOutstanding: limit,
		tickets:        make(map[string]sessionTicket),
		newToken:       randomTicketToken,
	}
}

func NewPersistentService(repo Repository, maxOutstanding ...int) *Service {
	svc := NewService(maxOutstanding...)
	svc.repo = repo
	return svc
}

func (s *Service) CreateTicket(ctx context.Context, in CreateSessionTicketInput) (CreateSessionTicketResult, error) {
	if s == nil {
		return CreateSessionTicketResult{}, errors.New("web session service unavailable")
	}
	userID := in.UserID
	keyID := in.KeyID
	if userID <= 0 || keyID <= 0 {
		return CreateSessionTicketResult{}, ErrInvalidTicket
	}
	redirectPath := strings.TrimSpace(in.RedirectPath)
	if err := validateRedirectPath(redirectPath); err != nil {
		return CreateSessionTicketResult{}, err
	}
	token, err := s.newTicketToken()
	if err != nil {
		return CreateSessionTicketResult{}, err
	}

	if s.repo != nil {
		var result CreateSessionTicketResult
		limit := s.maxOutstanding
		if limit <= 0 {
			limit = defaultMaxOutstandingTickets
		}
		err := s.repo.WithTx(ctx, func(tx TicketTx) error {
			if err := tx.AcquireWebSessionTicketCreateLock(ctx); err != nil {
				return err
			}
			if err := tx.CleanupExpiredWebSessionTickets(ctx); err != nil {
				return err
			}
			now := s.nowUTC()
			expiresAt := now.Add(s.ttl)
			count, err := tx.CountActiveWebSessionTickets(ctx)
			if err != nil {
				return err
			}
			if int(count) >= limit {
				return ErrSessionTicketLimitReached
			}
			_, err = tx.CreateWebSessionTicket(ctx, sqlc.CreateWebSessionTicketParams{
				TicketHash:   hashTicketToken(token),
				UserID:       userID,
				KeyID:        keyID,
				RedirectPath: redirectPath,
				ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
			})
			if err != nil {
				return err
			}
			result = CreateSessionTicketResult{
				Ticket:      token,
				RedirectURL: "/auth/session#ticket=" + token,
				ExpiresAt:   expiresAt,
			}
			return nil
		})
		if err != nil {
			return CreateSessionTicketResult{}, err
		}
		return result, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowUTC()
	s.cleanupExpiredLocked(now)
	expiresAt := now.Add(s.ttl)
	limit := s.maxOutstanding
	if limit <= 0 {
		limit = defaultMaxOutstandingTickets
	}
	if len(s.tickets) >= limit {
		return CreateSessionTicketResult{}, ErrSessionTicketLimitReached
	}
	s.tickets[token] = sessionTicket{
		UserID:       userID,
		KeyID:        keyID,
		RedirectPath: redirectPath,
		ExpiresAt:    expiresAt,
	}

	return CreateSessionTicketResult{
		Ticket:      token,
		RedirectURL: "/auth/session#ticket=" + token,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *Service) ConsumeTicket(ctx context.Context, ticket string) (ConsumeSessionTicketResult, error) {
	if s == nil {
		return ConsumeSessionTicketResult{}, errors.New("web session service unavailable")
	}
	token := strings.TrimSpace(ticket)
	if token == "" {
		return ConsumeSessionTicketResult{}, ErrInvalidTicket
	}

	if s.repo != nil {
		var result ConsumeSessionTicketResult
		err := s.repo.WithTx(ctx, func(tx TicketTx) error {
			if err := tx.CleanupExpiredWebSessionTickets(ctx); err != nil {
				return err
			}
			row, err := tx.ConsumeWebSessionTicket(ctx, hashTicketToken(token))
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrInvalidTicket
				}
				return err
			}
			result = ConsumeSessionTicketResult{
				UserID:       row.UserID,
				KeyID:        row.KeyID,
				RedirectPath: row.RedirectPath,
			}
			return nil
		})
		if err != nil {
			return ConsumeSessionTicketResult{}, err
		}
		return result, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(s.nowUTC())
	row, ok := s.tickets[token]
	if !ok {
		return ConsumeSessionTicketResult{}, ErrInvalidTicket
	}
	delete(s.tickets, token)
	return ConsumeSessionTicketResult{
		UserID:       row.UserID,
		KeyID:        row.KeyID,
		RedirectPath: row.RedirectPath,
	}, nil
}

func (s *Service) nowUTC() time.Time {
	now := s.now
	if now == nil {
		now = time.Now
	}
	return now().UTC()
}

func (s *Service) newTicketToken() (string, error) {
	if s.newToken != nil {
		return s.newToken()
	}
	return randomTicketToken()
}

func (s *Service) cleanupExpiredLocked(now time.Time) {
	for token, ticket := range s.tickets {
		if now.After(ticket.ExpiresAt) {
			delete(s.tickets, token)
		}
	}
}

func validateRedirectPath(redirectPath string) error {
	if strings.TrimSpace(redirectPath) == "" {
		return ErrInvalidRedirectPath
	}
	if !strings.HasPrefix(redirectPath, "/") || strings.HasPrefix(redirectPath, "//") {
		return ErrInvalidRedirectPath
	}
	parsed, err := url.Parse(redirectPath)
	if err != nil {
		return ErrInvalidRedirectPath
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return ErrInvalidRedirectPath
	}
	if parsed.Path == "" {
		return ErrInvalidRedirectPath
	}
	return nil
}

func randomTicketToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if token == "" {
		return "", errors.New("failed to generate session ticket")
	}
	return token, nil
}

func hashTicketToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
