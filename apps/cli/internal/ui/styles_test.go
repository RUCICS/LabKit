package ui

import "testing"

func TestSectionWithEmptyTitleReturnsBodyWithoutLeadingBlankLine(t *testing.T) {
	body := "line one\nline two"

	if got, want := Section("", body), body; got != want {
		t.Fatalf("Section() = %q, want %q", got, want)
	}
}
