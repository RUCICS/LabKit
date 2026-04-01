package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"labkit.local/apps/api/internal/config"
	authsvc "labkit.local/apps/api/internal/service/auth"
	authproviders "labkit.local/apps/api/internal/service/auth/providers"
	personalsvc "labkit.local/apps/api/internal/service/personal"
	"labkit.local/packages/go/auth"
	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const testAuthorizedKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJW3soU0xa1q9zZXbJC4wK3dV6mna4Cfea3/1s8LUbT9"

func TestRouterWiresDeviceAuthRoutes(t *testing.T) {
	repo := newRouterAuthRepo()
	svc := authsvc.NewService(repo, noopOAuthClient{}, config.OAuthConfig{})
	router := NewRouter(WithAuthService(svc))

	authorizeReq := httptest.NewRequest(http.MethodPost, "/api/device/authorize", bytes.NewBufferString(`{"public_key":"`+testAuthorizedKey+`"}`))
	authorizeRR := httptest.NewRecorder()
	router.ServeHTTP(authorizeRR, authorizeReq)
	if authorizeRR.Code != http.StatusCreated {
		t.Fatalf("authorize status = %d, want %d body=%s", authorizeRR.Code, http.StatusCreated, authorizeRR.Body.String())
	}
	var authorizePayload struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURL string `json:"verification_url"`
	}
	if err := json.Unmarshal(authorizeRR.Body.Bytes(), &authorizePayload); err != nil {
		t.Fatalf("unmarshal authorize response: %v", err)
	}
	if authorizePayload.DeviceCode == "" {
		t.Fatal("authorize response missing device_code")
	}
	if authorizePayload.VerificationURL != "/api/device/verify" {
		t.Fatalf("verification_url = %q, want %q", authorizePayload.VerificationURL, "/api/device/verify")
	}

	repo.requestsByDeviceCode[authorizePayload.DeviceCode] = sqlc.DeviceAuthRequests{
		DeviceCode: authorizePayload.DeviceCode,
		UserCode:   authorizePayload.UserCode,
		PublicKey:  testAuthorizedKey,
		StudentID:  pgtype.Text{String: "2026001", Valid: true},
		Status:     "approved",
	}
	repo.keysByPublicKey[testAuthorizedKey] = sqlc.UserKeys{ID: 11, PublicKey: testAuthorizedKey}

	pollReq := httptest.NewRequest(http.MethodPost, "/api/device/poll?device_code="+authorizePayload.DeviceCode, nil)
	pollRR := httptest.NewRecorder()
	router.ServeHTTP(pollRR, pollReq)
	if pollRR.Code != http.StatusOK {
		t.Fatalf("poll status = %d, want %d body=%s", pollRR.Code, http.StatusOK, pollRR.Body.String())
	}
	var pollPayload struct {
		Status string `json:"status"`
		KeyID  int64  `json:"key_id"`
	}
	if err := json.Unmarshal(pollRR.Body.Bytes(), &pollPayload); err != nil {
		t.Fatalf("unmarshal poll response: %v", err)
	}
	if pollPayload.Status != "approved" {
		t.Fatalf("status = %q, want %q", pollPayload.Status, "approved")
	}
	if pollPayload.KeyID != 11 {
		t.Fatalf("key_id = %d, want %d", pollPayload.KeyID, 11)
	}
}

