package ui

import (
	"strings"
	"testing"
)

func TestCompactListRenderOrganizesItemsAsAList(t *testing.T) {
	list := CompactList{
		Theme: DefaultTheme(),
		Title: "Recent submissions",
		Items: []CompactListItem{
			{
				Label:  "11111111-1111-7111-8111-111111111111",
				Status: "queued",
				Value:  "waiting for evaluation",
			},
			{
				Label:  "22222222-2222-7222-8222-222222222222",
				Status: "failed",
				Value:  "compile error",
				Notes: []string{
					"main.c:12: undefined reference",
					"check include path",
				},
			},
		},
	}

	out := plain(list.Render())
	for _, bad := range []string{"┌", "┐", "└", "┘", "│"} {
		if strings.Contains(out, bad) {
			t.Fatalf("Render() = %q, want no heavy border %q", out, bad)
		}
	}
	for _, want := range []string{
		"Recent submissions",
		"11111111-1111-7111-8111-111111111111",
		"queued",
		"waiting for evaluation",
		"22222222-2222-7222-8222-222222222222",
		"failed",
		"compile error",
		"main.c:12: undefined reference",
		"check include path",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() = %q, want %q", out, want)
		}
	}
	if strings.Count(out, "•") != 2 {
		t.Fatalf("Render() = %q, want two list bullets", out)
	}
}
