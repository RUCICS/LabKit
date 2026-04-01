package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"labkit.local/apps/api/internal/config"
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

	client := NewOAuthHTTPClient(config.OAuthConfig{
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

	client := NewOAuthHTTPClient(config.OAuthConfig{
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
	client := NewOAuthHTTPClient(config.OAuthConfig{})
	if client.httpClient.Timeout <= 0 {
		t.Fatalf("Timeout = %v, want > 0", client.httpClient.Timeout)
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
