package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

type errorEnvelope struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID := requestIDFromRequest(r)
	if requestID == "" {
		requestID = newRequestID()
	}
	if requestID != "" {
		w.Header().Set(requestIDHeader, requestID)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if strings.TrimSpace(code) == "" {
		code = http.StatusText(status)
	}
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: errorPayload{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}

func requestIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if requestID := RequestIDFromContext(r.Context()); requestID != "" {
		return requestID
	}
	return strings.TrimSpace(r.Header.Get(requestIDHeader))
}
