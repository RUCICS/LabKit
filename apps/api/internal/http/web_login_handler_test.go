package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"labkit.local/apps/api/internal/config"
	authsvc "labkit.local/apps/api/internal/service/auth"
	authproviders "labkit.local/apps/api/internal/service/auth/providers"
)

func newWebLoginRouter(repo *routerAuthRepo) *Router {
	casCfg := config.CASRUCConfig{
		ClientID:     "labkit-client",
		AuthorizeURL: "https://sso.example.edu/oauth2/authorize",
		RedirectURL:  "/api/device/verify",
	}
	svc := authsvc.NewServiceWithProvider(repo, authproviders.NewCASRUCProvider(verifyOAuthClient{}, casCfg), config.OAuthConfig{CASRUC: casCfg})
	return NewRouter(WithAuthService(svc))
}

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestWebLoginStartSetsStateCookieAndRedirects(t *testing.T) {
	router := newWebLoginRouter(newRouterAuthRepo())

	req := httptest.NewRequest(http.MethodGet, "/auth/login?next=/grade", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusFound, rr.Body.String())
	}
	location := rr.Header().Get("Location")
	if !strings.HasPrefix(location, "https://sso.example.edu/oauth2/authorize") {
		t.Fatalf("location = %q, want authorize prefix", location)
	}
	locURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	state := locURL.Query().Get("state")
	if state == "" {
		t.Fatal("authorize URL missing state")
	}

	cookies := rr.Result().Cookies()
	stateCookie := cookieByName(cookies, oauthStateCookieName)
	if stateCookie == nil {
		t.Fatal("missing labkit_oauth_state cookie")
	}
	if stateCookie.Value != state {
		t.Fatalf("state cookie = %q, want %q (must match authorize state)", stateCookie.Value, state)
	}
	if !stateCookie.HttpOnly {
		t.Fatal("state cookie must be HttpOnly")
	}
	nextCookie := cookieByName(cookies, oauthNextCookieName)
	if nextCookie == nil || nextCookie.Value != "/grade" {
		t.Fatalf("next cookie = %+v, want value /grade", nextCookie)
	}
}

func TestWebLoginStartRejectsOpenRedirectNext(t *testing.T) {
	router := newWebLoginRouter(newRouterAuthRepo())

	req := httptest.NewRequest(http.MethodGet, "/auth/login?next=https://evil.example/x", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	nextCookie := cookieByName(rr.Result().Cookies(), oauthNextCookieName)
	if nextCookie == nil || nextCookie.Value != defaultWebLoginNext {
		t.Fatalf("next cookie = %+v, want default %q", nextCookie, defaultWebLoginNext)
	}
}

func TestWebLoginCallbackIssuesBrowserSessionAndRedirectsToNext(t *testing.T) {
	repo := newRouterAuthRepo()
	router := newWebLoginRouter(repo)

	// 1) start to obtain a matching state + cookies.
	startReq := httptest.NewRequest(http.MethodGet, "/auth/login?next=/grade", nil)
	startRR := httptest.NewRecorder()
	router.ServeHTTP(startRR, startReq)
	startCookies := startRR.Result().Cookies()
	stateCookie := cookieByName(startCookies, oauthStateCookieName)
	if stateCookie == nil {
		t.Fatal("start did not set state cookie")
	}
	state := stateCookie.Value

	// 2) school redirects back to the shared callback with matching state + cookie.
	callbackReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/device/verify?state=%s&code=auth-code", url.QueryEscape(state)), nil)
	for _, c := range startCookies {
		callbackReq.AddCookie(c)
	}
	callbackRR := httptest.NewRecorder()
	router.ServeHTTP(callbackRR, callbackReq)

	if callbackRR.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want %d body=%s", callbackRR.Code, http.StatusFound, callbackRR.Body.String())
	}
	if got := callbackRR.Header().Get("Location"); got != "/grade" {
		t.Fatalf("callback location = %q, want %q", got, "/grade")
	}
	sessionCookie := cookieByName(callbackRR.Result().Cookies(), browserSessionCookieName)
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("callback did not set browser session cookie")
	}
	// The web user must have been upserted by student_id.
	found := false
	for _, user := range repo.usersByID {
		if user.StudentID == "2026001" {
			found = true
		}
	}
	if !found {
		t.Fatal("web login did not upsert user by student_id")
	}
}

func TestWebLoginCallbackWithoutCookieFallsThroughToDeviceFlow(t *testing.T) {
	repo := newRouterAuthRepo()
	router := newWebLoginRouter(repo)

	// No labkit_oauth_state cookie and no matching device request → device flow
	// rejects the unknown state. Crucially it must NOT mint a web session.
	callbackReq := httptest.NewRequest(http.MethodGet, "/api/device/verify?state=unknown-state&code=auth-code", nil)
	callbackRR := httptest.NewRecorder()
	router.ServeHTTP(callbackRR, callbackReq)

	if callbackRR.Code != http.StatusBadRequest {
		t.Fatalf("callback status = %d, want %d body=%s", callbackRR.Code, http.StatusBadRequest, callbackRR.Body.String())
	}
	if cookie := cookieByName(callbackRR.Result().Cookies(), browserSessionCookieName); cookie != nil {
		t.Fatal("device-flow fallback must not set a browser session cookie")
	}
}
