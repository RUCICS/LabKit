package ui

import "strings"

type CompactListItem struct {
	Label  string
	Value  string
	Status string
	Notes  []string
}

type CompactList struct {
	Theme Theme
	Title string
	Items []CompactListItem
}

func (l CompactList) Render() string {
	theme := l.Theme
	if zeroTheme(theme) {
		theme = DefaultTheme()
	}

	lines := make([]string, 0, len(l.Items)*2+1)
	if l.Title != "" {
		lines = append(lines, theme.Title(l.Title))
	}
	for _, item := range l.Items {
		line := "• " + item.Label
		if strings.TrimSpace(item.Status) != "" {
			line += "  " + theme.StatusBadge(item.Status)
		}
		lines = append(lines, line)
		if strings.TrimSpace(item.Value) != "" {
			lines = append(lines, "  "+item.Value)
		}
		for _, note := range item.Notes {
			if note == "" {
				lines = append(lines, "")
				continue
			}
			for _, part := range strings.Split(note, "\n") {
				lines = append(lines, "  "+part)
			}
		}
	}
	return strings.Join(lines, "\n")
}
