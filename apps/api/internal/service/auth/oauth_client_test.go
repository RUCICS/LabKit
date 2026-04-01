package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"labkit.local/apps/api/internal/config"
	"labkit.local/apps/api/internal/service/auth/providers"
)

func TestOAuthHTTPClientExchangeCode(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-123"}`))
	}))
	defer srv.Close()

	client := NewOAuthHTTPClient(config.CASRUCConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
		TokenURL:     srv.URL + "/oauth/token",
		ProfileURL:   srv.URL + "/oauth/profile",
	})

	token, err := client.ExchangeCode(context.Background(), "code-abc")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if token != "token-123" {
		t.Fatalf("ExchangeCode() token = %q, want %q", token, "token-123")
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotPath != "/oauth/token" {
		t.Fatalf("path = %q, want %q", gotPath, "/oauth/token")
	}
	form, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if form.Get("code") != "code-abc" {
		t.Fatalf("code = %q, want %q", form.Get("code"), "code-abc")
	}
	if form.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q, want %q", form.Get("client_id"), "client-id")
	}
	if form.Get("client_secret") != "client-secret" {
		t.Fatalf("client_secret = %q, want %q", form.Get("client_secret"), "client-secret")
	}
	if form.Get("grant_type") != "authorization_code" {
		t.Fatalf("grant_type = %q, want %q", form.Get("grant_type"), "authorization_code")
	}
	if form.Get("redirect_uri") != "https://example.com/callback" {
		t.Fatalf("redirect_uri = %q, want %q", form.Get("redirect_uri"), "https://example.com/callback")
	}
}

func TestOAuthHTTPClientFetchProfile(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"loginName":"2026001","name":"Student"}`))
	}))
	defer srv.Close()

	client := NewOAuthHTTPClient(config.CASRUCConfig{
		ProfileURL: srv.URL + "/oauth/profile",
	})

	profile, err := client.FetchProfile(context.Background(), "token-123")
	if err != nil {
		t.Fatalf("FetchProfile() error = %v", err)
	}
	if profile.LoginName != "2026001" {
		t.Fatalf("LoginName = %q, want %q", profile.LoginName, "2026001")
	}
	if gotQuery != "access_token=token-123" {
		t.Fatalf("query = %q, want %q", gotQuery, "access_token=token-123")
	}
}

func TestOAuthHTTPClientUsesTimeoutByDefault(t *testing.T) {
	client := NewOAuthHTTPClient(config.CASRUCConfig{})
	if client.Timeout() <= 0 {
		t.Fatalf("Timeout = %v, want > 0", client.Timeout())
	}
}

func TestParseAccessTokenRejectsErrorAndArbitraryBodies(t *testing.T) {
	if _, err := parseAccessToken([]byte(`{"error":"invalid_grant"}`)); err == nil {
		t.Fatalf("parseAccessToken() error = nil, want error")
	}
	if _, err := parseAccessToken([]byte("not a token")); err == nil {
		t.Fatalf("parseAccessToken() error = nil, want error")
	}
}

func TestCASRUCProviderBuildAuthorizeURL(t *testing.T) {
	provider := providers.NewCASRUCProvider(nil, config.CASRUCConfig{
		ClientID:     "client-id",
		RedirectURL:  "https://example.com/callback",
		AuthorizeURL: "https://cas.ruc.edu.cn/cas/oauth2.0/authorize",
	})

	got, err := provider.BuildAuthorizeURL("oauth-state")
	if err != nil {
		t.Fatalf("BuildAuthorizeURL() error = %v", err)
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
	if query.Get("scope") != "all" {
		t.Fatalf("scope = %q, want all", query.Get("scope"))
	}
	if query.Get("redirect_uri") != "https://example.com/callback" {
		t.Fatalf("redirect_uri = %q, want callback", query.Get("redirect_uri"))
	}
	if query.Get("state") != "oauth-state" {
		t.Fatalf("state = %q, want oauth-state", query.Get("state"))
	}
	if query.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q, want client-id", query.Get("client_id"))
	}
}

func TestCASRUCProviderExchangeCode(t *testing.T) {
	provider := providers.NewCASRUCProvider(&fakeProviderBackend{
		accessToken: "token-123",
	}, config.CASRUCConfig{})

	token, err := provider.ExchangeCode(context.Background(), "code-abc")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if token.AccessToken != "token-123" {
		t.Fatalf("AccessToken = %q, want %q", token.AccessToken, "token-123")
	}
}

func TestCASRUCProviderFetchIdentityMapsStudentID(t *testing.T) {
	provider := providers.NewCASRUCProvider(&fakeProviderBackend{
		profile: providers.OAuthProfile{LoginName: "2026001"},
	}, config.CASRUCConfig{})

	identity, err := provider.FetchIdentity(context.Background(), providers.TokenSet{AccessToken: "token-123"})
	if err != nil {
		t.Fatalf("FetchIdentity() error = %v", err)
	}
	if identity.StudentID != "2026001" {
		t.Fatalf("StudentID = %q, want %q", identity.StudentID, "2026001")
	}
	if identity.Subject != "2026001" {
		t.Fatalf("Subject = %q, want %q", identity.Subject, "2026001")
	}
}

type fakeProviderBackend struct {
	accessToken string
	profile     providers.OAuthProfile
}

func (f *fakeProviderBackend) ExchangeCode(_ context.Context, _ string) (string, error) {
	return f.accessToken, nil
}

func (f *fakeProviderBackend) FetchProfile(_ context.Context, _ string) (providers.OAuthProfile, error) {
	return f.profile, nil
}
