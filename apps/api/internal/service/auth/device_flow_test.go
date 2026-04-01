package auth

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"labkit.local/apps/api/internal/config"
	"labkit.local/packages/go/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const testAuthorizedKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJW3soU0xa1q9zZXbJC4wK3dV6mna4Cfea3/1s8LUbT9"

func TestCreateDeviceAuthorizationRequestPersistsPendingRequest(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, &fakeOAuthClient{})

	result, err := svc.CreateDeviceAuthorizationRequest(context.Background(), CreateDeviceAuthRequestInput{
		PublicKey: testAuthorizedKey,
	})
	if err != nil {
		t.Fatalf("CreateDeviceAuthorizationRequest() error = %v", err)
	}
	if result.DeviceCode != "device-code" {
		t.Fatalf("DeviceCode = %q, want %q", result.DeviceCode, "device-code")
	}
	if result.UserCode != "user-code" {
		t.Fatalf("UserCode = %q, want %q", result.UserCode, "user-code")
	}
	if result.OAuthState != "oauth-state" {
		t.Fatalf("OAuthState = %q, want %q", result.OAuthState, "oauth-state")
	}
	if result.VerificationURL != "/api/device/verify" {
		t.Fatalf("VerificationURL = %q, want %q", result.VerificationURL, "/api/device/verify")
	}
	if got := repo.requestsByDeviceCode["device-code"].Status; got != deviceAuthPending {
		t.Fatalf("stored status = %q, want %q", got, deviceAuthPending)
	}
	if got := repo.requestsByDeviceCode["device-code"].PublicKey; got != testAuthorizedKey {
		t.Fatalf("stored public key = %q, want %q", got, testAuthorizedKey)
	}
}

func TestPollDeviceAuthorizationRequestReturnsPending(t *testing.T) {
	repo := newFakeRepository()
	repo.requestsByDeviceCode["device-code"] = sqlc.DeviceAuthRequests{
		DeviceCode: "device-code",
		UserCode:   "user-code",
		Status:     deviceAuthPending,
	}
	svc := newTestService(repo, &fakeOAuthClient{})

	result, err := svc.PollDeviceAuthorizationRequest(context.Background(), "device-code")
	if err != nil {
		t.Fatalf("PollDeviceAuthorizationRequest() error = %v", err)
	}
	if result.Status != deviceAuthPending {
		t.Fatalf("Status = %q, want %q", result.Status, deviceAuthPending)
	}
}

func TestPollDeviceAuthorizationRequestReturnsApproved(t *testing.T) {
	repo := newFakeRepository()
	repo.requestsByDeviceCode["device-code"] = sqlc.DeviceAuthRequests{
		DeviceCode: "device-code",
		UserCode:   "user-code",
		PublicKey:  testAuthorizedKey,
		Status:     deviceAuthApproved,
		StudentID:  pgtype.Text{String: "2026001", Valid: true},
	}
	repo.keysByPublicKey[testAuthorizedKey] = sqlc.UserKeys{ID: 11, PublicKey: testAuthorizedKey}
	svc := newTestService(repo, &fakeOAuthClient{})

	result, err := svc.PollDeviceAuthorizationRequest(context.Background(), "device-code")
	if err != nil {
		t.Fatalf("PollDeviceAuthorizationRequest() error = %v", err)
	}
	if result.Status != deviceAuthApproved {
		t.Fatalf("Status = %q, want %q", result.Status, deviceAuthApproved)
	}
	if result.StudentID != "2026001" {
		t.Fatalf("StudentID = %q, want %q", result.StudentID, "2026001")
	}
	if result.KeyID != 11 {
		t.Fatalf("KeyID = %d, want %d", result.KeyID, 11)
	}
}

func TestHandleOAuthCallbackRejectsInvalidState(t *testing.T) {
	repo := newFakeRepository()
	oauth := &fakeOAuthClient{}
	svc := newTestService(repo, oauth)

	_, err := svc.HandleOAuthCallback(context.Background(), "bad-state", "code")
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("HandleOAuthCallback() error = %v, want ErrInvalidState", err)
	}
	if oauth.exchangeCalls != 0 {
		t.Fatalf("ExchangeCode called %d times, want 0", oauth.exchangeCalls)
	}
}

