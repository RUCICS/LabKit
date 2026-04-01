package ui

import (
	"strings"
	"testing"
)

func TestResultCardRenderKeepsKeyValuePairsStable(t *testing.T) {
	card := ResultCard{
		Theme: DefaultTheme(),
		Title: "Submission result",
		Fields: []Field{
			{Label: "Submission ID", Value: "11111111-1111-7111-8111-111111111111"},
			{Label: "Status", Value: "queued"},
			{Label: "Elapsed", Value: "12s"},
		},
	}

	out := plain(card.Render())
	for _, want := range []string{"Submission result", "Submission ID", "11111111-1111-7111-8111-111111111111", "Status", "queued", "Elapsed", "12s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() = %q, want %q", out, want)
		}
	}
}
