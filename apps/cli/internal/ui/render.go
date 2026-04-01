package ui

import "strings"

func Table(title string, headers []string, rows [][]string) string {
	return CompactTable{
		Theme:   DefaultTheme(),
		Title:   title,
		Headers: headers,
		Rows:    rows,
	}.Render()
}

func Lines(values ...string) string {
	return strings.Join(values, "\n")
}
