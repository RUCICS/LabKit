package ui

import "strings"

func Section(title, body string) string {
	theme := DefaultTheme()
	if strings.TrimSpace(title) == "" {
		return body
	}
	lines := []string{theme.Title(title)}
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		lines = append(lines, trimmed)
	}
	return strings.Join(lines, "\n")
}

func LabelValue(label, value string) string {
	return formatKeyValue(label, value, DefaultTheme())
}

func Success(message string) string {
	return DefaultTheme().Success(message)
}
