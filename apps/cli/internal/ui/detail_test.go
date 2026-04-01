package ui

import (
	"strings"
	"testing"
)

func TestDetailBlockRenderPreservesMultiLineFailureInformation(t *testing.T) {
	block := DetailBlock{
		Theme: DefaultTheme(),
		Title: "Failure details",
		Lines: []string{
			"compile error:",
			"main.c:12: undefined reference",
			"note: check include path",
		},
	}

	out := plain(block.Render())
	for _, want := range []string{"Failure details", "compile error:", "main.c:12: undefined reference", "note: check include path"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() = %q, want %q", out, want)
		}
	}
	if strings.Count(out, "\n") < 2 {
		t.Fatalf("Render() = %q, want multiple lines", out)
	}
}
