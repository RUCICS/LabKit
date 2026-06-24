package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"time"

	"labkit.local/apps/api/internal/http/middleware"
	personal "labkit.local/apps/api/internal/service/personal"
)

type personalAuthenticator interface {
	Authenticate(context.Context, personal.AuthInput) (personal.AuthenticatedUser, error)
}

func authenticatePersonalRequest(w http.ResponseWriter, r *http.Request, auth personalAuthenticator, body []byte) (personal.AuthenticatedUser, bool) {
	if auth == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return personal.AuthenticatedUser{}, false
	}

	keyFingerprint := strings.TrimSpace(r.Header.Get("X-LabKit-Key-Fingerprint"))
	if keyFingerprint == "" {
		middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
		return personal.AuthenticatedUser{}, false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(r.Header.Get("X-LabKit-Timestamp")))
	if err != nil {
		middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
		return personal.AuthenticatedUser{}, false
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(r.Header.Get("X-LabKit-Signature")))
	if err != nil {
		middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
		return personal.AuthenticatedUser{}, false
	}

	result, err := auth.Authenticate(r.Context(), personal.AuthInput{
		KeyFingerprint: keyFingerprint,
		Method:         r.Method,
		Path:           signedRequestPath(r),
		Timestamp:      timestamp,
		Nonce:          strings.TrimSpace(r.Header.Get("X-LabKit-Nonce")),
		Signature:      signature,
		Body:           body,
	})
	if err != nil {
		middleware.WriteError(w, r, http.StatusUnauthorized, "unauthorized", http.StatusText(http.StatusUnauthorized))
		return personal.AuthenticatedUser{}, false
	}
	return result, true
}

func authenticateBrowserSessionOrPersonalRequest(w http.ResponseWriter, r *http.Request, auth personalAuthenticator, body []byte) (personal.AuthenticatedUser, bool) {
	if user, ok := authenticateBrowserSessionRequest(r); ok {
		return user, true
	}
	return authenticatePersonalRequest(w, r, auth, body)
}

// authenticateWritableRequest authenticates a mutating request. Read-only
// web-login sessions (Source=="web", not bound to a device key) are rejected;
// device-paired browser sessions and CLI signatures are accepted. This keeps
// the web login strictly read-only without touching the key-only write paths
// (submit/keys/board), which never accept a browser session at all.
func authenticateWritableRequest(w http.ResponseWriter, r *http.Request, auth personalAuthenticator, body []byte) (personal.AuthenticatedUser, bool) {
	if session, ok := browserSessionFromRequest(r); ok {
		if session.Source == browserSessionSourceWeb {
			middleware.WriteError(w, r, http.StatusForbidden, "read_only_session", "Web login sessions are read-only")
			return personal.AuthenticatedUser{}, false
		}
		return personal.AuthenticatedUser{UserID: session.UserID, KeyID: session.KeyID}, true
	}
	return authenticatePersonalRequest(w, r, auth, body)
}

func authenticateBrowserSessionRequest(r *http.Request) (personal.AuthenticatedUser, bool) {
	session, ok := browserSessionFromRequest(r)
	if !ok {
		return personal.AuthenticatedUser{}, false
	}
	return personal.AuthenticatedUser{UserID: session.UserID, KeyID: session.KeyID}, true
}

func browserSessionFromRequest(r *http.Request) (browserSession, bool) {
	if r == nil {
		return browserSession{}, false
	}
	cookie, err := r.Cookie(browserSessionCookieName)
	if err != nil {
		return browserSession{}, false
	}
	return lookupBrowserSession(strings.TrimSpace(cookie.Value))
}

func readRequestBody(r *http.Request) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// signedRequestPath returns the path string that the CLI includes in the signed payload:
// URL path plus raw query (when present). This must match apps/cli signedRequest(), which
// passes the full request path including "?by=..." for leaderboard views.
func signedRequestPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	path := strings.TrimSpace(r.URL.Path)
	if raw := strings.TrimSpace(r.URL.RawQuery); raw != "" {
		return path + "?" + raw
	}
	return path
}
