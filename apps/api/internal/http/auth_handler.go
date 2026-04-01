package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	authsvc "labkit.local/apps/api/internal/service/auth"
)

type AuthHandler struct {
	Service *authsvc.Service
}

type createDeviceAuthRequest struct {
	PublicKey string `json:"public_key"`
}

func (h *AuthHandler) CreateDeviceAuthorizationRequest(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
		return
	}
	var req createDeviceAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	result, err := h.Service.CreateDeviceAuthorizationRequest(r.Context(), authsvc.CreateDeviceAuthRequestInput{
		PublicKey: req.PublicKey,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *AuthHandler) PollDeviceAuthorizationRequest(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
		return
	}
	deviceCode := r.URL.Query().Get("device_code")
	result, err := h.Service.PollDeviceAuthorizationRequest(r.Context(), deviceCode)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *AuthHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authsvc.ErrInvalidRequest):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, authsvc.ErrInvalidState):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
