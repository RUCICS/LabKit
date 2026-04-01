package main

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintCLIErrorUsesStructuredFormat(t *testing.T) {
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = orig
	})

	printCLIError(errors.New("boom"))

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	gotBytes, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	got := string(gotBytes)
	for _, want := range []string{"✗", "Error", "boom"} {
		if !strings.Contains(got, want) {
			t.Fatalf("printCLIError() = %q, want %q", got, want)
		}
	}
}
