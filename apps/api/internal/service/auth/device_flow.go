package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"labkit.local/apps/api/internal/config"
	sharedauth "labkit.local/packages/go/auth"
	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	deviceAuthPending      = "pending"
	deviceAuthApproved     = "approved"
	deviceAuthExpired      = "expired"
	defaultDeviceAuthTTL   = 10 * time.Minute
	defaultDeviceName      = "unknown"
	defaultVerificationURL = "/api/device/verify"
)

var (
	ErrInvalidState   = errors.New("invalid oauth state")
	ErrInvalidCode    = errors.New("oauth code exchange failed")
	ErrInvalidRequest = errors.New("invalid device authorization request")
)

// Repository defines the DB operations needed by the device authorization flow.
type Repository interface {
	CreateDeviceAuthRequest(context.Context, sqlc.CreateDeviceAuthRequestParams) (sqlc.DeviceAuthRequests, error)
	GetDeviceAuthRequestByDeviceCode(context.Context, string) (sqlc.DeviceAuthRequests, error)
	GetPendingDeviceAuthRequestByUserCode(context.Context, string) (sqlc.DeviceAuthRequests, error)
	GetPendingDeviceAuthRequestByOAuthState(context.Context, string) (sqlc.DeviceAuthRequests, error)
	CompleteDeviceAuthRequest(context.Context, sqlc.CompleteDeviceAuthRequestParams) (sqlc.CompleteDeviceAuthRequestRow, error)
	GetUserKeyByPublicKey(context.Context, string) (sqlc.UserKeys, error)
}

// OAuthClient exchanges codes and fetches the identity profile.
type OAuthClient interface {
	ExchangeCode(context.Context, string) (string, error)
	FetchProfile(context.Context, string) (OAuthProfile, error)
}

// OAuthProfile is the minimal user profile required by the callback flow.
type OAuthProfile struct {
	LoginName string
}

type Service struct {
	repo          Repository
	oauth         OAuthClient
	cfg           config.OAuthConfig
	now           func() time.Time
	newDeviceCode func() (string, error)
	newUserCode   func() (string, error)
	newState      func() (string, error)
}

type CreateDeviceAuthRequestInput struct {
	PublicKey string
}

