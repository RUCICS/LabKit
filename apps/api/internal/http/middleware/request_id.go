package middleware

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const requestIDHeader = "X-Request-Id"

type requestIDContextKey struct{}

var requestIDFallbackCounter uint64

func RequestIDMiddleware(next http.Handler) http.Handler {
	if next == nil {
		panic("request id middleware requires a next handler")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set(requestIDHeader, requestID)
		ctx := ContextWithRequestID(r.Context(), requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIDContextKey{}, strings.TrimSpace(requestID))
}

func newRequestID() string {
	var buf [16]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}

	binary.BigEndian.PutUint64(buf[:8], uint64(time.Now().UnixNano()))
	binary.BigEndian.PutUint64(buf[8:], atomic.AddUint64(&requestIDFallbackCounter, 1))

	return hex.EncodeToString(buf[:])
}