func TestRouterWiresDeviceVerifyRoute(t *testing.T) {
	repo := newRouterAuthRepo()
	svc := authsvc.NewServiceWithProvider(repo, authproviders.NewCASRUCProvider(verifyOAuthClient{}, config.CASRUCConfig{
		ClientID:     "labkit-client",
		AuthorizeURL: "https://sso.example.edu/oauth2/authorize",
		RedirectURL:  "/api/device/verify",
	}), config.OAuthConfig{
		CASRUC: config.CASRUCConfig{
			ClientID:     "labkit-client",
			AuthorizeURL: "https://sso.example.edu/oauth2/authorize",
			RedirectURL:  "/api/device/verify",
		},
	})
	router := NewRouter(WithAuthService(svc))

	authorizeReq := httptest.NewRequest(http.MethodPost, "/api/device/authorize", bytes.NewBufferString(`{"public_key":"`+testAuthorizedKey+`"}`))
	authorizeRR := httptest.NewRecorder()
	router.ServeHTTP(authorizeRR, authorizeReq)
	if authorizeRR.Code != http.StatusCreated {
		t.Fatalf("authorize status = %d, want %d body=%s", authorizeRR.Code, http.StatusCreated, authorizeRR.Body.String())
	}
	var authorizePayload struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
		OAuthState string `json:"oauth_state"`
	}
	if err := json.Unmarshal(authorizeRR.Body.Bytes(), &authorizePayload); err != nil {
		t.Fatalf("unmarshal authorize response: %v", err)
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/api/device/verify?user_code="+url.QueryEscape(authorizePayload.UserCode), nil)
	verifyRR := httptest.NewRecorder()
	router.ServeHTTP(verifyRR, verifyReq)
	if verifyRR.Code != http.StatusFound {
		t.Fatalf("verify status = %d, want %d body=%s", verifyRR.Code, http.StatusFound, verifyRR.Body.String())
	}
	location := verifyRR.Header().Get("Location")
	if location == "" {
		t.Fatal("verify response missing Location header")
	}
	if want := "https://sso.example.edu/oauth2/authorize"; !strings.HasPrefix(location, want) {
		t.Fatalf("verify redirect = %q, want prefix %q", location, want)
	}
	locationURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse Location header: %v", err)
	}
	if got := locationURL.Query().Get("state"); got != authorizePayload.OAuthState {
		t.Fatalf("verify redirect state = %q, want %q", got, authorizePayload.OAuthState)
	}
	if got := locationURL.Query().Get("redirect_uri"); got != "/api/device/verify" {
		t.Fatalf("verify redirect redirect_uri = %q, want %q", got, "/api/device/verify")
	}

	repo.requestsByState[authorizePayload.OAuthState] = sqlc.DeviceAuthRequests{
		DeviceCode: authorizePayload.DeviceCode,
		UserCode:   authorizePayload.UserCode,
		PublicKey:  testAuthorizedKey,
		StudentID:  pgtype.Text{String: "2026001", Valid: true},
		OauthState: pgtype.Text{String: authorizePayload.OAuthState, Valid: true},
		Status:     "pending",
	}
	repo.keysByPublicKey[testAuthorizedKey] = sqlc.UserKeys{ID: 11, PublicKey: testAuthorizedKey}

	callbackReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/device/verify?state=%s&code=auth-code", url.QueryEscape(authorizePayload.OAuthState)), nil)
	callbackRR := httptest.NewRecorder()
	router.ServeHTTP(callbackRR, callbackReq)
	if callbackRR.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want %d body=%s", callbackRR.Code, http.StatusFound, callbackRR.Body.String())
	}
	if got := callbackRR.Header().Get("Location"); got != "/auth/confirm?student_id=2026001&user_key_id=11" {
		t.Fatalf("callback location = %q, want %q", got, "/auth/confirm?student_id=2026001&user_key_id=11")
	}
	if cookies := callbackRR.Result().Cookies(); len(cookies) == 0 {
		t.Fatal("callback response missing browser session cookie")
	}
}

