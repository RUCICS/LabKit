package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"labkit.local/apps/api/internal/config"
)

func TestNewConstructsCASRUCProvider(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/oauth/token"; got != want {
			t.Fatalf("token path = %q, want %q", got, want)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got, want := r.Form.Get("client_id"), "client-id"; got != want {
			t.Fatalf("client_id = %q, want %q", got, want)
		}
		if got, want := r.Form.Get("redirect_uri"), "https://example.com/callback"; got != want {
			t.Fatalf("redirect_uri = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"access_token":"token-123"}`))
	}))
	defer tokenSrv.Close()

	profileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/oauth/profile"; got != want {
			t.Fatalf("profile path = %q, want %q", got, want)
		}
		if got := r.URL.Query().Get("access_token"); got != "token-123" {
			t.Fatalf("access_token = %q, want %q", got, "token-123")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"loginName": "2026001"})
	}))
	defer profileSrv.Close()

	provider, err := New(config.OAuthConfig{
		Provider: "cas_ruc",
		CASRUC: config.CASRUCConfig{
			ClientID:     "client-id",
			RedirectURL:  "https://example.com/callback",
			AuthorizeURL: "https://cas.ruc.edu.cn/cas/oauth2.0/authorize",
			TokenURL:     tokenSrv.URL + "/oauth/token",
			ProfileURL:   profileSrv.URL + "/oauth/profile",
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if provider.Name() != "cas_ruc" {
		t.Fatalf("Name() = %q, want %q", provider.Name(), "cas_ruc")
	}

	gotURL, err := provider.BuildAuthorizeURL("oauth-state")
	if err != nil {
		t.Fatalf("BuildAuthorizeURL() error = %v", err)
	}
	parsed, err := url.Parse(gotURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if got := parsed.Query().Get("client_id"); got != "client-id" {
		t.Fatalf("authorize client_id = %q, want %q", got, "client-id")
	}
	if got := parsed.Query().Get("state"); got != "oauth-state" {
		t.Fatalf("authorize state = %q, want %q", got, "oauth-state")
	}

	token, err := provider.ExchangeCode(context.Background(), "code-abc")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if token.AccessToken != "token-123" {
		t.Fatalf("AccessToken = %q, want %q", token.AccessToken, "token-123")
	}

	identity, err := provider.FetchIdentity(context.Background(), token)
	if err != nil {
		t.Fatalf("FetchIdentity() error = %v", err)
	}
	if identity.StudentID != "2026001" {
		t.Fatalf("StudentID = %q, want %q", identity.StudentID, "2026001")
	}
}

func TestNewRejectsUnsupportedProvider(t *testing.T) {
	_, err := New(config.OAuthConfig{Provider: "unknown"})
	if err == nil {
		t.Fatal("New() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported auth provider") {
		t.Fatalf("error = %q, want unsupported provider error", err)
	}
}
