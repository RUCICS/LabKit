package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestRelativeTimeLessThanMinute(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ago := now.Add(-30 * time.Second)
	got := RelativeTime(ago, now)
	if got != "30s ago" {
		t.Fatalf("RelativeTime() = %q, want %q", got, "30s ago")
	}
}

func TestRelativeTimeLessThanHour(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ago := now.Add(-45 * time.Minute)
	got := RelativeTime(ago, now)
	if got != "45m ago" {
		t.Fatalf("RelativeTime() = %q, want %q", got, "45m ago")
	}
}

func TestRelativeTimeLessThanDay(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ago := now.Add(-3 * time.Hour)
	got := RelativeTime(ago, now)
	if got != "3h ago" {
		t.Fatalf("RelativeTime() = %q, want %q", got, "3h ago")
	}
}

func TestRelativeTimeMoreThanDay(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ago := now.Add(-50 * time.Hour)
	got := RelativeTime(ago, now)
	if got != "2d ago" {
		t.Fatalf("RelativeTime() = %q, want %q", got, "2d ago")
	}
}

func TestRelativeTimeFutureTimeClampsToZero(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(30 * time.Second)
	got := RelativeTime(future, now)
	if got != "0s ago" {
		t.Fatalf("RelativeTime(future) = %q, want %q", got, "0s ago")
	}
}

func TestStatusBadgeUsesThemeBadgeStyles(t *testing.T) {
	theme := Theme{
		SuccessStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		WarningStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		ErrorStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		InfoStyle:         lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
		BadgeBase:         lipgloss.NewStyle().Bold(true),
		SuccessBadgeStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("201")).Bold(true),
		WarningBadgeStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("202")).Bold(true),
		ErrorBadgeStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		InfoBadgeStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true),
		NeutralBadgeStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true),
	}

	if got, want := theme.StatusBadge("done"), theme.SuccessBadgeStyle.Render("done"); got != want {
		t.Fatalf("done badge = %q, want %q", got, want)
	}
	if got, want := theme.StatusBadge("queued"), theme.WarningBadgeStyle.Render("queued"); got != want {
		t.Fatalf("queued badge = %q, want %q", got, want)
	}
	if got, want := theme.StatusBadge("failed"), theme.ErrorBadgeStyle.Render("failed"); got != want {
		t.Fatalf("failed badge = %q, want %q", got, want)
	}
	if got, want := theme.StatusBadge("running"), theme.InfoBadgeStyle.Render("running"); got != want {
		t.Fatalf("running badge = %q, want %q", got, want)
	}
}
