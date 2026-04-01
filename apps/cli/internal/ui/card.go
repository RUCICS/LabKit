package ui

import "strings"

type Field struct {
	Label string
	Value string
}

type ResultCard struct {
	Theme  Theme
	Title  string
	Fields []Field
}

func (c ResultCard) Render() string {
	theme := c.Theme
	if zeroTheme(theme) {
		theme = DefaultTheme()
	}

	lines := make([]string, 0, len(c.Fields)+1)
	if c.Title != "" {
		lines = append(lines, theme.Title(c.Title))
	}
	for _, field := range c.Fields {
		lines = append(lines, formatKeyValue(field.Label, field.Value, theme))
	}
	return strings.Join(lines, "\n")
}
