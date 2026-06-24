package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	authsvc "labkit.local/apps/api/internal/service/auth"
)

const (
	// oauthStateCookieName carries the double-submit CSRF token for web login.
	oauthStateCookieName = "labkit_oauth_state"
	// oauthNextCookieName remembers the post-login landing path across the
	// round-trip to 微人大 (the school callback can't carry our query params).
	oauthNextCookieName = "labkit_oauth_next"
	oauthStateCookieTTL = 10 * time.Minute

	defaultWebLoginNext = "/"
)

// WebLoginHandler implements the browser-only 微人大 login path. It shares the
// school OAuth callback (/api/device/verify) with the device flow and is
// disambiguated there by the presence of the labkit_oauth_state cookie.
type WebLoginHandler struct {
	Service              *authsvc.Service
	BrowserSessionSecure bool
}

// Start handles GET /auth/login: it mints a CSRF state, stores it (and the
// desired landing path) in short-lived cookies, then redirects to 微人大.
func (h *WebLoginHandler) Start(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
		return
	}
	authorizeURL, state, err := h.Service.BuildWebLoginAuthorizeURL()
	if err != nil {
		if errors.Is(err, authsvc.ErrProviderNotConfigured) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}

	next := sanitizeWebLoginNext(r.URL.Query().Get("next"))
	h.setShortCookie(w, oauthStateCookieName, state)
	h.setShortCookie(w, oauthNextCookieName, next)
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

// handleCallback completes web login from the shared OAuth callback. The caller
// (DeviceVerifyHandler) only routes here when a labkit_oauth_state cookie is
// present; we still re-verify it against the query state (double-submit CSRF).
func (h *WebLoginHandler) handleCallback(w http.ResponseWriter, r *http.Request, state, code string) {
	if h == nil || h.Service == nil {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
		return
	}
	if !hasMatchingOAuthStateCookie(r, state) {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	result, err := h.Service.HandleWebLoginCallback(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrInvalidCode):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, authsvc.ErrProviderNotConfigured):
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		default:
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}

	sessionToken, err := issueWebBrowserSession(result.UserID, result.StudentID)
	if err != nil {
		http.Error(w, "failed to create browser session", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     browserSessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.BrowserSessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(browserSessionTTL.Seconds()),
	})

	next := defaultWebLoginNext
	if cookie, err := r.Cookie(oauthNextCookieName); err == nil {
		next = sanitizeWebLoginNext(cookie.Value)
	}
	h.clearCookie(w, oauthStateCookieName)
	h.clearCookie(w, oauthNextCookieName)
	http.Redirect(w, r, next, http.StatusFound)
}

func (h *WebLoginHandler) setShortCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.BrowserSessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(oauthStateCookieTTL.Seconds()),
	})
}

func (h *WebLoginHandler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.BrowserSessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// hasMatchingOAuthStateCookie reports whether the request carries a
// labkit_oauth_state cookie equal to the supplied (query) state.
func hasMatchingOAuthStateCookie(r *http.Request, state string) bool {
	state = strings.TrimSpace(state)
	if r == nil || state == "" {
		return false
	}
	cookie, err := r.Cookie(oauthStateCookieName)
	if err != nil {
		return false
	}
	got := strings.TrimSpace(cookie.Value)
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(state)) == 1
}

// sanitizeWebLoginNext only allows same-origin absolute paths to prevent open
// redirects; anything else falls back to the default landing page.
func sanitizeWebLoginNext(next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return defaultWebLoginNext
	}
	// Reject protocol-relative ("//host") and absolute ("https://host") URLs.
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return defaultWebLoginNext
	}
	if strings.Contains(next, "\\") {
		return defaultWebLoginNext
	}
	return next
}