func TestRouterRequiresSignedAuthForWebSessionTicket(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	router := NewRouter(WithPersonalService(personalsvc.NewService(repo)))

	req := httptest.NewRequest(http.MethodPost, "/api/web/session-ticket", bytes.NewBufferString(`{"redirect_path":"/auth/confirm"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusUnauthorized, "", "unauthorized", "Unauthorized")
}

func TestRouterCreatesAndConsumesWebSessionTicket(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	router := NewRouter(WithPersonalService(personalsvc.NewService(repo)))

	createBody := []byte(`{"redirect_path":"/auth/confirm"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/web/session-ticket", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	repo.signRequest(t, createReq, "/api/web/session-ticket", createBody, "nonce-web-session-ticket", 11)

	createRR := httptest.NewRecorder()
	router.ServeHTTP(createRR, createReq)

	if createRR.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d, want %d body=%s", createRR.Code, http.StatusCreated, createRR.Body.String())
	}

	var createPayload struct {
		Ticket      string `json:"ticket"`
		RedirectURL string `json:"redirect_url"`
		ExpiresAt   string `json:"expires_at"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if createPayload.Ticket == "" {
		t.Fatal("create response missing ticket")
	}
	if got, want := createPayload.RedirectURL, "/auth/session#ticket="+createPayload.Ticket; got != want {
		t.Fatalf("redirect_url = %q, want %q", got, want)
	}
	if createPayload.ExpiresAt == "" {
		t.Fatal("create response missing expires_at")
	}

	shellReq := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	shellRR := httptest.NewRecorder()
	router.ServeHTTP(shellRR, shellReq)

	if shellRR.Code != http.StatusOK {
		t.Fatalf("shell status = %d, want %d body=%s", shellRR.Code, http.StatusOK, shellRR.Body.String())
	}
	if got := shellRR.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("shell cache-control = %q, want %q", got, "no-store")
	}
	if got := shellRR.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("shell content type = %q, want %q", got, "text/html; charset=utf-8")
	}
	if body := shellRR.Body.String(); !strings.Contains(body, "/auth/session/exchange") || !strings.Contains(body, "history.replaceState") {
		t.Fatalf("shell body missing exchange script: %s", body)
	}

	exchangeBody := strings.NewReader("ticket=" + url.QueryEscape(createPayload.Ticket))
	exchangeReq := httptest.NewRequest(http.MethodPost, "/auth/session/exchange", exchangeBody)
	exchangeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	exchangeRR := httptest.NewRecorder()
	router.ServeHTTP(exchangeRR, exchangeReq)

	if exchangeRR.Code != http.StatusFound {
		t.Fatalf("exchange status = %d, want %d body=%s", exchangeRR.Code, http.StatusFound, exchangeRR.Body.String())
	}
	if got := exchangeRR.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("exchange cache-control = %q, want %q", got, "no-store")
	}
	if got, want := exchangeRR.Header().Get("Location"), "/auth/confirm?mode=web-session"; got != want {
		t.Fatalf("exchange location = %q, want %q", got, want)
	}
	if cookies := exchangeRR.Result().Cookies(); len(cookies) == 0 {
		t.Fatal("exchange response missing browser session cookie")
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/auth/session/exchange", strings.NewReader("ticket="+url.QueryEscape(createPayload.Ticket)))
	replayReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	replayRR := httptest.NewRecorder()
	router.ServeHTTP(replayRR, replayReq)
	assertStructuredErrorResponse(t, replayRR, http.StatusBadRequest, "", "invalid_session_ticket", "invalid session ticket")
}

func TestRouterRejectsOpenRedirectInWebSessionTicket(t *testing.T) {
	repo := newPersonalTestRepo(t, true)
	router := NewRouter(WithPersonalService(personalsvc.NewService(repo)))

	body := []byte(`{"redirect_path":"https://evil.example"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/web/session-ticket", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	fingerprint := auth.PublicKeyFingerprint(repo.private.Public().(ed25519.PublicKey))
	req.Header.Set("X-LabKit-Key-Fingerprint", fingerprint)
	req.Header.Set("X-LabKit-Timestamp", time.Now().UTC().Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", "nonce-web-session-open-redirect")
	req.Header.Set("X-LabKit-Signature", "")
	repo.signRequest(t, req, "/api/web/session-ticket", body, "nonce-web-session-open-redirect", 11)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assertStructuredErrorResponse(t, rr, http.StatusBadRequest, "", "invalid_request", "invalid redirect path")
}

func TestRouterRedirectsOAuthCallbackToAuthConfirmAndIssuesBrowserSession(t *testing.T) {
	repo := newRouterAuthRepo()
	authSvc := authsvc.NewServiceWithProvider(repo, authproviders.NewCASRUCProvider(verifyOAuthClient{}, config.CASRUCConfig{
		ClientID:     "labkit-client",
		AuthorizeURL: "https://sso.example.edu/oauth2/authorize",
		RedirectURL:  "/api/device/verify",
	}), config.OAuthConfig{
		CASRUC: config.CASRUCConfig{
			ClientID:     "labkit-client",
			AuthorizeURL: "https://sso.example.edu/oauth2/authorize",
			RedirectURL:  "/api/device/verify",
		},
	})
	personalRepo := newPersonalTestRepo(t, true)
	router := NewRouter(
		WithAuthService(authSvc),
		WithPersonalService(personalsvc.NewService(personalRepo)),
	)

	authorizeReq := httptest.NewRequest(http.MethodPost, "/api/device/authorize", bytes.NewBufferString(`{"public_key":"`+testAuthorizedKey+`"}`))
	authorizeRR := httptest.NewRecorder()
	router.ServeHTTP(authorizeRR, authorizeReq)
	if authorizeRR.Code != http.StatusCreated {
		t.Fatalf("authorize status = %d, want %d body=%s", authorizeRR.Code, http.StatusCreated, authorizeRR.Body.String())
	}

	var authorizePayload struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
		OAuthState string `json:"oauth_state"`
	}
	if err := json.Unmarshal(authorizeRR.Body.Bytes(), &authorizePayload); err != nil {
		t.Fatalf("unmarshal authorize response: %v", err)
	}

	repo.requestsByState[authorizePayload.OAuthState] = sqlc.DeviceAuthRequests{
		DeviceCode: authorizePayload.DeviceCode,
		UserCode:   authorizePayload.UserCode,
		PublicKey:  testAuthorizedKey,
		StudentID:  pgtype.Text{String: "2026001", Valid: true},
		OauthState: pgtype.Text{String: authorizePayload.OAuthState, Valid: true},
		Status:     "pending",
	}
	repo.keysByPublicKey[testAuthorizedKey] = sqlc.UserKeys{ID: 11, PublicKey: testAuthorizedKey}

	callbackReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/device/verify?state=%s&code=auth-code", url.QueryEscape(authorizePayload.OAuthState)), nil)
	callbackRR := httptest.NewRecorder()
	router.ServeHTTP(callbackRR, callbackReq)
	if callbackRR.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want %d body=%s", callbackRR.Code, http.StatusFound, callbackRR.Body.String())
	}
	if got := callbackRR.Header().Get("Location"); got != "/auth/confirm?student_id=2026001&user_key_id=11" {
		t.Fatalf("callback location = %q, want %q", got, "/auth/confirm?student_id=2026001&user_key_id=11")
	}
	cookies := callbackRR.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("callback response missing browser session cookie")
	}

	keysReq := httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	for _, cookie := range cookies {
		keysReq.AddCookie(cookie)
	}
	keysRR := httptest.NewRecorder()
	router.ServeHTTP(keysRR, keysReq)
	if keysRR.Code != http.StatusOK {
		t.Fatalf("keys status = %d, want %d body=%s", keysRR.Code, http.StatusOK, keysRR.Body.String())
	}
	var keysPayload struct {
		Keys []personalsvc.Key `json:"keys"`
	}
	if err := json.Unmarshal(keysRR.Body.Bytes(), &keysPayload); err != nil {
		t.Fatalf("unmarshal keys response: %v", err)
	}
	if len(keysPayload.Keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(keysPayload.Keys))
	}
}

func TestRouterWiresDevDeviceBindRouteWhenDevModeEnabled(t *testing.T) {
	repo := newRouterAuthRepo()
	svc := authsvc.NewService(repo, noopOAuthClient{}, config.OAuthConfig{})
	router := NewRouter(WithAuthService(svc), WithDevMode(true))

	repo.requestsByDeviceCode["device-code"] = sqlc.DeviceAuthRequests{
		DeviceCode: "device-code",
		UserCode:   "user-code",
		PublicKey:  testAuthorizedKey,
		Status:     "pending",
		OauthState: pgtype.Text{String: "oauth-state", Valid: true},
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(time.Minute), Valid: true},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/dev/device/bind", bytes.NewBufferString(`{"device_code":"device-code","student_id":"2026e2e","device_name":"e2e-script"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload struct {
		Status    string `json:"status"`
		StudentID string `json:"student_id"`
		KeyID     int64  `json:"key_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Status != "approved" {
		t.Fatalf("status = %q, want %q", payload.Status, "approved")
	}
	if payload.StudentID != "2026e2e" {
		t.Fatalf("student_id = %q, want %q", payload.StudentID, "2026e2e")
	}
	if payload.KeyID == 0 {
		t.Fatal("key_id was not populated")
	}
}

func TestRouterDoesNotWireDevDeviceBindRouteWhenDevModeDisabled(t *testing.T) {
	repo := newRouterAuthRepo()
	svc := authsvc.NewService(repo, noopOAuthClient{}, config.OAuthConfig{})
	router := NewRouter(WithAuthService(svc))

	req := httptest.NewRequest(http.MethodPost, "/api/dev/device/bind", bytes.NewBufferString(`{"device_code":"device-code","student_id":"2026e2e"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

type routerAuthRepo struct {
	requestsByDeviceCode map[string]sqlc.DeviceAuthRequests
	requestsByState      map[string]sqlc.DeviceAuthRequests
	keysByPublicKey      map[string]sqlc.UserKeys
	usersByID            map[int64]sqlc.Users
	nextID               int64
}

func newRouterAuthRepo() *routerAuthRepo {
	return &routerAuthRepo{
		requestsByDeviceCode: make(map[string]sqlc.DeviceAuthRequests),
		requestsByState:      make(map[string]sqlc.DeviceAuthRequests),
		keysByPublicKey:      make(map[string]sqlc.UserKeys),
		usersByID:            make(map[int64]sqlc.Users),
		nextID:               1,
	}
}

func (r *routerAuthRepo) CreateDeviceAuthRequest(_ context.Context, arg sqlc.CreateDeviceAuthRequestParams) (sqlc.DeviceAuthRequests, error) {
	row := sqlc.DeviceAuthRequests{
		ID:         r.nextID,
		DeviceCode: arg.DeviceCode,
		UserCode:   arg.UserCode,
		PublicKey:  arg.PublicKey,
		StudentID:  arg.StudentID,
		OauthState: arg.OauthState,
		Status:     arg.Status,
		ExpiresAt:  arg.ExpiresAt,
	}
	r.nextID++
	r.requestsByDeviceCode[arg.DeviceCode] = row
	if arg.OauthState.Valid {
		r.requestsByState[arg.OauthState.String] = row
	}
	return row, nil
}

func (r *routerAuthRepo) GetDeviceAuthRequestByDeviceCode(_ context.Context, deviceCode string) (sqlc.DeviceAuthRequests, error) {
	row, ok := r.requestsByDeviceCode[deviceCode]
	if !ok {
		return sqlc.DeviceAuthRequests{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *routerAuthRepo) GetPendingDeviceAuthRequestByOAuthState(_ context.Context, state string) (sqlc.DeviceAuthRequests, error) {
	row, ok := r.requestsByState[state]
	if !ok || row.Status != "pending" {
		return sqlc.DeviceAuthRequests{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *routerAuthRepo) GetPendingDeviceAuthRequestByUserCode(_ context.Context, userCode string) (sqlc.DeviceAuthRequests, error) {
	for _, row := range r.requestsByDeviceCode {
		if row.UserCode == userCode && row.Status == "pending" {
			return row, nil
		}
	}
	return sqlc.DeviceAuthRequests{}, pgx.ErrNoRows
}

func (r *routerAuthRepo) CompleteDeviceAuthRequest(_ context.Context, arg sqlc.CompleteDeviceAuthRequestParams) (sqlc.CompleteDeviceAuthRequestRow, error) {
	return sqlc.CompleteDeviceAuthRequestRow{UserID: 7, UserKeyID: 11, StudentID: arg.StudentID.String}, nil
}

func (r *routerAuthRepo) GetUserKeyByPublicKey(_ context.Context, publicKey string) (sqlc.UserKeys, error) {
	key, ok := r.keysByPublicKey[publicKey]
	if !ok {
		return sqlc.UserKeys{}, pgx.ErrNoRows
	}
	return key, nil
}

func (r *routerAuthRepo) GetUserByID(_ context.Context, id int64) (sqlc.Users, error) {
	user, ok := r.usersByID[id]
	if !ok {
		return sqlc.Users{}, pgx.ErrNoRows
	}
	return user, nil
}

type noopOAuthClient struct{}

func (noopOAuthClient) ExchangeCode(context.Context, string) (string, error) {
	return "", nil
}

func (noopOAuthClient) FetchProfile(context.Context, string) (authsvc.OAuthProfile, error) {
	return authsvc.OAuthProfile{}, nil
}

type verifyOAuthClient struct{}

func (verifyOAuthClient) ExchangeCode(context.Context, string) (string, error) {
	return "access-token", nil
}

func (verifyOAuthClient) FetchProfile(context.Context, string) (authsvc.OAuthProfile, error) {
	return authsvc.OAuthProfile{LoginName: "2026001"}, nil
}