func TestHandleOAuthCallbackExchangesCodeAndBindsPublicKeyToUser(t *testing.T) {
	repo := newFakeRepository()
	request := sqlc.DeviceAuthRequests{
		DeviceCode: "device-code",
		UserCode:   "user-code",
		PublicKey:  testAuthorizedKey,
		Status:     deviceAuthPending,
		OauthState: pgtype.Text{String: "oauth-state", Valid: true},
	}
	repo.requestsByState["oauth-state"] = request
	repo.requestsByDeviceCode["device-code"] = request
	oauth := &fakeOAuthClient{
		accessToken: "access-token",
		profile:     OAuthProfile{LoginName: "2026001"},
	}
	svc := newTestService(repo, oauth)

	result, err := svc.HandleOAuthCallback(context.Background(), "oauth-state", "auth-code")
	if err != nil {
		t.Fatalf("HandleOAuthCallback() error = %v", err)
	}
	if result.StudentID != "2026001" {
		t.Fatalf("StudentID = %q, want %q", result.StudentID, "2026001")
	}
	if got := repo.requestsByDeviceCode["device-code"].Status; got != deviceAuthApproved {
		t.Fatalf("device request status = %q, want %q", got, deviceAuthApproved)
	}
	if got := repo.usersByStudentID["2026001"].ID; got != 1 {
		t.Fatalf("user id = %d, want %d", got, 1)
	}
	if got := repo.keysByID[result.UserKeyID].PublicKey; got != testAuthorizedKey {
		t.Fatalf("bound public key = %q, want %q", got, testAuthorizedKey)
	}
	if oauth.exchangeCalls != 1 {
		t.Fatalf("ExchangeCode called %d times, want 1", oauth.exchangeCalls)
	}
	if oauth.profileCalls != 1 {
		t.Fatalf("FetchProfile called %d times, want 1", oauth.profileCalls)
	}
	if repo.completeCalls != 1 {
		t.Fatalf("CompleteDeviceAuthRequest called %d times, want 1", repo.completeCalls)
	}
}

func TestHandleOAuthCallbackDoesNotApproveRequestWhenKeyBindingFails(t *testing.T) {
	repo := newFakeRepository()
	request := sqlc.DeviceAuthRequests{
		DeviceCode: "device-code",
		UserCode:   "user-code",
		PublicKey:  testAuthorizedKey,
		Status:     deviceAuthPending,
		OauthState: pgtype.Text{String: "oauth-state", Valid: true},
	}
	repo.requestsByState["oauth-state"] = request
	repo.requestsByDeviceCode["device-code"] = request
	repo.failComplete = errors.New("complete request failed")
	oauth := &fakeOAuthClient{
		accessToken: "access-token",
		profile:     OAuthProfile{LoginName: "2026001"},
	}
	svc := newTestService(repo, oauth)

	_, err := svc.HandleOAuthCallback(context.Background(), "oauth-state", "auth-code")
	if err == nil {
		t.Fatalf("HandleOAuthCallback() error = nil, want error")
	}
	if got := repo.requestsByDeviceCode["device-code"].Status; got != deviceAuthPending {
		t.Fatalf("device request status = %q, want %q", got, deviceAuthPending)
	}
	if _, ok := repo.keysByID[1]; ok {
		t.Fatalf("unexpected user key inserted")
	}
	if repo.completeCalls != 1 {
		t.Fatalf("CompleteDeviceAuthRequest called %d times, want 1", repo.completeCalls)
	}
}

