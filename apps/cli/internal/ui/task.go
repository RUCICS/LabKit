package ui

import (
	"fmt"
	"strings"
)

type TaskStep struct {
	Stage     string
	Status    string
	Completed bool
}

type TaskFlow struct {
	Theme Theme
	Title string
	Steps []TaskStep
}

func (f TaskFlow) Render() string {
	theme := f.Theme
	if zeroTheme(theme) {
		theme = DefaultTheme()
	}

	lines := make([]string, 0, len(f.Steps)+1)
	if f.Title != "" {
		lines = append(lines, theme.Title(f.Title))
	}
	for _, step := range f.Steps {
		stage := step.Stage
		if strings.TrimSpace(stage) == "" {
			stage = "step"
		}
		line := fmt.Sprintf("• %s", theme.Label(stage))
		if badge := theme.StatusBadge(step.Status); badge != "" {
			line += "  " + badge
		}
		if step.Completed {
			line += "  " + theme.CompletionBadge(true)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
