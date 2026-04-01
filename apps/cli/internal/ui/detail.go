package ui

import "strings"

type DetailBlock struct {
	Theme Theme
	Title string
	Lines []string
}

func (d DetailBlock) Render() string {
	theme := d.Theme
	if zeroTheme(theme) {
		theme = DefaultTheme()
	}

	lines := make([]string, 0, len(d.Lines)+1)
	if d.Title != "" {
		lines = append(lines, theme.Title(d.Title))
	}
	for _, line := range d.Lines {
		if line == "" {
			lines = append(lines, "")
			continue
		}
		for _, part := range strings.Split(line, "\n") {
			lines = append(lines, "  "+part)
		}
	}
	return strings.Join(lines, "\n")
}