type CreateDeviceAuthRequestResult struct {
	DeviceCode      string    `json:"device_code"`
	UserCode        string    `json:"user_code"`
	VerificationURL string    `json:"verification_url"`
	OAuthState      string    `json:"oauth_state"`
	Status          string    `json:"status"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type PollDeviceAuthRequestResult struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	Status     string `json:"status"`
	StudentID  string `json:"student_id,omitempty"`
	KeyID      int64  `json:"key_id,omitempty"`
}

type OAuthCallbackResult struct {
	StudentID string `json:"student_id"`
	UserID    int64  `json:"user_id"`
	UserKeyID int64  `json:"user_key_id"`
}

type DevBindDeviceAuthRequestInput struct {
	DeviceCode string
	StudentID  string
	DeviceName string
}

type DevBindDeviceAuthRequestResult struct {
	DeviceCode string `json:"device_code"`
	StudentID  string `json:"student_id"`
	UserID     int64  `json:"user_id"`
	KeyID      int64  `json:"key_id"`
	Status     string `json:"status"`
}

func NewService(repo Repository, oauth OAuthClient, cfg config.OAuthConfig) *Service {
	return &Service{
		repo:          repo,
		oauth:         oauth,
		cfg:           cfg,
		now:           time.Now,
		newDeviceCode: func() (string, error) { return randomToken(24) },
		newUserCode:   func() (string, error) { return randomUserCode() },
		newState:      func() (string, error) { return randomToken(32) },
	}
}

func (s *Service) CreateDeviceAuthorizationRequest(ctx context.Context, in CreateDeviceAuthRequestInput) (CreateDeviceAuthRequestResult, error) {
	if strings.TrimSpace(in.PublicKey) == "" {
		return CreateDeviceAuthRequestResult{}, ErrInvalidRequest
	}
	deviceCode, err := s.newDeviceCode()
	if err != nil {
		return CreateDeviceAuthRequestResult{}, err
	}
	userCode, err := s.newUserCode()
	if err != nil {
		return CreateDeviceAuthRequestResult{}, err
	}
	state, err := s.newState()
	if err != nil {
		return CreateDeviceAuthRequestResult{}, err
	}
	now := s.nowUTC()
	ttl := s.cfg.DeviceAuthTTL
	if ttl <= 0 {
		ttl = defaultDeviceAuthTTL
	}
	row, err := s.repo.CreateDeviceAuthRequest(ctx, sqlc.CreateDeviceAuthRequestParams{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		PublicKey:  in.PublicKey,
		StudentID:  pgtype.Text{},
		OauthState: pgtype.Text{String: state, Valid: true},
		Status:     deviceAuthPending,
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(ttl), Valid: true},
	})
	if err != nil {
		return CreateDeviceAuthRequestResult{}, err
	}
	return CreateDeviceAuthRequestResult{
		DeviceCode:      row.DeviceCode,
		UserCode:        row.UserCode,
		VerificationURL: defaultVerificationURL,
		OAuthState:      state,
		Status:          row.Status,
		ExpiresAt:       row.ExpiresAt.Time,
	}, nil
}

func (s *Service) PollDeviceAuthorizationRequest(ctx context.Context, deviceCode string) (PollDeviceAuthRequestResult, error) {
	if strings.TrimSpace(deviceCode) == "" {
		return PollDeviceAuthRequestResult{}, ErrInvalidRequest
	}
	row, err := s.repo.GetDeviceAuthRequestByDeviceCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PollDeviceAuthRequestResult{}, ErrInvalidRequest
		}
		return PollDeviceAuthRequestResult{}, err
	}
	status := row.Status
	if status == deviceAuthPending && row.ExpiresAt.Valid && s.nowUTC().After(row.ExpiresAt.Time) {
		status = deviceAuthExpired
	}
	studentID := ""
	if row.StudentID.Valid {
		studentID = row.StudentID.String
	}
	keyID := int64(0)
	if status == deviceAuthApproved {
		key, err := s.repo.GetUserKeyByPublicKey(ctx, row.PublicKey)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return PollDeviceAuthRequestResult{}, err
			}
		} else {
			keyID = key.ID
		}
	}
	return PollDeviceAuthRequestResult{
		DeviceCode: row.DeviceCode,
		UserCode:   row.UserCode,
		Status:     status,
		StudentID:  studentID,
		KeyID:      keyID,
	}, nil
}

func (s *Service) BeginDeviceVerification(ctx context.Context, userCode string) (string, error) {
	if strings.TrimSpace(userCode) == "" {
		return "", ErrInvalidRequest
	}
	request, err := s.repo.GetPendingDeviceAuthRequestByUserCode(ctx, strings.TrimSpace(userCode))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrInvalidRequest
		}
		return "", err
	}
	if request.Status != deviceAuthPending {
		return "", ErrInvalidRequest
	}
	if request.ExpiresAt.Valid && s.nowUTC().After(request.ExpiresAt.Time) {
		return "", ErrInvalidRequest
	}
	authorizeURL := strings.TrimSpace(s.cfg.AuthorizeURL)
	if authorizeURL == "" {
		return "", fmt.Errorf("oauth authorize url is required")
	}
	redirectURL := strings.TrimSpace(s.cfg.RedirectURL)
	if redirectURL == "" {
		redirectURL = defaultVerificationURL
	}
	parsed, err := url.Parse(authorizeURL)
	if err != nil {
		return "", err
	}
	params := parsed.Query()
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURL)
	params.Set("state", request.OauthState.String)
	if clientID := strings.TrimSpace(s.cfg.ClientID); clientID != "" {
		params.Set("client_id", clientID)
	}
	parsed.RawQuery = params.Encode()
	return parsed.String(), nil
}

func (s *Service) DevBindDeviceAuthorizationRequest(ctx context.Context, in DevBindDeviceAuthRequestInput) (DevBindDeviceAuthRequestResult, error) {
	deviceCode := strings.TrimSpace(in.DeviceCode)
	studentID := strings.TrimSpace(in.StudentID)
	if deviceCode == "" || studentID == "" {
		return DevBindDeviceAuthRequestResult{}, ErrInvalidRequest
	}

	request, err := s.repo.GetDeviceAuthRequestByDeviceCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DevBindDeviceAuthRequestResult{}, ErrInvalidRequest
		}
		return DevBindDeviceAuthRequestResult{}, err
	}
	if request.Status != deviceAuthPending {
		return DevBindDeviceAuthRequestResult{}, ErrInvalidRequest
	}
	if request.ExpiresAt.Valid && s.nowUTC().After(request.ExpiresAt.Time) {
		return DevBindDeviceAuthRequestResult{}, ErrInvalidRequest
	}
	if !request.OauthState.Valid || strings.TrimSpace(request.OauthState.String) == "" {
		return DevBindDeviceAuthRequestResult{}, ErrInvalidState
	}
	fingerprint, err := sharedauth.AuthorizedKeyFingerprint(request.PublicKey)
	if err != nil {
		return DevBindDeviceAuthRequestResult{}, ErrInvalidRequest
	}

	deviceName := strings.TrimSpace(in.DeviceName)
	if deviceName == "" {
		deviceName = defaultDeviceName
	}

	completed, err := s.repo.CompleteDeviceAuthRequest(ctx, sqlc.CompleteDeviceAuthRequestParams{
		DeviceCode:  request.DeviceCode,
		OauthState:  pgtype.Text{String: request.OauthState.String, Valid: request.OauthState.Valid},
		StudentID:   pgtype.Text{String: studentID, Valid: studentID != ""},
		Fingerprint: fingerprint,
		DeviceName:  deviceName,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DevBindDeviceAuthRequestResult{}, ErrInvalidRequest
		}
		return DevBindDeviceAuthRequestResult{}, err
	}

	return DevBindDeviceAuthRequestResult{
		DeviceCode: request.DeviceCode,
		StudentID:  completed.StudentID,
		UserID:     completed.UserID,
		KeyID:      completed.UserKeyID,
		Status:     deviceAuthApproved,
	}, nil
}

func (s *Service) HandleOAuthCallback(ctx context.Context, state, code string) (OAuthCallbackResult, error) {
	if strings.TrimSpace(state) == "" || strings.TrimSpace(code) == "" {
		return OAuthCallbackResult{}, ErrInvalidState
	}
	request, err := s.repo.GetPendingDeviceAuthRequestByOAuthState(ctx, state)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OAuthCallbackResult{}, ErrInvalidState
		}
		return OAuthCallbackResult{}, err
	}
	if request.OauthState.Valid && request.OauthState.String != state {
		return OAuthCallbackResult{}, ErrInvalidState
	}
	if request.Status != deviceAuthPending {
		return OAuthCallbackResult{}, ErrInvalidState
	}
	if request.ExpiresAt.Valid && s.nowUTC().After(request.ExpiresAt.Time) {
		return OAuthCallbackResult{}, ErrInvalidState
	}
	accessToken, err := s.oauth.ExchangeCode(ctx, code)
	if err != nil {
		return OAuthCallbackResult{}, fmt.Errorf("%w: %v", ErrInvalidCode, err)
	}
	profile, err := s.oauth.FetchProfile(ctx, accessToken)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	studentID := strings.TrimSpace(profile.LoginName)
	if studentID == "" {
		return OAuthCallbackResult{}, ErrInvalidCode
	}
	fingerprint, err := sharedauth.AuthorizedKeyFingerprint(request.PublicKey)
	if err != nil {
		return OAuthCallbackResult{}, ErrInvalidRequest
	}
	completed, err := s.repo.CompleteDeviceAuthRequest(ctx, sqlc.CompleteDeviceAuthRequestParams{
		DeviceCode:  request.DeviceCode,
		OauthState:  pgtype.Text{String: state, Valid: state != ""},
		StudentID:   pgtype.Text{String: studentID, Valid: studentID != ""},
		Fingerprint: fingerprint,
		DeviceName:  defaultDeviceName,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OAuthCallbackResult{}, ErrInvalidState
		}
		return OAuthCallbackResult{}, err
	}
	return OAuthCallbackResult{
		StudentID: studentID,
		UserID:    completed.UserID,
		UserKeyID: completed.UserKeyID,
	}, nil
}

func (s *Service) nowUTC() time.Time {
	now := s.now
	if now == nil {
		now = time.Now
	}
	return now().UTC()
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		size = 16
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), "="), nil
}

func randomUserCode() (string, error) {
	token, err := randomToken(5)
	if err != nil {
		return "", err
	}
	token = strings.ToUpper(token)
	if len(token) < 8 {
		token += strings.Repeat("X", 8-len(token))
	}
	return token[:4] + "-" + token[4:8], nil
}
