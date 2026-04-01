package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"labkit.local/apps/api/internal/config"
)

func TestSchoolDevcenterBuildAuthorizeURL(t *testing.T) {
	provider := NewSchoolDevcenterProvider(config.SchoolDevcenterConfig{
		ClientID:     "client-id",
		RedirectURL:  "https://example.com/callback",
		AuthorizeURL: "https://school.example.edu/oauth2/authorize",
		TokenURL:     "https://school.example.edu/oauth2/token",
		UserURL:      "https://school.example.edu/oauth2/user",
		ProfileURL:   "https://school.example.edu/oauth2/profile",
		Scope:        "openid profile",
	})

	got, err := provider.BuildAuthorizeURL("oauth-state")
	if err != nil {
		t.Fatalf("BuildAuthorizeURL() error = %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if got, want := parsed.Scheme, "https"; got != want {
		t.Fatalf("authorize scheme = %q, want %q", got, want)
	}
	if got, want := parsed.Host, "school.example.edu"; got != want {
		t.Fatalf("authorize host = %q, want %q", got, want)
	}
	if got, want := parsed.Path, "/oauth2/authorize"; got != want {
		t.Fatalf("authorize path = %q, want %q", got, want)
	}
	query := parsed.Query()
	if got, want := query.Get("response_type"), "code"; got != want {
		t.Fatalf("response_type = %q, want %q", got, want)
	}
	if got, want := query.Get("client_id"), "client-id"; got != want {
		t.Fatalf("client_id = %q, want %q", got, want)
	}
	if got, want := query.Get("redirect_uri"), "https://example.com/callback"; got != want {
		t.Fatalf("redirect_uri = %q, want %q", got, want)
	}
	if got, want := query.Get("scope"), "openid profile"; got != want {
		t.Fatalf("scope = %q, want %q", got, want)
	}
	if got, want := query.Get("state"), "oauth-state"; got != want {
		t.Fatalf("state = %q, want %q", got, want)
	}
}

func TestSchoolDevcenterExchangeCodeUsesPOSTFormAndBasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPost; got != want {
			t.Fatalf("method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/oauth2/token"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"; got != want {
			t.Fatalf("content-type = %q, want %q", got, want)
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("client-id:client-secret"))
		if got := r.Header.Get("Authorization"); got != wantAuth {
			t.Fatalf("authorization = %q, want %q", got, wantAuth)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got, want := r.Form.Get("code"), "code-abc"; got != want {
			t.Fatalf("code = %q, want %q", got, want)
		}
		if got, want := r.Form.Get("grant_type"), "authorization_code"; got != want {
			t.Fatalf("grant_type = %q, want %q", got, want)
		}
		if got, want := r.Form.Get("redirect_uri"), "https://example.com/callback"; got != want {
			t.Fatalf("redirect_uri = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"access_token":"token-123","token_type":"Bearer","scope":"profile","expires_in":3600}`))
	}))
	defer srv.Close()

	provider := NewSchoolDevcenterProvider(config.SchoolDevcenterConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
		AuthorizeURL: srv.URL + "/oauth2/authorize",
		TokenURL:     srv.URL + "/oauth2/token",
		UserURL:      srv.URL + "/oauth2/user",
		ProfileURL:   srv.URL + "/oauth2/profile",
	})

	token, err := provider.ExchangeCode(context.Background(), "code-abc")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if got, want := token.AccessToken, "token-123"; got != want {
		t.Fatalf("AccessToken = %q, want %q", got, want)
	}
	if got, want := token.TokenType, "Bearer"; got != want {
		t.Fatalf("TokenType = %q, want %q", got, want)
	}
	if got, want := token.Scope, "profile"; got != want {
		t.Fatalf("Scope = %q, want %q", got, want)
	}
}

func TestSchoolDevcenterFetchIdentityUsesBearerProfileAndExtractsStudentID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/oauth2/v1/profile":
			if got, want := r.Header.Get("Authorization"), "Bearer token-123"; got != want {
				t.Fatalf("authorization = %q, want %q", got, want)
			}
			if got := r.URL.Query().Get("access_token"); got != "" {
				t.Fatalf("access_token query = %q, want empty", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"uid":   "1085379",
				"name":  "全商佑",
				"email": "student@example.edu",
				"profiles": []map[string]any{
					{
						"stno":      "20150119",
						"isprimary": true,
					},
				},
			})
		case "/apis/oauth2/v1/user":
			if got, want := r.Header.Get("Authorization"), "Bearer token-123"; got != want {
				t.Fatalf("authorization = %q, want %q", got, want)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name":     "全商佑",
				"username": "590a8272aeb84410e6709ce7",
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := NewSchoolDevcenterProvider(config.SchoolDevcenterConfig{
		AuthorizeURL: srv.URL + "/oauth2/authorize",
		TokenURL:     srv.URL + "/oauth2/token",
		UserURL:      srv.URL + "/apis/oauth2/v1/user",
		ProfileURL:   srv.URL + "/apis/oauth2/v1/profile",
	})

	identity, err := provider.FetchIdentity(context.Background(), TokenSet{AccessToken: "token-123", TokenType: "Bearer"})
	if err != nil {
		t.Fatalf("FetchIdentity() error = %v", err)
	}
	if got, want := identity.Provider, "school_devcenter"; got != want {
		t.Fatalf("Provider = %q, want %q", got, want)
	}
	if got, want := identity.Subject, "1085379"; got != want {
		t.Fatalf("Subject = %q, want %q", got, want)
	}
	if got, want := identity.StudentID, "20150119"; got != want {
		t.Fatalf("StudentID = %q, want %q", got, want)
	}
	if got, want := identity.Name, "全商佑"; got != want {
		t.Fatalf("Name = %q, want %q", got, want)
	}
	if got, want := identity.Email, "student@example.edu"; got != want {
		t.Fatalf("Email = %q, want %q", got, want)
	}
}

func TestSchoolDevcenterFetchIdentityFallsBackToUserInfoForSubjectAndName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/oauth2/v1/profile":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"profiles": []map[string]any{
					{
						"stno": "20150119",
					},
				},
			})
		case "/apis/oauth2/v1/user":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name":     "张三",
				"username": "590a8272aeb84410e6709ce7",
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	provider := NewSchoolDevcenterProvider(config.SchoolDevcenterConfig{
		UserURL:    srv.URL + "/apis/oauth2/v1/user",
		ProfileURL: srv.URL + "/apis/oauth2/v1/profile",
	})

	identity, err := provider.FetchIdentity(context.Background(), TokenSet{AccessToken: "token-123"})
	if err != nil {
		t.Fatalf("FetchIdentity() error = %v", err)
	}
	if got, want := identity.Subject, "590a8272aeb84410e6709ce7"; got != want {
		t.Fatalf("Subject = %q, want %q", got, want)
	}
	if got, want := identity.Name, "张三"; got != want {
		t.Fatalf("Name = %q, want %q", got, want)
	}
}
