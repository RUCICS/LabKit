package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// BgFillRow renders text with a proportional background color fill.
// text must contain no ANSI codes. fillFraction ∈ [0, 1] is the fraction of
// visible characters that receive bgColor as background. fgColor applies to all text.
//
// Used by the leaderboard to show score proportion as a background fill behind each row.
func BgFillRow(text string, fillFraction float64, fgColor, bgColor lipgloss.Color) string {
	if fillFraction < 0 {
		fillFraction = 0
	}
	if fillFraction > 1 {
		fillFraction = 1
	}

	runes := []rune(text)
	totalVW := runewidth.StringWidth(text)
	if totalVW == 0 {
		return text
	}

	fillVW := int(float64(totalVW) * fillFraction)

	// Split runes at the fill boundary (visual width, not byte count).
	splitIdx := 0
	accum := 0
	for i, r := range runes {
		if accum >= fillVW {
			splitIdx = i
			break
		}
		accum += runewidth.RuneWidth(r)
		splitIdx = i + 1
	}

	filled := string(runes[:splitIdx])
	rest := string(runes[splitIdx:])

	filledStyle := lipgloss.NewStyle().Foreground(fgColor).Background(bgColor)
	restStyle := lipgloss.NewStyle().Foreground(fgColor)

	if fillFraction >= 1.0 || rest == "" {
		return filledStyle.Render(text)
	}
	if fillFraction <= 0.0 || filled == "" {
		return restStyle.Render(text)
	}
	return filledStyle.Render(filled) + restStyle.Render(rest)
}
