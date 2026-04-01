package ui

import (
	"regexp"
	"strings"
	"testing"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TestTaskFlowRenderIncludesStageStatusAndCompletion(t *testing.T) {
	flow := TaskFlow{
		Theme: DefaultTheme(),
		Title: "Submit",
		Steps: []TaskStep{
			{Stage: "validate", Status: "running"},
			{Stage: "package", Status: "done"},
			{Stage: "upload", Status: "done", Completed: true},
		},
	}

	out := plain(flow.Render())
	for _, want := range []string{"Submit", "validate", "running", "package", "done", "upload", "completed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() = %q, want %q", out, want)
		}
	}
}

func TestTaskFlowRenderOmitsBadgeForEmptyStatus(t *testing.T) {
	flow := TaskFlow{
		Theme: DefaultTheme(),
		Title: "Submit",
		Steps: []TaskStep{
			{Stage: "validate"},
		},
	}

	out := plain(flow.Render())
	if strings.Contains(out, "UNKNOWN") {
		t.Fatalf("Render() = %q, want no UNKNOWN badge for empty status", out)
	}
	if !strings.Contains(out, "validate") {
		t.Fatalf("Render() = %q, want stage label", out)
	}
}
