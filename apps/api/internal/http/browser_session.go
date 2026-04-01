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
)

var browserSessions sync.Map

type browserSession struct {
	UserID    int64
	KeyID     int64
	StudentID string
	ExpiresAt time.Time
}

func issueBrowserSession(userID, keyID int64, studentID string) (string, error) {
	token, err := randomBrowserSessionToken()
	if err != nil {
		return "", err
	}
	browserSessions.Store(token, browserSession{
		UserID:    userID,
		KeyID:     keyID,
		StudentID: studentID,
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
