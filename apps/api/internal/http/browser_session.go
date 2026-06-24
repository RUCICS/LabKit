package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const (
	browserSessionCookieName = "labkit_browser_session"
	browserSessionTTL        = 8 * time.Hour

	// browserSessionSource* records how a browser session was minted, for
	// auditing and for any future "web sessions are read-only" enforcement.
	browserSessionSourceDevice = "device"
	browserSessionSourceWeb    = "web"
)

var browserSessions sync.Map

type browserSession struct {
	UserID    int64
	KeyID     int64
	StudentID string
	Source    string
	ExpiresAt time.Time
}

// issueBrowserSession mints a session for the CLI device flow (key-bound).
func issueBrowserSession(userID, keyID int64, studentID string) (string, error) {
	return issueBrowserSessionWithSource(userID, keyID, studentID, browserSessionSourceDevice)
}

// issueWebBrowserSession mints a session for the 微人大 web login. It is not
// bound to a device key (KeyID == 0); read handlers rely on UserID/StudentID.
func issueWebBrowserSession(userID int64, studentID string) (string, error) {
	return issueBrowserSessionWithSource(userID, 0, studentID, browserSessionSourceWeb)
}

func issueBrowserSessionWithSource(userID, keyID int64, studentID, source string) (string, error) {
	token, err := randomBrowserSessionToken()
	if err != nil {
		return "", err
	}
	browserSessions.Store(token, browserSession{
		UserID:    userID,
		KeyID:     keyID,
		StudentID: studentID,
		Source:    source,
		ExpiresAt: time.Now().UTC().Add(browserSessionTTL),
	})
	return token, nil
}

func lookupBrowserSession(token string) (browserSession, bool) {
	value, ok := browserSessions.Load(token)
	if !ok {
		return browserSession{}, false
	}
	session, ok := value.(browserSession)
	if !ok {
		browserSessions.Delete(token)
		return browserSession{}, false
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		browserSessions.Delete(token)
		return browserSession{}, false
	}
	return session, true
}

func randomBrowserSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if token == "" {
		return "", errors.New("failed to generate browser session token")
	}
	return token, nil
}
