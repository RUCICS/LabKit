package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderQuotaSummaryShowsBonusRemaining(t *testing.T) {
	var buf bytes.Buffer
	q := &quotaSummaryResponse{
		Daily:     3,
		Used:      1,
		Left:      2,
		ResetHint: "00:00 Asia/Shanghai",
		Bonus:     &bonusSummaryResponse{Remaining: 5},
	}
	if err := renderQuotaSummary(&buf, q, ""); err != nil {
		t.Fatalf("renderQuotaSummary() error = %v", err)
	}
	got := stripANSIForTest(buf.String())
	if !strings.Contains(got, "2 left today") {
		t.Fatalf("output missing daily remaining: %q", got)
	}
	if !strings.Contains(got, "5 bonus left") {
		t.Fatalf("output missing bonus tail: %q", got)
	}
	if strings.Contains(got, "next submission will spend") {
		t.Fatalf("hint should not appear while daily quota remains: %q", got)
	}
}

func TestRenderQuotaSummaryHintsBonusWillBeSpentWhenDailyExhausted(t *testing.T) {
	var buf bytes.Buffer
	q := &quotaSummaryResponse{
		Daily:     3,
		Used:      3,
		Left:      0,
		ResetHint: "00:00 Asia/Shanghai",
		Bonus:     &bonusSummaryResponse{Remaining: 5},
	}
	if err := renderQuotaSummary(&buf, q, ""); err != nil {
		t.Fatalf("renderQuotaSummary() error = %v", err)
	}
	got := stripANSIForTest(buf.String())
	if !strings.Contains(got, "0 left today") {
		t.Fatalf("missing exhausted daily line: %q", got)
	}
	if !strings.Contains(got, "next submission will spend 1 bonus credit") {
		t.Fatalf("missing bonus-usage hint: %q", got)
	}
	if !strings.Contains(got, "(5 remaining)") {
		t.Fatalf("hint should include remaining count: %q", got)
	}
}

func TestRenderQuotaSummaryNoHintWhenNoBonus(t *testing.T) {
	var buf bytes.Buffer
	q := &quotaSummaryResponse{
		Daily:     3,
		Used:      3,
		Left:      0,
		ResetHint: "00:00 Asia/Shanghai",
	}
	if err := renderQuotaSummary(&buf, q, ""); err != nil {
		t.Fatalf("renderQuotaSummary() error = %v", err)
	}
	got := stripANSIForTest(buf.String())
	if strings.Contains(got, "bonus") {
		t.Fatalf("output should not mention bonus when none available: %q", got)
	}
	if strings.Contains(got, "next submission will spend") {
		t.Fatalf("output should not show bonus hint when none available: %q", got)
	}
}

func TestRenderQuotaSummaryNoHintWhenFreeVerdictPresent(t *testing.T) {
	var buf bytes.Buffer
	q := &quotaSummaryResponse{
		Daily:     3,
		Used:      3,
		Left:      0,
		ResetHint: "00:00 Asia/Shanghai",
		Bonus:     &bonusSummaryResponse{Remaining: 5},
	}
	// freeVerdict is set when the most recent submission resolved to a free
	// verdict — the existing UX is to print "X is free" instead of the
	// resets-at hint. The bonus hint would conflict with that messaging, so
	// it must be suppressed in this case.
	if err := renderQuotaSummary(&buf, q, "build_failed"); err != nil {
		t.Fatalf("renderQuotaSummary() error = %v", err)
	}
	got := stripANSIForTest(buf.String())
	if !strings.Contains(got, "build_failed is free") {
		t.Fatalf("missing free-verdict detail: %q", got)
	}
	if strings.Contains(got, "next submission will spend") {
		t.Fatalf("bonus hint should be suppressed when free verdict already explains the situation: %q", got)
	}
}
