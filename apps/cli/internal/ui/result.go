package ui

import "strings"

// ResultBlock renders a ╷ … ╵ vertical accent card used for submission outcomes.
// Title is shown bold in success or error color. ID is shown muted below it.
// Details are additional lines (e.g. failure messages) inside the block.
type ResultBlock struct {
	Theme   Theme
	Title   string   // e.g. "PASSED   5.0s"
	ID      string   // submission ID, shown muted
	Details []string // optional failure detail lines
	Failed  bool     // if true, accent uses error color instead of success
}

func (b ResultBlock) Render() string {
	theme := b.Theme
	if zeroTheme(theme) {
		theme = DefaultTheme()
	}

	accentStyle := theme.SuccessStyle
	titleStyle := theme.SuccessStyle
	if b.Failed {
		accentStyle = theme.ErrorStyle
		titleStyle = theme.ErrorStyle
	}

	top := accentStyle.Render("╷")
	bar := accentStyle.Render("│")
	bot := accentStyle.Render("╵")

	lines := []string{
		"  " + top,
		"  " + bar + "  " + titleStyle.Render(b.Title),
	}
	if strings.TrimSpace(b.ID) != "" {
		lines = append(lines, "  "+bar+"  "+theme.MutedStyle.Render(b.ID))
	}
	for _, d := range b.Details {
		if strings.TrimSpace(d) == "" {
			lines = append(lines, "  "+bar)
		} else {
			lines = append(lines, "  "+bar+"  "+d)
		}
	}
	lines = append(lines, "  "+bot)
	return strings.Join(lines, "\n")
}
