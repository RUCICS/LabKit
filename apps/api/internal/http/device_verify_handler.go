package httpapi

import (
	"errors"
	"fmt"
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>LabKit Device Verification</title></head>
<body>
<h1>LabKit Device Verification</h1>
<form method="get" action="/api/device/verify">
  <label for="user_code">User code</label>
  <input id="user_code" name="user_code" autocomplete="off" />
  <button type="submit">Continue</button>
</form>
</body>
</html>`)
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
