package ui

import (
	"strings"
	"testing"
)

func TestResultBlockRendersVerticalAccent(t *testing.T) {
	b := ResultBlock{
		Title: "PASSED   5.0s",
		ID:    "4f3a9b2c",
	}
	got := plain(b.Render())
	for _, want := range []string{"╷", "│", "╵", "PASSED", "4f3a9b2c"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ResultBlock.Render() plain = %q, missing %q", got, want)
		}
	}
}

func TestResultBlockWithDetailsRendersFailureLines(t *testing.T) {
	b := ResultBlock{
		Title:   "FAILED   2.4s",
		ID:      "aabbccdd",
		Details: []string{"test_case_3: expected 42, got 17"},
		Failed:  true,
	}
	got := plain(b.Render())
	for _, want := range []string{"FAILED", "aabbccdd", "test_case_3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ResultBlock.Render() plain = %q, missing %q", got, want)
		}
	}
}

func TestResultBlockEmptyDetailsOmitsDetailLines(t *testing.T) {
	b := ResultBlock{Title: "PASSED", ID: "abc"}
	got := plain(b.Render())
	lines := strings.Split(got, "\n")
	// Should be: ╷ line, title line, id line, ╵ line = 4 lines
	if len(lines) != 4 {
		t.Fatalf("ResultBlock.Render() = %q, want 4 lines, got %d", got, len(lines))
	}
}
