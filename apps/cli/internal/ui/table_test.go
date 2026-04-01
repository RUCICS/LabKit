package ui

import (
	"strings"
	"testing"
)

func TestCompactTableRenderOrganizesFieldsWithoutHeavyBorders(t *testing.T) {
	table := CompactTable{
		Theme:   DefaultTheme(),
		Title:   "History",
		Headers: []string{"ID", "STATUS", "SCORE"},
		Rows: [][]string{
			{"a1", "queued", "-"},
			{"b2", "scored", "99"},
		},
	}

	out := plain(table.Render())
	for _, bad := range []string{"┌", "┐", "└", "┘", "│"} {
		if strings.Contains(out, bad) {
			t.Fatalf("Render() = %q, want no heavy border %q", out, bad)
		}
	}
	for _, want := range []string{"History", "ID", "STATUS", "SCORE", "a1", "queued", "b2", "scored", "99"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() = %q, want %q", out, want)
		}
	}
}