func TestBeginDeviceVerificationIncludesScopeAndConfiguredRedirect(t *testing.T) {
	repo := newFakeRepository()
	request := sqlc.DeviceAuthRequests{
		DeviceCode: "device-code",
		UserCode:   "user-code",
		PublicKey:  testAuthorizedKey,
		Status:     deviceAuthPending,
		OauthState: pgtype.Text{String: "oauth-state", Valid: true},
		ExpiresAt:  pgtype.Timestamptz{Time: time.Unix(1700000000, 0).UTC().Add(time.Minute), Valid: true},
	}
	repo.requestsByDeviceCode["device-code"] = request
	repo.requestsByState["oauth-state"] = request

	svc := NewService(repo, &fakeOAuthClient{}, config.OAuthConfig{
		ClientID:      "school-client",
		AuthorizeURL:  "https://cas.ruc.edu.cn/cas/oauth2.0/authorize",
		RedirectURL:   "https://lab.ics.astralis.icu/api/device/verify",
		DeviceAuthTTL: time.Minute,
	})
	svc.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	got, err := svc.BeginDeviceVerification(context.Background(), "user-code")
	if err != nil {
		t.Fatalf("BeginDeviceVerification() error = %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "cas.ruc.edu.cn" {
		t.Fatalf("authorize url = %q, want CAS host", got)
	}
	query := parsed.Query()
	if query.Get("response_type") != "code" {
		t.Fatalf("response_type = %q, want code", query.Get("response_type"))
	}
	if query.Get("client_id") != "school-client" {
		t.Fatalf("client_id = %q, want school-client", query.Get("client_id"))
	}
	if query.Get("redirect_uri") != "https://lab.ics.astralis.icu/api/device/verify" {
		t.Fatalf("redirect_uri = %q, want production callback", query.Get("redirect_uri"))
	}
	if query.Get("state") != "oauth-state" {
		t.Fatalf("state = %q, want oauth-state", query.Get("state"))
	}
	if query.Get("scope") != "all" {
		t.Fatalf("scope = %q, want all", query.Get("scope"))
	}
}

func TestDevBindDeviceAuthorizationRequestApprovesPendingRequest(t *testing.T) {
	repo := newFakeRepository()
	request := sqlc.DeviceAuthRequests{
		DeviceCode: "device-code",
		UserCode:   "user-code",
		PublicKey:  testAuthorizedKey,
		Status:     deviceAuthPending,
		OauthState: pgtype.Text{String: "oauth-state", Valid: true},
		ExpiresAt:  pgtype.Timestamptz{Time: time.Unix(1700000000, 0).UTC().Add(time.Minute), Valid: true},
	}
	repo.requestsByDeviceCode["device-code"] = request
	repo.requestsByState["oauth-state"] = request

	svc := newTestService(repo, &fakeOAuthClient{})

	result, err := svc.DevBindDeviceAuthorizationRequest(context.Background(), DevBindDeviceAuthRequestInput{
		DeviceCode: "device-code",
		StudentID:  "2026e2e",
		DeviceName: "e2e-script",
	})
	if err != nil {
		t.Fatalf("DevBindDeviceAuthorizationRequest() error = %v", err)
	}
	if result.Status != deviceAuthApproved {
		t.Fatalf("Status = %q, want %q", result.Status, deviceAuthApproved)
	}
	if result.StudentID != "2026e2e" {
		t.Fatalf("StudentID = %q, want %q", result.StudentID, "2026e2e")
	}
	if result.KeyID == 0 {
		t.Fatal("KeyID was not populated")
	}
	if got := repo.requestsByDeviceCode["device-code"].Status; got != deviceAuthApproved {
		t.Fatalf("stored status = %q, want %q", got, deviceAuthApproved)
	}
}

func TestDevBindDeviceAuthorizationRequestRejectsBlankStudentID(t *testing.T) {
	svc := newTestService(newFakeRepository(), &fakeOAuthClient{})

	_, err := svc.DevBindDeviceAuthorizationRequest(context.Background(), DevBindDeviceAuthRequestInput{
		DeviceCode: "device-code",
		StudentID:  "   ",
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("DevBindDeviceAuthorizationRequest() error = %v, want ErrInvalidRequest", err)
	}
}

func newTestService(repo *fakeRepository, oauth OAuthClient) *Service {
	svc := NewService(repo, oauth, config.OAuthConfig{DeviceAuthTTL: time.Minute})
	svc.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	svc.newDeviceCode = func() (string, error) { return "device-code", nil }
	svc.newUserCode = func() (string, error) { return "user-code", nil }
	svc.newState = func() (string, error) { return "oauth-state", nil }
	return svc
}

type fakeRepository struct {
	requestsByDeviceCode map[string]sqlc.DeviceAuthRequests
	requestsByState      map[string]sqlc.DeviceAuthRequests
	usersByStudentID     map[string]sqlc.Users
	keysByID             map[int64]sqlc.UserKeys
	keysByPublicKey      map[string]sqlc.UserKeys
	nextUserID           int64
	nextKeyID            int64
	completeCalls        int
	failComplete         error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		requestsByDeviceCode: make(map[string]sqlc.DeviceAuthRequests),
		requestsByState:      make(map[string]sqlc.DeviceAuthRequests),
		usersByStudentID:     make(map[string]sqlc.Users),
		keysByID:             make(map[int64]sqlc.UserKeys),
		keysByPublicKey:      make(map[string]sqlc.UserKeys),
		nextUserID:           1,
		nextKeyID:            1,
	}
}

func (r *fakeRepository) CreateDeviceAuthRequest(_ context.Context, arg sqlc.CreateDeviceAuthRequestParams) (sqlc.DeviceAuthRequests, error) {
	row := sqlc.DeviceAuthRequests{
		DeviceCode: arg.DeviceCode,
		UserCode:   arg.UserCode,
		PublicKey:  arg.PublicKey,
		Status:     arg.Status,
		OauthState: arg.OauthState,
		ExpiresAt:  arg.ExpiresAt,
	}
	r.requestsByDeviceCode[arg.DeviceCode] = row
	if arg.OauthState.Valid {
		r.requestsByState[arg.OauthState.String] = row
	}
	return row, nil
}

func (r *fakeRepository) GetDeviceAuthRequestByDeviceCode(_ context.Context, deviceCode string) (sqlc.DeviceAuthRequests, error) {
	row, ok := r.requestsByDeviceCode[deviceCode]
	if !ok {
		return sqlc.DeviceAuthRequests{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *fakeRepository) GetPendingDeviceAuthRequestByOAuthState(_ context.Context, state string) (sqlc.DeviceAuthRequests, error) {
	row, ok := r.requestsByState[state]
	if !ok || row.Status != deviceAuthPending {
		return sqlc.DeviceAuthRequests{}, pgx.ErrNoRows
	}
	return row, nil
}

func (r *fakeRepository) GetPendingDeviceAuthRequestByUserCode(_ context.Context, userCode string) (sqlc.DeviceAuthRequests, error) {
	for _, row := range r.requestsByDeviceCode {
		if row.UserCode == userCode && row.Status == deviceAuthPending {
			return row, nil
		}
	}
	return sqlc.DeviceAuthRequests{}, pgx.ErrNoRows
}

func (r *fakeRepository) CompleteDeviceAuthRequest(_ context.Context, arg sqlc.CompleteDeviceAuthRequestParams) (sqlc.CompleteDeviceAuthRequestRow, error) {
	r.completeCalls++
	if r.failComplete != nil {
		return sqlc.CompleteDeviceAuthRequestRow{}, r.failComplete
	}
	row, ok := r.requestsByDeviceCode[arg.DeviceCode]
	if !ok {
		return sqlc.CompleteDeviceAuthRequestRow{}, pgx.ErrNoRows
	}
	if !row.OauthState.Valid || row.OauthState.String != arg.OauthState.String || row.Status != deviceAuthPending {
		return sqlc.CompleteDeviceAuthRequestRow{}, pgx.ErrNoRows
	}
	row.Status = deviceAuthApproved
	row.StudentID = pgtype.Text{String: arg.StudentID.String, Valid: arg.StudentID.Valid}
	r.requestsByDeviceCode[arg.DeviceCode] = row
	if row.OauthState.Valid {
		r.requestsByState[row.OauthState.String] = row
	}
	user, ok := r.usersByStudentID[arg.StudentID.String]
	if !ok {
		user = sqlc.Users{ID: r.nextUserID, StudentID: arg.StudentID.String}
		r.nextUserID++
		r.usersByStudentID[arg.StudentID.String] = user
	}
	key := sqlc.UserKeys{
		ID:         r.nextKeyID,
		UserID:     user.ID,
		PublicKey:  row.PublicKey,
		DeviceName: arg.DeviceName,
	}
	r.nextKeyID++
	r.keysByID[key.ID] = key
	r.keysByPublicKey[key.PublicKey] = key
	return sqlc.CompleteDeviceAuthRequestRow{
		UserID:    user.ID,
		UserKeyID: key.ID,
		StudentID: arg.StudentID.String,
	}, nil
}

func (r *fakeRepository) GetUserKeyByPublicKey(_ context.Context, publicKey string) (sqlc.UserKeys, error) {
	key, ok := r.keysByPublicKey[publicKey]
	if !ok {
		return sqlc.UserKeys{}, pgx.ErrNoRows
	}
	return key, nil
}

type fakeOAuthClient struct {
	accessToken   string
	profile       OAuthProfile
	exchangeCalls int
	profileCalls  int
}

func (f *fakeOAuthClient) ExchangeCode(_ context.Context, _ string) (string, error) {
	f.exchangeCalls++
	if f.accessToken == "" {
		return "access-token", nil
	}
	return f.accessToken, nil
}

func (f *fakeOAuthClient) FetchProfile(_ context.Context, _ string) (OAuthProfile, error) {
	f.profileCalls++
	if f.profile.LoginName == "" {
		return OAuthProfile{LoginName: "2026001"}, nil
	}
	return f.profile, nil
}
