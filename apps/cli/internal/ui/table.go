package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

type CompactTable struct {
	Theme   Theme
	Title   string
	Headers []string
	Rows    [][]string
}

func (t CompactTable) Render() string {
	theme := t.Theme
	if zeroTheme(theme) {
		theme = DefaultTheme()
	}

	cols := len(t.Headers)
	for _, row := range t.Rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		if t.Title == "" {
			return ""
		}
		return theme.Title(t.Title)
	}

	widths := make([]int, cols)
	for i := 0; i < cols; i++ {
		if i < len(t.Headers) {
			widths[i] = max(widths[i], runewidth.StringWidth(stripANSI(t.Headers[i])))
		}
		for _, row := range t.Rows {
			if i < len(row) {
				widths[i] = max(widths[i], runewidth.StringWidth(stripANSI(row[i])))
			}
		}
	}

	lines := make([]string, 0, len(t.Rows)+2)
	if t.Title != "" {
		lines = append(lines, theme.Title(t.Title))
	}
	if len(t.Headers) > 0 {
		lines = append(lines, renderCompactRow(t.Headers, widths, theme, true))
	}
	for _, row := range t.Rows {
		lines = append(lines, renderCompactRow(row, widths, theme, false))
	}
	return strings.Join(lines, "\n")
}

func renderCompactRow(cols []string, widths []int, theme Theme, header bool) string {
	rendered := make([]string, len(widths))
	for i := range widths {
		value := ""
		if i < len(cols) {
			value = cols[i]
		}
		if header {
			value = theme.Label(value)
		} else {
			value = theme.Value(value)
		}
		rendered[i] = padRight(value, widths[i])
	}
	return strings.Join(rendered, "  ")
}

func padRight(text string, width int) string {
	current := runewidth.StringWidth(stripANSI(text))
	if current >= width {
		return text
	}
	return text + strings.Repeat(" ", width-current)
}

// PadRight pads text to the given visible width. Exported for use in commands.
func PadRight(text string, width int) string { return padRight(text, width) }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
