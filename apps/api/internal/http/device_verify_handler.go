package httpapi

import (
	"errors"
	"net/http"
	"strings"

	authsvc "labkit.local/apps/api/internal/service/auth"
)

type DeviceVerifyHandler struct {
	Service              *authsvc.Service
	BrowserSessionSecure bool
}

func (h *DeviceVerifyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
		return
	}

	query := r.URL.Query()
	if strings.TrimSpace(query.Get("code")) != "" || strings.TrimSpace(query.Get("state")) != "" {
		callback := &OAuthCallbackHandler{Service: h.Service, BrowserSessionSecure: h.BrowserSessionSecure}
		callback.ServeHTTP(w, r)
		return
	}

	if userCode := strings.TrimSpace(query.Get("user_code")); userCode != "" {
		location, err := h.Service.BeginDeviceVerification(r.Context(), userCode)
		if err != nil {
			h.writeError(w, r, err)
			return
		}
		http.Redirect(w, r, location, http.StatusFound)
		return
	}

	http.Error(w, "missing user_code or oauth callback parameters", http.StatusBadRequest)
}

func (h *DeviceVerifyHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authsvc.ErrInvalidRequest):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, authsvc.ErrInvalidState):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
