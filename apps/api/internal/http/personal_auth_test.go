package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestSignedRequestPathIncludesRawQuery(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/api/labs/sorting/board?by=latency", nil)
	if got, want := signedRequestPath(req), "/api/labs/sorting/board?by=latency"; got != want {
		t.Fatalf("signedRequestPath() = %q, want %q", got, want)
	}
}

func TestSignedRequestPathWithoutQuery(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/api/labs/sorting/board", nil)
	if got, want := signedRequestPath(req), "/api/labs/sorting/board"; got != want {
		t.Fatalf("signedRequestPath() = %q, want %q", got, want)
	}
}
