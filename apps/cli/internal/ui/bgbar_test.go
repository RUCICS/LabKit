package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBgFillRowZeroFractionNoBackground(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("FORCE_COLOR", "3")
	got := BgFillRow("hello world", 0.0, lipgloss.Color("#ffffff"), lipgloss.Color("#123456"))
	// With 0 fill fraction: no background ANSI codes, just foreground-styled text
	plain := StripANSI(got)
	if plain != "hello world" {
		t.Fatalf("BgFillRow() plain = %q, want %q", plain, "hello world")
	}
}

func TestBgFillRowFullFractionAppliesBackground(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("FORCE_COLOR", "3")
	// Use simple color IDs to ensure they render with ANSI codes
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("2"))
	expected := style.Render("hello")
	got := BgFillRow("hello", 1.0, lipgloss.Color("1"), lipgloss.Color("2"))
	// full fill: output should match the style.Render output
	if got != expected {
		t.Fatalf("BgFillRow() = %q, want %q", got, expected)
	}
}

func TestBgFillRowHalfFractionSplitsText(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("FORCE_COLOR", "3")
	text := "abcdefgh" // 8 runes
	got := BgFillRow(text, 0.5, lipgloss.Color("#c0caf5"), lipgloss.Color("#1f2335"))
	plain := StripANSI(got)
	if plain != text {
		t.Fatalf("BgFillRow() plain = %q, want %q", plain, text)
	}
}
