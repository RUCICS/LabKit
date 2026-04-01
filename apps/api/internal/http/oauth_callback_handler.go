package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	authsvc "labkit.local/apps/api/internal/service/auth"
)

type OAuthCallbackHandler struct {
	Service              *authsvc.Service
	BrowserSessionSecure bool
}

func (h *OAuthCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
		return
	}
	result, err := h.Service.HandleOAuthCallback(r.Context(), r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrInvalidState):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}

	sessionToken, err := issueBrowserSession(result.UserID, result.UserKeyID, result.StudentID)
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
	location := "/auth/confirm?student_id=" + result.StudentID + "&user_key_id=" + strconv.FormatInt(result.UserKeyID, 10)
	http.Redirect(w, r, location, http.StatusFound)
}
