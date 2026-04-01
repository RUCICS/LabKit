package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	authsvc "labkit.local/apps/api/internal/service/auth"
)

type DevDeviceHandler struct {
	Service *authsvc.Service
}

type devBindDeviceRequest struct {
	DeviceCode string `json:"device_code"`
	StudentID  string `json:"student_id"`
	DeviceName string `json:"device_name"`
}

func (h *DevDeviceHandler) BindDevice(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		http.Error(w, "service unavailable", http.StatusInternalServerError)
		return
	}

	var req devBindDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.Service.DevBindDeviceAuthorizationRequest(r.Context(), authsvc.DevBindDeviceAuthRequestInput{
		DeviceCode: req.DeviceCode,
		StudentID:  req.StudentID,
		DeviceName: req.DeviceName,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *DevDeviceHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authsvc.ErrInvalidRequest), errors.Is(err, authsvc.ErrInvalidState):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
