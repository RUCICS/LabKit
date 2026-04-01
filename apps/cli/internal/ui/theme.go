package ui

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Tokyo Night palette — hex values for true-color terminals.
const (
	colorBlue   = lipgloss.Color("#7aa2f7") // primary — titles, active state, info
	colorGreen  = lipgloss.Color("#9ece6a") // success — passed, completed
	colorYellow = lipgloss.Color("#e0af68") // warning — queued, gold, below-threshold
	colorRed    = lipgloss.Color("#f7768e") // error — failed, error state
	colorMuted  = lipgloss.Color("#565f89") // muted — timestamps, secondary labels
	colorText   = lipgloss.Color("#c0caf5") // body text
	colorPanel  = lipgloss.Color("#1f2335") // used by BgFillRow (Task 3) for leaderboard row highlight
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type Theme struct {
	TitleStyle        lipgloss.Style
	LabelStyle        lipgloss.Style
	ValueStyle        lipgloss.Style
	MutedStyle        lipgloss.Style
	SuccessStyle      lipgloss.Style
	WarningStyle      lipgloss.Style
	ErrorStyle        lipgloss.Style
	InfoStyle         lipgloss.Style
	SuccessBadgeStyle lipgloss.Style
	WarningBadgeStyle lipgloss.Style
	ErrorBadgeStyle   lipgloss.Style
	InfoBadgeStyle    lipgloss.Style
	NeutralBadgeStyle lipgloss.Style
	BadgeBase         lipgloss.Style
	SeparatorStyle    lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		TitleStyle:        lipgloss.NewStyle().Bold(true).Foreground(colorBlue),
		LabelStyle:        lipgloss.NewStyle().Bold(true).Foreground(colorText),
		ValueStyle:        lipgloss.NewStyle().Foreground(colorText),
		MutedStyle:        lipgloss.NewStyle().Foreground(colorMuted),
		SuccessStyle:      lipgloss.NewStyle().Bold(true).Foreground(colorGreen),
		WarningStyle:      lipgloss.NewStyle().Bold(true).Foreground(colorYellow),
		ErrorStyle:        lipgloss.NewStyle().Bold(true).Foreground(colorRed),
		InfoStyle:         lipgloss.NewStyle().Bold(true).Foreground(colorBlue),
		SuccessBadgeStyle: lipgloss.NewStyle().Bold(true).Foreground(colorGreen),
		WarningBadgeStyle: lipgloss.NewStyle().Bold(true).Foreground(colorYellow),
		ErrorBadgeStyle:   lipgloss.NewStyle().Bold(true).Foreground(colorRed),
		InfoBadgeStyle:    lipgloss.NewStyle().Bold(true).Foreground(colorBlue),
		NeutralBadgeStyle: lipgloss.NewStyle().Bold(true).Foreground(colorMuted),
		BadgeBase:         lipgloss.NewStyle().Bold(true),
		SeparatorStyle:    lipgloss.NewStyle().Foreground(colorMuted),
	}
}

// RelativeTime formats t relative to now as a human-readable string ("3h ago", "2d ago").
// Pass now explicitly so callers can inject time in tests.
func RelativeTime(t, now time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// StripANSI removes ANSI escape codes. Exported for use outside ui package.
func StripANSI(s string) string { return stripANSI(s) }

func (t Theme) Title(text string) string   { return t.TitleStyle.Render(text) }
func (t Theme) Label(text string) string   { return t.LabelStyle.Render(text) }
func (t Theme) Value(text string) string   { return t.ValueStyle.Render(text) }
func (t Theme) Muted(text string) string   { return t.MutedStyle.Render(text) }
func (t Theme) Success(text string) string { return t.SuccessStyle.Render(text) }

func (t Theme) StatusBadge(status string) string {
	normalized := strings.TrimSpace(strings.ToLower(status))
	if normalized == "" {
		return ""
	}
	switch normalized {
	case "done", "passed", "approved", "complete", "completed", "success", "scored":
		return t.SuccessBadgeStyle.Render(normalized)
	case "queued", "pending":
		return t.WarningBadgeStyle.Render(normalized)
	case "running", "waiting":
		return t.InfoBadgeStyle.Render(normalized)
	case "failed", "error", "rejected", "cancelled", "canceled":
		return t.ErrorBadgeStyle.Render(normalized)
	default:
		return t.NeutralBadgeStyle.Render(normalized)
	}
}

func (t Theme) CompletionBadge(completed bool) string {
	if completed {
		return t.Success("completed")
	}
	return t.Muted("in progress")
}

func stripANSI(text string) string {
	return ansiPattern.ReplaceAllString(text, "")
}

func textWidth(text string) int {
	return len([]rune(stripANSI(text)))
}

func joinNonEmpty(lines ...string) string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func formatKeyValue(label, value string, theme Theme) string {
	return fmt.Sprintf("%s: %s", theme.Label(label), theme.Value(value))
}

func zeroTheme(theme Theme) bool {
	return reflect.ValueOf(theme).IsZero()
}
