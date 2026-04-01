# CLI UX Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle all CLI output to the Tokyo Night-palette, Cargo-inspired design from the spec — structured hierarchy, colored status, live 4-line progress, BgFill leaderboard, relative timestamps, consistent error format.

**Architecture:** `ui/` package gains three new primitives (`RelativeTime`, `ResultBlock`, `BgFillRow`). Command render functions in `lab_commands.go`, `auth.go`, and `keys.go` are rewritten to use the new primitives and Tokyo Night colors. `submit_live.go` is rewritten to a 4-line in-place region renderer. No new dependencies.

**Tech Stack:** Go, `charmbracelet/lipgloss` v1.1.0, `golang.org/x/term`, `mattn/go-runewidth`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `apps/cli/internal/ui/theme.go` | Modify | Tokyo Night colors, add `RelativeTime()`, `StatusColor()` helpers |
| `apps/cli/internal/ui/result.go` | **Create** | `ResultBlock` — `╷ … ╵` vertical accent card for submission results |
| `apps/cli/internal/ui/bgbar.go` | **Create** | `BgFillRow` — proportional background-fill row for leaderboard |
| `apps/cli/internal/commands/submit_live.go` | Modify | 4-line multi-line in-place renderer with progress bar + stage indicators |
| `apps/cli/internal/commands/lab_commands.go` | Modify | All render functions: submit, board, history, nick, track |
| `apps/cli/internal/commands/auth.go` | Modify | Auth flow output (title format, spinner while polling, completion card) |
| `apps/cli/internal/commands/keys.go` | Modify | Keys list (separator, relative time), revoke confirmation |
| `apps/cli/cmd/labkit/main.go` | Modify | Structured `✗ Error  …` format for command errors |

---

## Task 1: Theme — Tokyo Night palette + `RelativeTime`

**Files:**
- Modify: `apps/cli/internal/ui/theme.go`
- Modify: `apps/cli/internal/ui/theme_test.go`

- [ ] **Step 1: Write failing tests for new helpers**

Add to `apps/cli/internal/ui/theme_test.go`:

```go
func TestRelativeTimeLessThanMinute(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ago := now.Add(-30 * time.Second)
	got := RelativeTime(ago, now)
	if got != "30s ago" {
		t.Fatalf("RelativeTime() = %q, want %q", got, "30s ago")
	}
}

func TestRelativeTimeLessThanHour(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ago := now.Add(-45 * time.Minute)
	got := RelativeTime(ago, now)
	if got != "45m ago" {
		t.Fatalf("RelativeTime() = %q, want %q", got, "45m ago")
	}
}

func TestRelativeTimeLessThanDay(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ago := now.Add(-3 * time.Hour)
	got := RelativeTime(ago, now)
	if got != "3h ago" {
		t.Fatalf("RelativeTime() = %q, want %q", got, "3h ago")
	}
}

func TestRelativeTimeMoreThanDay(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ago := now.Add(-50 * time.Hour)
	got := RelativeTime(ago, now)
	if got != "2d ago" {
		t.Fatalf("RelativeTime() = %q, want %q", got, "2d ago")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/ui/ -run TestRelativeTime -v
```

Expected: compile error — `RelativeTime undefined`

- [ ] **Step 3: Replace `theme.go` with Tokyo Night palette + new helpers**

Replace the entire `DefaultTheme()` function and add helpers. The existing `Theme` struct fields stay the same; only the color values change.

```go
package ui

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Tokyo Night palette — hex values for true-color terminals.
const (
	colorBlue   = lipgloss.Color("#7aa2f7") // primary — titles, active state, info
	colorGreen  = lipgloss.Color("#9ece6a") // success — passed, completed
	colorYellow = lipgloss.Color("#e0af68") // warning — queued, gold, below-threshold
	colorRed    = lipgloss.Color("#f7768e") // error — failed, error state
	colorMuted  = lipgloss.Color("#565f89") // muted — timestamps, secondary labels
	colorText   = lipgloss.Color("#c0caf5") // body text
	colorPanel  = lipgloss.Color("#1f2335") // subtle background for highlighted rows
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type Theme struct {
	TitleStyle        lipgloss.Style
	LabelStyle        lipgloss.Style
	ValueStyle        lipgloss.Style
	MutedStyle        lipgloss.Style
	SuccessStyle      lipgloss.Style
	WarningStyle      lipgloss.Style
	ErrorStyle        lipgloss.Style
	InfoStyle         lipgloss.Style
	SuccessBadgeStyle lipgloss.Style
	WarningBadgeStyle lipgloss.Style
	ErrorBadgeStyle   lipgloss.Style
	InfoBadgeStyle    lipgloss.Style
	NeutralBadgeStyle lipgloss.Style
	BadgeBase         lipgloss.Style
	SeparatorStyle    lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		TitleStyle:        lipgloss.NewStyle().Bold(true).Foreground(colorBlue),
		LabelStyle:        lipgloss.NewStyle().Bold(true).Foreground(colorText),
		ValueStyle:        lipgloss.NewStyle().Foreground(colorText),
		MutedStyle:        lipgloss.NewStyle().Foreground(colorMuted),
		SuccessStyle:      lipgloss.NewStyle().Bold(true).Foreground(colorGreen),
		WarningStyle:      lipgloss.NewStyle().Bold(true).Foreground(colorYellow),
		ErrorStyle:        lipgloss.NewStyle().Bold(true).Foreground(colorRed),
		InfoStyle:         lipgloss.NewStyle().Bold(true).Foreground(colorBlue),
		SuccessBadgeStyle: lipgloss.NewStyle().Bold(true).Foreground(colorGreen),
		WarningBadgeStyle: lipgloss.NewStyle().Bold(true).Foreground(colorYellow),
		ErrorBadgeStyle:   lipgloss.NewStyle().Bold(true).Foreground(colorRed),
		InfoBadgeStyle:    lipgloss.NewStyle().Bold(true).Foreground(colorBlue),
		NeutralBadgeStyle: lipgloss.NewStyle().Bold(true).Foreground(colorMuted),
		BadgeBase:         lipgloss.NewStyle().Bold(true),
		SeparatorStyle:    lipgloss.NewStyle().Foreground(colorMuted),
	}
}

// RelativeTime formats t relative to now as a human-readable string ("3h ago", "2d ago").
// Pass now explicitly so callers can inject time in tests.
func RelativeTime(t, now time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func (t Theme) Title(text string) string   { return t.TitleStyle.Render(text) }
func (t Theme) Label(text string) string   { return t.LabelStyle.Render(text) }
func (t Theme) Value(text string) string   { return t.ValueStyle.Render(text) }
func (t Theme) Muted(text string) string   { return t.MutedStyle.Render(text) }
func (t Theme) Success(text string) string { return t.SuccessStyle.Render(text) }

func (t Theme) StatusBadge(status string) string {
	normalized := strings.TrimSpace(strings.ToLower(status))
	if normalized == "" {
		return ""
	}
	switch normalized {
	case "done", "passed", "approved", "complete", "completed", "success", "scored":
		return t.SuccessBadgeStyle.Render(normalized)
	case "queued", "pending":
		return t.WarningBadgeStyle.Render(normalized)
	case "running", "waiting":
		return t.InfoBadgeStyle.Render(normalized)
	case "failed", "error", "rejected", "cancelled", "canceled":
		return t.ErrorBadgeStyle.Render(normalized)
	default:
		return t.NeutralBadgeStyle.Render(normalized)
	}
}

func (t Theme) CompletionBadge(completed bool) string {
	if completed {
		return t.Success("completed")
	}
	return t.Muted("in progress")
}

func stripANSI(text string) string {
	return ansiPattern.ReplaceAllString(text, "")
}

func textWidth(text string) int {
	return len([]rune(stripANSI(text)))
}

func joinNonEmpty(lines ...string) string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func formatKeyValue(label, value string, theme Theme) string {
	return fmt.Sprintf("%s: %s", theme.Label(label), theme.Value(value))
}

func zeroTheme(theme Theme) bool {
	return reflect.ValueOf(theme).IsZero()
}
```

- [ ] **Step 4: Update `theme_test.go` to use new color constants**

The `TestStatusBadgeUsesThemeBadgeStyles` test constructs its own `Theme` with explicit colors and doesn't depend on `DefaultTheme()` colors, so it passes as-is. Just add `"time"` to the imports.

Replace the import block in `theme_test.go`:

```go
import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)
```

- [ ] **Step 5: Run all ui tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/ui/ -v
```

Expected: all pass, including the 4 new `TestRelativeTime*` tests.

- [ ] **Step 6: Commit**

```bash
git add apps/cli/internal/ui/theme.go apps/cli/internal/ui/theme_test.go
git commit -m "feat(cli/ui): Tokyo Night palette + RelativeTime helper"
```

---

## Task 2: `ResultBlock` primitive

**Files:**
- Create: `apps/cli/internal/ui/result.go`
- Create: `apps/cli/internal/ui/result_test.go`

- [ ] **Step 1: Write failing tests**

Create `apps/cli/internal/ui/result_test.go`:

```go
package ui

import (
	"strings"
	"testing"
)

func TestResultBlockRendersVerticalAccent(t *testing.T) {
	b := ResultBlock{
		Title: "PASSED   5.0s",
		ID:    "4f3a9b2c",
	}
	got := b.Render()
	for _, want := range []string{"╷", "│", "╵", "PASSED", "4f3a9b2c"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ResultBlock.Render() = %q, missing %q", got, want)
		}
	}
}

func TestResultBlockWithDetailsRendersFailureLines(t *testing.T) {
	b := ResultBlock{
		Title:   "FAILED   2.4s",
		ID:      "aabbccdd",
		Details: []string{"test_case_3: expected 42, got 17"},
		Failed:  true,
	}
	got := b.Render()
	for _, want := range []string{"FAILED", "aabbccdd", "test_case_3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ResultBlock.Render() = %q, missing %q", got, want)
		}
	}
}

func TestResultBlockEmptyDetailsOmitsDetailLines(t *testing.T) {
	b := ResultBlock{Title: "PASSED", ID: "abc"}
	got := b.Render()
	lines := strings.Split(got, "\n")
	// Should be: ╷, title line, id line, ╵ = 4 lines
	if len(lines) != 4 {
		t.Fatalf("ResultBlock.Render() = %q, want 4 lines, got %d", got, len(lines))
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/ui/ -run TestResultBlock -v
```

Expected: compile error — `ResultBlock undefined`

- [ ] **Step 3: Implement `result.go`**

Create `apps/cli/internal/ui/result.go`:

```go
package ui

import "strings"

// ResultBlock renders a ╷ … ╵ vertical accent card used for submission outcomes.
// Title is shown bold in success or error color. ID is shown muted below it.
// Details are additional lines (e.g. failure messages) inside the block.
type ResultBlock struct {
	Theme   Theme
	Title   string   // e.g. "PASSED   5.0s"
	ID      string   // submission ID, shown muted
	Details []string // optional failure detail lines
	Failed  bool     // if true, accent uses error color instead of success
}

func (b ResultBlock) Render() string {
	theme := b.Theme
	if zeroTheme(theme) {
		theme = DefaultTheme()
	}

	accentStyle := theme.SuccessStyle
	titleStyle := theme.SuccessStyle
	if b.Failed {
		accentStyle = theme.ErrorStyle
		titleStyle = theme.ErrorStyle
	}

	top := accentStyle.Render("╷")
	bar := accentStyle.Render("│")
	bot := accentStyle.Render("╵")

	lines := []string{
		"  " + top,
		"  " + bar + "  " + titleStyle.Render(b.Title),
	}
	if strings.TrimSpace(b.ID) != "" {
		lines = append(lines, "  "+bar+"  "+theme.MutedStyle.Render(b.ID))
	}
	for _, d := range b.Details {
		if strings.TrimSpace(d) == "" {
			lines = append(lines, "  "+bar)
		} else {
			lines = append(lines, "  "+bar+"  "+d)
		}
	}
	lines = append(lines, "  "+bot)
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/ui/ -run TestResultBlock -v
```

Expected: all 3 pass.

- [ ] **Step 5: Commit**

```bash
git add apps/cli/internal/ui/result.go apps/cli/internal/ui/result_test.go
git commit -m "feat(cli/ui): add ResultBlock primitive"
```

---

## Task 3: `BgFillRow` primitive

**Files:**
- Create: `apps/cli/internal/ui/bgbar.go`
- Create: `apps/cli/internal/ui/bgbar_test.go`

- [ ] **Step 1: Write failing tests**

Create `apps/cli/internal/ui/bgbar_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBgFillRowZeroFractionNoBackground(t *testing.T) {
	got := BgFillRow("hello world", 0.0, lipgloss.Color("#ffffff"), lipgloss.Color("#123456"))
	// With 0 fill fraction: no background ANSI codes, just foreground-styled text
	plain := stripANSI(got)
	if plain != "hello world" {
		t.Fatalf("BgFillRow() plain = %q, want %q", plain, "hello world")
	}
}

func TestBgFillRowFullFractionAppliesBackground(t *testing.T) {
	got := BgFillRow("hello", 1.0, lipgloss.Color("#c0caf5"), lipgloss.Color("#1f2335"))
	// full fill: output contains the text
	if !strings.Contains(stripANSI(got), "hello") {
		t.Fatalf("BgFillRow() = %q, missing text", got)
	}
	// and contains ANSI background escape
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("BgFillRow() = %q, want ANSI escape codes", got)
	}
}

func TestBgFillRowHalfFractionSplitsText(t *testing.T) {
	text := "abcdefgh" // 8 runes
	got := BgFillRow(text, 0.5, lipgloss.Color("#c0caf5"), lipgloss.Color("#1f2335"))
	plain := stripANSI(got)
	if plain != text {
		t.Fatalf("BgFillRow() plain = %q, want %q", plain, text)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/ui/ -run TestBgFillRow -v
```

Expected: compile error — `BgFillRow undefined`

- [ ] **Step 3: Implement `bgbar.go`**

Create `apps/cli/internal/ui/bgbar.go`:

```go
package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// BgFillRow renders text with a proportional background color fill.
// text must contain no ANSI codes. fillFraction ∈ [0, 1] is the fraction of
// visible characters that receive bgColor as background. fgColor applies to all text.
//
// Used by the leaderboard to show score proportion as a background fill behind each row.
func BgFillRow(text string, fillFraction float64, fgColor, bgColor lipgloss.Color) string {
	if fillFraction < 0 {
		fillFraction = 0
	}
	if fillFraction > 1 {
		fillFraction = 1
	}

	runes := []rune(text)
	totalVW := runewidth.StringWidth(text)
	if totalVW == 0 {
		return text
	}

	fillVW := int(float64(totalVW) * fillFraction)

	// Split runes at the fill boundary (visual width, not byte count).
	splitIdx := 0
	accum := 0
	for i, r := range runes {
		if accum >= fillVW {
			splitIdx = i
			break
		}
		accum += runewidth.RuneWidth(r)
		splitIdx = i + 1
	}

	filled := string(runes[:splitIdx])
	rest := string(runes[splitIdx:])

	filledStyle := lipgloss.NewStyle().Foreground(fgColor).Background(bgColor)
	restStyle := lipgloss.NewStyle().Foreground(fgColor)

	if fillFraction >= 1.0 || rest == "" {
		return filledStyle.Render(text)
	}
	if fillFraction <= 0.0 || filled == "" {
		return restStyle.Render(text)
	}
	return filledStyle.Render(filled) + restStyle.Render(rest)
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/ui/ -run TestBgFillRow -v
```

Expected: all 3 pass.

- [ ] **Step 5: Run all ui tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/ui/ -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add apps/cli/internal/ui/bgbar.go apps/cli/internal/ui/bgbar_test.go
git commit -m "feat(cli/ui): add BgFillRow primitive for leaderboard"
```

---

## Task 4: Submit live renderer — 4-line in-place region

**Files:**
- Modify: `apps/cli/internal/commands/submit_live.go`
- Modify: `apps/cli/internal/commands/submit_live_test.go`

The renderer manages a 4-line block (blank + spinner + bar + stages). On `Start()` it prints 4 lines. On each tick it rewrites them using `\x1b[4A` (cursor up 4). On `Stop()` it erases the block with `\x1b[4A\x1b[J`.

- [ ] **Step 1: Write failing tests**

Replace `apps/cli/internal/commands/submit_live_test.go`:

```go
package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatSubmitLiveBlockContainsSpinnerStatusAndElapsed(t *testing.T) {
	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 4, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "running")

	lines := r.renderLines()

	// line[1] is the spinner line: must contain status and elapsed
	combined := strings.Join(lines[:], "\n")
	for _, want := range []string{"running", "4s"} {
		if !strings.Contains(stripANSIForTest(combined), want) {
			t.Fatalf("renderLines() = %v, missing %q", lines, want)
		}
	}
}

func TestFormatSubmitLiveBlockContainsProgressBar(t *testing.T) {
	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 2, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "queued")

	lines := r.renderLines()
	// line[2] is the progress bar
	barLine := stripANSIForTest(lines[2])
	if !strings.Contains(barLine, "█") && !strings.Contains(barLine, "%") {
		t.Fatalf("bar line = %q, want progress bar characters", barLine)
	}
}

func TestFormatSubmitLiveBlockStageIndicators(t *testing.T) {
	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 1, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "running")

	lines := r.renderLines()
	stageLine := stripANSIForTest(lines[3])
	// "running" → stages 0+1 complete, stage 2 active
	for _, want := range []string{"packed", "queued", "running", "scoring"} {
		if !strings.Contains(stageLine, want) {
			t.Fatalf("stage line = %q, missing %q", stageLine, want)
		}
	}
}

func TestSubmitLiveRendererStartsAndStops(t *testing.T) {
	var buf bytes.Buffer
	startedAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	r := newSubmitLiveRenderer(&buf, func() time.Time {
		return startedAt.Add(2 * time.Second)
	}, startedAt, "queued")
	r.ticker = time.NewTicker(time.Hour) // prevent real ticks
	defer r.ticker.Stop()

	if err := r.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := r.Update("running"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	out := buf.String()
	// Must contain cursor-up sequence to rewrite in place
	if !strings.Contains(out, "\x1b[4A") {
		t.Fatalf("output = %q, want cursor-up-4 escape", out)
	}
	// Must contain clear-to-end-of-screen on Stop
	if !strings.Contains(out, "\x1b[J") {
		t.Fatalf("output = %q, want clear-to-end escape on stop", out)
	}
	// Must contain status and elapsed text
	plain := stripANSIForTest(out)
	for _, want := range []string{"queued", "running", "2s"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("output plain = %q, missing %q", plain, want)
		}
	}
}

// stripANSIForTest removes ANSI escape codes for test assertions.
func stripANSIForTest(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' && s[i] != 'A' && s[i] != 'J' && s[i] != 'K' {
				i++
			}
			i++ // skip terminator
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/commands/ -run "TestFormatSubmitLive|TestSubmitLiveRenderer" -v
```

Expected: compile errors — `renderLines`, `newSubmitLiveRenderer` signature mismatch.

- [ ] **Step 3: Rewrite `submit_live.go`**

Replace the entire file:

```go
package commands

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"labkit.local/apps/cli/internal/ui"
	"golang.org/x/term"
)

const (
	liveNumLines   = 4
	liveCursorUp   = "\x1b[4A"
	liveClearLine  = "\r\x1b[2K"
	liveClearToEnd = "\x1b[J"
	liveBarWidth   = 24
)

var submitLiveSpinnerFrames = []string{"◐", "◓", "◑", "◒"}

var submitOutputIsTTY = func(out io.Writer) bool {
	fdProvider, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return term.IsTerminal(int(fdProvider.Fd()))
}

// submitLiveStages defines the 4 pipeline stages in display order.
var submitLiveStages = []string{"packed", "queued", "running", "scoring"}

// submitStatusToStageIndex maps a submission status string to the index of
// the currently active stage (0=packed, 1=queued, 2=running, 3=scoring/done).
func submitStatusToStageIndex(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "prepared", "uploaded":
		return 0
	case "queued":
		return 1
	case "running":
		return 2
	default:
		return 3
	}
}

type submitLiveRenderer struct {
	out        io.Writer
	now        func() time.Time
	startedAt  time.Time
	status     string
	ticker     *time.Ticker
	done       chan struct{}
	stopped    chan struct{}
	mu         sync.Mutex
	frameIndex int
	started    bool // true after first render
}

func newSubmitLiveRenderer(out io.Writer, now func() time.Time, startedAt time.Time, status string) *submitLiveRenderer {
	if out == nil {
		out = io.Discard
	}
	if now == nil {
		now = time.Now
	}
	return &submitLiveRenderer{
		out:       out,
		now:       now,
		startedAt: startedAt,
		status:    strings.TrimSpace(status),
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
}

func (r *submitLiveRenderer) Start() error {
	r.mu.Lock()
	if r.ticker == nil {
		r.ticker = time.NewTicker(120 * time.Millisecond)
	}
	err := r.renderCurrentFrameLocked()
	ticker := r.ticker
	r.mu.Unlock()
	if err != nil {
		return err
	}

	go func() {
		defer close(r.stopped)
		for {
			select {
			case <-ticker.C:
				r.mu.Lock()
				r.frameIndex++
				_ = r.renderCurrentFrameLocked()
				r.mu.Unlock()
			case <-r.done:
				return
			}
		}
	}()
	return nil
}

func (r *submitLiveRenderer) Update(status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = strings.TrimSpace(status)
	return r.renderCurrentFrameLocked()
}

func (r *submitLiveRenderer) Stop() error {
	if r.ticker != nil {
		r.ticker.Stop()
	}
	select {
	case <-r.done:
	default:
		close(r.done)
	}
	<-r.stopped
	r.mu.Lock()
	defer r.mu.Unlock()
	// Move cursor to top of live block and clear to end of screen.
	_, err := fmt.Fprint(r.out, liveCursorUp+liveClearToEnd)
	return err
}

// renderLines returns the 4 lines that make up the live block (no ANSI cursor sequences).
// Line 0: blank spacing line.
// Line 1: spinner + status + elapsed (right-aligned).
// Line 2: progress bar + percentage.
// Line 3: stage indicators.
func (r *submitLiveRenderer) renderLines() [4]string {
	theme := ui.DefaultTheme()
	stageIdx := submitStatusToStageIndex(r.status)
	elapsed := r.now().Sub(r.startedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	elapsedStr := theme.MutedStyle.Render(elapsed.Round(time.Second).String())

	// Spinner: cycles through frames at the active stage position.
	frame := submitLiveSpinnerFrames[r.frameIndex%len(submitLiveSpinnerFrames)]
	spinnerStr := theme.InfoStyle.Render(frame)

	statusDisplay := r.status
	if statusDisplay == "" {
		statusDisplay = "waiting"
	}

	// Line 1: "  ◐ running                          4s"
	line1 := fmt.Sprintf("  %s %s", spinnerStr, theme.ValueStyle.Render(statusDisplay))
	// Pad to 50 chars (ANSI-aware), then append elapsed
	const spinnerLineWidth = 50
	visWidth := len([]rune(ui.StripANSI(line1)))
	if visWidth < spinnerLineWidth {
		line1 += strings.Repeat(" ", spinnerLineWidth-visWidth)
	}
	line1 += "  " + elapsedStr

	// Line 2: progress bar
	fraction := float64(stageIdx+1) / float64(len(submitLiveStages))
	filled := int(fraction * float64(liveBarWidth))
	if filled > liveBarWidth {
		filled = liveBarWidth
	}
	bar := theme.InfoStyle.Render(strings.Repeat("█", filled)) +
		theme.MutedStyle.Render(strings.Repeat("░", liveBarWidth-filled))
	pct := int(fraction * 100)
	line2 := fmt.Sprintf("  %s  %s", bar, theme.MutedStyle.Render(fmt.Sprintf("%d%%", pct)))

	// Line 3: stage indicators
	parts := make([]string, len(submitLiveStages))
	for i, s := range submitLiveStages {
		switch {
		case i < stageIdx:
			parts[i] = theme.SuccessStyle.Render("✓") + " " + theme.MutedStyle.Render(s)
		case i == stageIdx:
			parts[i] = theme.InfoStyle.Render("●") + " " + theme.ValueStyle.Render(s)
		default:
			parts[i] = theme.MutedStyle.Render("○ "+s)
		}
	}
	line3 := "  " + strings.Join(parts, "   ")

	return [4]string{"", line1, line2, line3}
}

func (r *submitLiveRenderer) renderCurrentFrameLocked() error {
	lines := r.renderLines()
	if !r.started {
		// First render: just print 4 lines with newlines.
		for _, line := range lines {
			if _, err := fmt.Fprintln(r.out, line); err != nil {
				return err
			}
		}
		r.started = true
		return nil
	}
	// Subsequent renders: move cursor up 4, rewrite each line.
	if _, err := fmt.Fprint(r.out, liveCursorUp); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprint(r.out, liveClearLine+line+"\n"); err != nil {
			return err
		}
	}
	return nil
}
```

Note: `ui.StripANSI` is not currently exported. Add this export to `theme.go`:

```go
// StripANSI removes ANSI escape codes from s. Exported for use in commands.
func StripANSI(s string) string { return stripANSI(s) }
```

Also update the `newSubmitLiveRenderer` call-sites in `lab_commands.go` — the signature changed (removed `phase` parameter):

```go
// old: newSubmitLiveRenderer(deps.Out, deps.Now, client.now().UTC(), "waiting", current.Status)
// new: newSubmitLiveRenderer(deps.Out, deps.Now, client.now().UTC(), current.Status)
```

- [ ] **Step 4: Add `StripANSI` export to `theme.go`**

Edit `apps/cli/internal/ui/theme.go` — add after the `stripANSI` function:

```go
// StripANSI removes ANSI escape codes. Exported for use outside ui package.
func StripANSI(s string) string { return stripANSI(s) }
```

- [ ] **Step 5: Fix `newSubmitLiveRenderer` call in `lab_commands.go`**

Find the line:
```go
live = newSubmitLiveRenderer(deps.Out, deps.Now, client.now().UTC(), "waiting", current.Status)
```

Replace with:
```go
live = newSubmitLiveRenderer(deps.Out, deps.Now, client.now().UTC(), current.Status)
```

- [ ] **Step 6: Run tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/commands/ -run "TestFormatSubmitLive|TestSubmitLiveRenderer" -v
```

Expected: all pass.

- [ ] **Step 7: Run all CLI tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/...
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add apps/cli/internal/ui/theme.go apps/cli/internal/commands/submit_live.go apps/cli/internal/commands/submit_live_test.go apps/cli/internal/commands/lab_commands.go
git commit -m "feat(cli): 4-line live submit renderer with progress bar"
```

---

## Task 5: Update submit render functions in `lab_commands.go`

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`

Changes: `renderSubmissionTaskStart`, `renderSubmissionFinal`, `renderSubmissionScores`, `renderSubmissionSummary`, `renderSubmissionInterruptHint`, `submitWaitInterrupted`.

- [ ] **Step 1: Write tests for new render output format**

Add to `apps/cli/internal/commands/submit_test.go` (find the file and append):

```go
func TestRenderSubmissionTaskStartNewFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := renderSubmissionTaskStart(&buf, "matrix-mul"); err != nil {
		t.Fatalf("error = %v", err)
	}
	got := buf.String()
	// Must contain ● and lab ID
	for _, want := range []string{"●", "matrix-mul"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderSubmissionTaskStart() = %q, missing %q", got, want)
		}
	}
}

func TestRenderSubmissionFinalPassedContainsResultBlock(t *testing.T) {
	var buf bytes.Buffer
	now := time.Date(2026, 4, 1, 12, 0, 5, 0, time.UTC)
	finished := now
	detail := submissionDetailResponse{
		ID:         "4f3a9b2c",
		Status:     "completed",
		Verdict:    "scored",
		CreatedAt:  now.Add(-5 * time.Second),
		FinishedAt: &finished,
		Scores:     []submissionScoreItem{{MetricID: "score", Value: 92.6}},
	}
	lab := manifest.PublicManifest{
		Metrics: []manifest.MetricSection{{ID: "score", Name: "score"}},
	}
	if err := renderSubmissionFinal(&buf, lab, detail); err != nil {
		t.Fatalf("error = %v", err)
	}
	got := buf.String()
	for _, want := range []string{"╷", "│", "╵", "4f3a9b2c", "92.6"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderSubmissionFinal() = %q, missing %q", got, want)
		}
	}
}

func TestRenderSubmissionFinalFailedContainsErrorAccent(t *testing.T) {
	var buf bytes.Buffer
	now := time.Date(2026, 4, 1, 12, 0, 2, 0, time.UTC)
	finished := now
	detail := submissionDetailResponse{
		ID:         "aabbccdd",
		Status:     "completed",
		Verdict:    "failed",
		Message:    "test failed",
		CreatedAt:  now.Add(-2 * time.Second),
		FinishedAt: &finished,
	}
	if err := renderSubmissionFinal(&buf, manifest.PublicManifest{}, detail); err != nil {
		t.Fatalf("error = %v", err)
	}
	got := buf.String()
	for _, want := range []string{"╷", "aabbccdd", "FAILED"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderSubmissionFinal() = %q, missing %q", got, want)
		}
	}
}
```

Add required imports to `submit_test.go` — check what's already imported and add:
```go
"labkit.local/packages/go/manifest"
```

- [ ] **Step 2: Run failing tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/commands/ -run "TestRenderSubmission" -v
```

Expected: tests fail because the old format doesn't match.

- [ ] **Step 3: Rewrite `renderSubmissionTaskStart`**

Find and replace in `lab_commands.go`:

```go
func renderSubmissionTaskStart(out io.Writer, labID string) error {
	if out == nil {
		out = io.Discard
	}
	_, err := fmt.Fprintf(out, "Submitting %s\n  steps   prepared -> uploaded -> waiting\n", labID)
	return err
}
```

Replace with:

```go
func renderSubmissionTaskStart(out io.Writer, labID string) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()
	title := theme.InfoStyle.Render("●") + " " + theme.TitleStyle.Render("Submitting") + "  " + theme.ValueStyle.Render(labID)
	_, err := fmt.Fprintln(out, title)
	return err
}
```

- [ ] **Step 4: Rewrite `renderSubmissionFinal`**

Find and replace the entire `renderSubmissionFinal` function:

```go
func renderSubmissionFinal(out io.Writer, lab manifest.PublicManifest, detail submissionDetailResponse) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()

	elapsed := ""
	if detail.FinishedAt != nil && !detail.CreatedAt.IsZero() {
		elapsed = "   " + detail.FinishedAt.Sub(detail.CreatedAt).Round(time.Second).String()
	}

	passed := strings.EqualFold(detail.Verdict, "scored") || strings.EqualFold(detail.Status, "completed") && detail.Verdict == ""
	failed := !passed && detail.Verdict != ""

	// Build title line e.g. "PASSED   5s" or "FAILED   2s"
	var titleStr string
	if failed || strings.EqualFold(detail.Status, "failed") || strings.EqualFold(detail.Verdict, "failed") {
		titleStr = theme.ErrorStyle.Render("FAILED") + elapsed
	} else {
		titleStr = theme.SuccessStyle.Render("PASSED") + elapsed
	}

	// Build failure detail lines from message and detail JSON.
	detailLines := renderSubmissionDetailLines(detail.Detail)
	if msg := strings.TrimSpace(detail.Message); msg != "" && len(detailLines) == 0 {
		detailLines = []string{msg}
	}

	isFailure := failed || strings.EqualFold(detail.Verdict, "failed") || strings.EqualFold(detail.Status, "failed")
	block := ui.ResultBlock{
		Title:   titleStr,
		ID:      detail.ID,
		Details: detailLines,
		Failed:  isFailure,
	}

	// Title line: ✓ Submitted  labID  or  ✗ Submitted  labID
	var prefix string
	if isFailure {
		prefix = theme.ErrorStyle.Render("✗")
	} else {
		prefix = theme.SuccessStyle.Render("✓")
	}
	titleLine := prefix + " " + theme.TitleStyle.Render("Submitted")

	if _, err := fmt.Fprintln(out, titleLine); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, block.Render()); err != nil {
		return err
	}

	if !isFailure && len(detail.Scores) > 0 {
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		return renderSubmissionScores(out, lab, detail.Scores)
	}
	return nil
}
```

- [ ] **Step 5: Rewrite `renderSubmissionScores`**

Find and replace:

```go
func renderSubmissionScores(out io.Writer, lab manifest.PublicManifest, scores []submissionScoreItem) error {
	if len(scores) == 0 {
		return nil
	}
	ordered := orderedSubmissionScores(lab, scores)
	if _, err := fmt.Fprintln(out, "Scores"); err != nil {
		return err
	}
	width := 0
	for _, score := range ordered {
		if len(score.DisplayLabel) > width {
			width = len(score.DisplayLabel)
		}
	}
	for _, score := range ordered {
		if _, err := fmt.Fprintf(out, "  %-*s   %s\n", width, score.DisplayLabel, score.Value); err != nil {
			return err
		}
	}
	return nil
}
```

Replace with:

```go
func renderSubmissionScores(out io.Writer, lab manifest.PublicManifest, scores []submissionScoreItem) error {
	if len(scores) == 0 {
		return nil
	}
	theme := ui.DefaultTheme()
	ordered := orderedSubmissionScores(lab, scores)
	if _, err := fmt.Fprintln(out, theme.LabelStyle.Render("  Scores")); err != nil {
		return err
	}
	width := 0
	for _, score := range ordered {
		if len(score.DisplayLabel) > width {
			width = len(score.DisplayLabel)
		}
	}
	for _, score := range ordered {
		valueStr := theme.SuccessStyle.Render(score.Value)
		if _, err := fmt.Fprintf(out, "  %-*s   %s\n", width, theme.MutedStyle.Render(score.DisplayLabel), valueStr); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 6: Rewrite `renderSubmissionSummary` (detach mode)**

Find and replace:

```go
func renderSubmissionSummary(out io.Writer, submission submissionResponse, verdict, elapsed string) error {
	if out == nil {
		out = io.Discard
	}
	if _, err := fmt.Fprintf(out, "Submitted   %s\n", strings.TrimSpace(submission.Status)); err != nil {
		return err
	}
	return renderSubmissionKV(out, "id", submission.ID)
}
```

Replace with:

```go
func renderSubmissionSummary(out io.Writer, submission submissionResponse, verdict, elapsed string) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()
	title := theme.InfoStyle.Render("●") + " " + theme.TitleStyle.Render("Submitted") + "  " +
		theme.MutedStyle.Render("(detached)")
	if _, err := fmt.Fprintln(out, title); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  %s  %s\n",
		theme.MutedStyle.Render("id"),
		theme.ValueStyle.Render(submission.ID)); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 7: Rewrite `renderSubmissionInterruptHint`**

Find and replace:

```go
func renderSubmissionInterruptHint(out io.Writer, submissionID string) error {
	_, err := fmt.Fprintln(out, ui.DetailBlock{
		Title: "Waiting interrupted",
		Lines: []string{
			"Submission " + submissionID + " is still running on the server.",
			"Use `labkit history` or `labkit board` later to check the final result.",
		},
	}.Render())
	return err
}
```

Replace with:

```go
func renderSubmissionInterruptHint(out io.Writer, submissionID string) error {
	theme := ui.DefaultTheme()
	if _, err := fmt.Fprintln(out, theme.WarningStyle.Render("⚠ Waiting interrupted")); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  %s is still running on the server.\n  Check results later with %s or %s.\n",
		theme.MutedStyle.Render(submissionID),
		theme.InfoStyle.Render("labkit history"),
		theme.InfoStyle.Render("labkit board"),
	)
	return err
}
```

- [ ] **Step 8: Run tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/commands/ -run "TestRenderSubmission" -v
```

Expected: all pass.

- [ ] **Step 9: Run all CLI tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/...
```

Expected: all pass.

- [ ] **Step 10: Commit**

```bash
git add apps/cli/internal/commands/lab_commands.go apps/cli/internal/commands/submit_test.go
git commit -m "feat(cli): redesign submit output with ResultBlock and new title format"
```

---

## Task 6: Update leaderboard (`renderBoard`)

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`

- [ ] **Step 1: Write test**

Add to `submit_test.go` (or create `board_test.go` — but since there is no separate board test file, add to `submit_test.go`):

```go
func TestRenderBoardShowsMedalsAndBgFill(t *testing.T) {
	var buf bytes.Buffer
	lab := manifest.PublicManifest{}
	board := boardResponse{
		SelectedMetric: "score",
		Metrics:        []boardMetricResponse{{ID: "score", Name: "score"}},
		Rows: []boardRowResponse{
			{Rank: 1, Nickname: "alice", Scores: []boardScoreResponse{{MetricID: "score", Value: 95.5}}, UpdatedAt: time.Now().Add(-2 * time.Hour)},
			{Rank: 2, Nickname: "bob", Scores: []boardScoreResponse{{MetricID: "score", Value: 80.0}}, UpdatedAt: time.Now().Add(-5 * time.Hour)},
		},
	}
	if err := renderBoard(&buf, lab, board); err != nil {
		t.Fatalf("error = %v", err)
	}
	got := buf.String()
	for _, want := range []string{"🥇", "alice", "95.5", "bob", "ago"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderBoard() = %q, missing %q", got, want)
		}
	}
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/commands/ -run TestRenderBoard -v
```

Expected: fails — old format has no medals.

- [ ] **Step 3: Rewrite `renderBoard`**

Find and replace the entire `renderBoard` function:

```go
func renderBoard(out io.Writer, lab manifest.PublicManifest, board boardResponse) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()

	// Build title
	metricSuffix := ""
	if strings.TrimSpace(board.SelectedMetric) != "" {
		metricSuffix = " · sorted by " + board.SelectedMetric
	}
	count := fmt.Sprintf(" · %d participants", len(board.Rows))
	title := theme.TitleStyle.Render("Leaderboard") +
		theme.MutedStyle.Render(metricSuffix+count)

	// Metric tab line (shows available metrics, current one bold)
	if len(board.Metrics) > 1 {
		tabs := make([]string, len(board.Metrics))
		for i, m := range board.Metrics {
			if m.Selected || strings.EqualFold(m.ID, board.SelectedMetric) {
				tabs[i] = theme.TitleStyle.Render(m.Name)
			} else {
				tabs[i] = theme.MutedStyle.Render(m.Name)
			}
		}
		if _, err := fmt.Fprintln(out, title+"\n  "+strings.Join(tabs, " · ")); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(out, title); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}

	// Column widths (fixed layout):
	//   rankW=5  gap=2  nickW=16  gap=2  scoreW=8  gap=2  updatedW=8
	const (
		rankW    = 5
		nickW    = 16
		scoreW   = 8
		updatedW = 8
		gap      = "  "
	)
	rowWidth := rankW + len(gap) + nickW + len(gap) + scoreW + len(gap) + updatedW

	// Header
	header := ui.PadRight(theme.MutedStyle.Render("#"), rankW) + gap +
		ui.PadRight(theme.MutedStyle.Render("NICKNAME"), nickW) + gap +
		ui.PadRight(theme.MutedStyle.Render("SCORE"), scoreW) + gap +
		theme.MutedStyle.Render("UPDATED")
	if _, err := fmt.Fprintln(out, "  "+header); err != nil {
		return err
	}
	sep := theme.SeparatorStyle.Render(strings.Repeat("─", rowWidth))
	if _, err := fmt.Fprintln(out, "  "+sep); err != nil {
		return err
	}

	// Determine max score for fill fraction
	unitByID := make(map[string]string, len(lab.Metrics))
	for _, m := range lab.Metrics {
		unitByID[m.ID] = m.Unit
	}
	maxScore := float32(0)
	for _, row := range board.Rows {
		for _, s := range row.Scores {
			if s.MetricID == board.SelectedMetric && s.Value > maxScore {
				maxScore = s.Value
			}
		}
	}

	now := time.Now()
	for _, row := range board.Rows {
		// Rank/medal column
		var rankStr string
		switch row.Rank {
		case 1:
			rankStr = "🥇"
		case 2:
			rankStr = "🥈"
		case 3:
			rankStr = "🥉"
		default:
			rankStr = fmt.Sprintf("%d", row.Rank)
		}

		// Find selected score value
		scoreVal := float32(0)
		for _, s := range row.Scores {
			if s.MetricID == board.SelectedMetric {
				scoreVal = s.Value
			}
		}
		scoreStr := formatScore(scoreVal, unitByID[board.SelectedMetric])
		updatedStr := ui.RelativeTime(row.UpdatedAt, now)

		// Determine row foreground and background colors
		var fgColor, bgColor lipgloss.Color
		switch row.Rank {
		case 1:
			fgColor = lipgloss.Color("#e0af68")
			bgColor = lipgloss.Color("#2a2015")
		default:
			fgColor = lipgloss.Color("#c0caf5")
			bgColor = lipgloss.Color("#1e2030")
		}

		// Build plain-text row (no ANSI) for BgFillRow
		plainRank := ui.PadRight(rankStr, rankW)
		plainNick := ui.PadRight(row.Nickname, nickW)
		plainScore := ui.PadRight(scoreStr, scoreW)
		plainUpdated := ui.PadRight(updatedStr, updatedW)
		plainRow := plainRank + gap + plainNick + gap + plainScore + gap + plainUpdated

		// Fill fraction
		fillFraction := 0.0
		if maxScore > 0 {
			fillFraction = float64(scoreVal) / float64(maxScore)
		}

		rendered := "  " + ui.BgFillRow(plainRow, fillFraction, fgColor, bgColor)
		if _, err := fmt.Fprintln(out, rendered); err != nil {
			return err
		}
	}

	return nil
}
```

Note: `ui.PadRight` and `lipgloss.Color` need to be accessible. Export `PadRight` from `ui/table.go`:

```go
// PadRight pads text on the right to the given visible width. Exported for commands package.
func PadRight(text string, width int) string { return padRight(text, width) }
```

Also add the `lipgloss` import to `lab_commands.go`:
```go
"github.com/charmbracelet/lipgloss"
```

And remove the `clioutput` import from `lab_commands.go` if it's no longer used after this change (it was only used in `renderBoard`).

- [ ] **Step 4: Export `PadRight` from `ui/table.go`**

Add after the `padRight` function:

```go
// PadRight pads text to the given visible width. Exported for use in commands.
func PadRight(text string, width int) string { return padRight(text, width) }
```

- [ ] **Step 5: Update imports in `lab_commands.go`**

Remove:
```go
clioutput "labkit.local/apps/cli/internal/output"
```

Add:
```go
"github.com/charmbracelet/lipgloss"
```

- [ ] **Step 6: Run tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/commands/ -run TestRenderBoard -v
```

Expected: passes.

- [ ] **Step 7: Run all CLI tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/...
```

Expected: all pass. If any test depends on the old `clioutput` import, fix it.

- [ ] **Step 8: Commit**

```bash
git add apps/cli/internal/ui/table.go apps/cli/internal/commands/lab_commands.go apps/cli/internal/commands/submit_test.go
git commit -m "feat(cli): redesign leaderboard with medals and BgFillRow"
```

---

## Task 7: Update `renderHistory`, `runNick`, `runTrack`, `keys`, `revoke`

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Modify: `apps/cli/internal/commands/keys.go`

- [ ] **Step 1: Write tests**

Add to `submit_test.go`:

```go
func TestRenderHistoryShowsColoredStatusAndRelativeTime(t *testing.T) {
	var buf bytes.Buffer
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	finished := now.Add(-1 * time.Hour)
	history := historyResponse{
		Submissions: []historyItemResponse{
			{ID: "abc", Status: "completed", Verdict: "scored", CreatedAt: now.Add(-2 * time.Hour), FinishedAt: &finished},
			{ID: "def", Status: "failed", Verdict: "failed", CreatedAt: now.Add(-5 * time.Hour)},
		},
	}
	if err := renderHistory(&buf, history); err != nil {
		t.Fatalf("error = %v", err)
	}
	got := buf.String()
	for _, want := range []string{"abc", "def", "ago", "completed", "failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderHistory() = %q, missing %q", got, want)
		}
	}
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/internal/commands/ -run TestRenderHistory -v
```

Expected: fails — old format doesn't use relative time.

- [ ] **Step 3: Rewrite `renderHistory`**

Find and replace the entire `renderHistory` function:

```go
func renderHistory(out io.Writer, history historyResponse) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()
	now := time.Now()

	count := fmt.Sprintf("  %s", theme.MutedStyle.Render(fmt.Sprintf("%d submissions", len(history.Submissions))))
	title := theme.TitleStyle.Render("Submission history")
	if _, err := fmt.Fprintln(out, title+"\n"); err != nil {
		return err
	}

	const (
		idW      = 10
		statusW  = 10
		verdictW = 10
		gap      = "  "
	)
	header := ui.PadRight(theme.MutedStyle.Render("ID"), idW) + gap +
		ui.PadRight(theme.MutedStyle.Render("STATUS"), statusW) + gap +
		ui.PadRight(theme.MutedStyle.Render("VERDICT"), verdictW) + gap +
		theme.MutedStyle.Render("SUBMITTED")
	rowWidth := idW + len(gap) + statusW + len(gap) + verdictW + len(gap) + 10
	sep := theme.SeparatorStyle.Render(strings.Repeat("─", rowWidth))

	if _, err := fmt.Fprintln(out, "  "+header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  "+sep); err != nil {
		return err
	}

	for _, item := range history.Submissions {
		shortID := item.ID
		if len(shortID) > idW {
			shortID = shortID[:idW]
		}

		// Color status
		var statusStr string
		switch strings.ToLower(item.Status) {
		case "completed", "done":
			statusStr = theme.SuccessStyle.Render(item.Status)
		case "failed", "error":
			statusStr = theme.ErrorStyle.Render(item.Status)
		case "queued", "pending":
			statusStr = theme.WarningStyle.Render(item.Status)
		case "running":
			statusStr = theme.InfoStyle.Render(item.Status)
		default:
			statusStr = theme.MutedStyle.Render(item.Status)
		}

		// Color verdict
		var verdictStr string
		switch strings.ToLower(item.Verdict) {
		case "scored", "passed":
			verdictStr = theme.SuccessStyle.Render(item.Verdict)
		case "failed", "error":
			verdictStr = theme.ErrorStyle.Render(item.Verdict)
		default:
			verdictStr = theme.MutedStyle.Render(item.Verdict)
		}
		if strings.TrimSpace(item.Verdict) == "" {
			verdictStr = theme.MutedStyle.Render("—")
		}

		submittedStr := ui.RelativeTime(item.CreatedAt, now)

		row := ui.PadRight(theme.ValueStyle.Render(shortID), idW) + gap +
			ui.PadRight(statusStr, statusW) + gap +
			ui.PadRight(verdictStr, verdictW) + gap +
			theme.MutedStyle.Render(submittedStr)

		if _, err := fmt.Fprintln(out, "  "+row); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, count)
	return err
}
```

- [ ] **Step 4: Update `runNick` output**

Find:
```go
fmt.Fprintln(deps.Out, ui.Success(fmt.Sprintf("Nickname updated to %s.", profile.Nickname)))
```

Replace with:
```go
theme := ui.DefaultTheme()
fmt.Fprintln(deps.Out, theme.SuccessStyle.Render("✓")+" "+theme.TitleStyle.Render("Nickname updated")+"  "+theme.ValueStyle.Render(profile.Nickname))
```

- [ ] **Step 5: Update `runTrack` output**

Find:
```go
fmt.Fprintln(deps.Out, ui.Success(fmt.Sprintf("Track updated to %s.", profile.Track)))
```

Replace with:
```go
theme := ui.DefaultTheme()
fmt.Fprintln(deps.Out, theme.SuccessStyle.Render("✓")+" "+theme.TitleStyle.Render("Track set")+"  "+theme.ValueStyle.Render(profile.Track))
```

- [ ] **Step 6: Rewrite `keys.go` — `runListKeys`**

Find and replace the rendering section in `runListKeys` (the `rows` building and final print):

```go
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{
			strconv.FormatInt(key.ID, 10),
			key.PublicKey,
			key.DeviceName,
			key.CreatedAt.Format(time.RFC3339),
		})
	}
	_, err = fmt.Fprintln(deps.Out, ui.Table("Bound keys", []string{"ID", "PUBLIC KEY", "DEVICE", "CREATED AT"}, rows))
	return err
```

Replace with:

```go
	theme := ui.DefaultTheme()
	now := time.Now()

	const (
		idW     = 10
		keyW    = 20
		deviceW = 14
		gap     = "  "
	)
	title := theme.TitleStyle.Render("Bound keys") +
		"  " + theme.MutedStyle.Render(fmt.Sprintf("%d total", len(keys)))
	if _, err = fmt.Fprintln(deps.Out, title+"\n"); err != nil {
		return err
	}
	header := ui.PadRight(theme.MutedStyle.Render("ID"), idW) + gap +
		ui.PadRight(theme.MutedStyle.Render("DEVICE"), deviceW) + gap +
		theme.MutedStyle.Render("CREATED")
	rowWidth := idW + len(gap) + deviceW + len(gap) + 8
	sep := theme.SeparatorStyle.Render(strings.Repeat("─", rowWidth))
	if _, err = fmt.Fprintln(deps.Out, "  "+header); err != nil {
		return err
	}
	if _, err = fmt.Fprintln(deps.Out, "  "+sep); err != nil {
		return err
	}
	for _, key := range keys {
		shortID := strconv.FormatInt(key.ID, 10)
		row := ui.PadRight(theme.ValueStyle.Render(shortID), idW) + gap +
			ui.PadRight(theme.ValueStyle.Render(key.DeviceName), deviceW) + gap +
			theme.MutedStyle.Render(ui.RelativeTime(key.CreatedAt, now))
		if _, err = fmt.Fprintln(deps.Out, "  "+row); err != nil {
			return err
		}
	}
	return nil
```

Add required imports to `keys.go`:
```go
"fmt"
"strings"
```

(`time` is already imported.)

- [ ] **Step 7: Update `runRevokeKey` to print confirmation**

The current `runRevokeKey` returns nil on success with no output. Update the caller in the command:

```go
func NewRevokeCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	return &cobra.Command{
		Use:   "revoke <key-id>",
		Short: "Revoke a bound key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid key id")
			}
			if err := runRevokeKey(cmd.Context(), deps, keyID); err != nil {
				return err
			}
			theme := ui.DefaultTheme()
			fmt.Fprintln(deps.Out, theme.SuccessStyle.Render("✓")+" "+theme.TitleStyle.Render("Revoked")+"  "+theme.MutedStyle.Render(fmt.Sprintf("key %d", keyID)))
			return nil
		},
	}
}
```

- [ ] **Step 8: Run tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/... -v 2>&1 | tail -30
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add apps/cli/internal/commands/lab_commands.go apps/cli/internal/commands/keys.go apps/cli/internal/commands/submit_test.go
git commit -m "feat(cli): redesign history, keys, nick, track, revoke output"
```

---

## Task 8: Update auth output

**Files:**
- Modify: `apps/cli/internal/commands/auth.go`

The auth command outputs two phases:
1. During polling: device URL + code + spinner hint
2. On completion: `✓ Authorized` card

- [ ] **Step 1: Update the pending phase output**

In `auth.go`, find the section that prints the device authorization info:

```go
fmt.Fprintln(deps.Out, ui.Section("Device authorization", ui.Lines(
    ui.LabelValue("Verification URL", verificationURL),
    ui.LabelValue("Code", authorization.UserCode),
)))
```

Replace with:

```go
theme := ui.DefaultTheme()
serverHost := serverURL
if u, err2 := url.Parse(serverURL); err2 == nil {
    serverHost = u.Host
}
titleLine := theme.InfoStyle.Render("●") + " " + theme.TitleStyle.Render("Authorizing") + "  " + theme.MutedStyle.Render(serverHost)
fmt.Fprintln(deps.Out, titleLine)
fmt.Fprintln(deps.Out)
fmt.Fprintln(deps.Out, "  Open this URL in your browser:")
fmt.Fprintln(deps.Out, "  "+theme.InfoStyle.Render(verificationURL))
fmt.Fprintln(deps.Out)
fmt.Fprintf(deps.Out, "  Enter code:  %s\n", theme.TitleStyle.Render(authorization.UserCode))
fmt.Fprintln(deps.Out)
fmt.Fprintln(deps.Out, "  "+theme.MutedStyle.Render("◐ Waiting for authorization…"))
```

- [ ] **Step 2: Update the completion output**

Find:
```go
fmt.Fprintln(deps.Out, ui.Success("Device authorized."))
```

Replace with:

```go
theme2 := ui.DefaultTheme()
serverHost2 := cfg.ServerURL
if u, err2 := url.Parse(cfg.ServerURL); err2 == nil {
    serverHost2 = u.Host
}
completionTitle := theme2.SuccessStyle.Render("✓") + " " + theme2.TitleStyle.Render("Authorized") + "  " + theme2.MutedStyle.Render(serverHost2)
fmt.Fprintln(deps.Out, completionTitle)
fmt.Fprintln(deps.Out)
fmt.Fprintf(deps.Out, "  %s  %s\n", theme2.MutedStyle.Render("key   "), theme2.ValueStyle.Render(fmt.Sprintf("%d", cfg.KeyID)))
fmt.Fprintf(deps.Out, "  %s  %s\n", theme2.MutedStyle.Render("server"), theme2.ValueStyle.Render(serverHost2))
```

- [ ] **Step 3: Compile check**

```bash
cd /home/starrydream/ICS2/LabKit && go build ./apps/cli/...
```

Expected: no errors.

- [ ] **Step 4: Run all tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add apps/cli/internal/commands/auth.go
git commit -m "feat(cli): redesign auth output with new title format"
```

---

## Task 9: Structured error format in `main.go`

**Files:**
- Modify: `apps/cli/cmd/labkit/main.go`

Currently errors are printed as plain `fmt.Fprintln(os.Stderr, err)`. Replace with the `✗ Error  …` format.

- [ ] **Step 1: Update `main.go`**

Replace the entire file:

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"labkit.local/apps/cli/internal/commands"
	"labkit.local/apps/cli/internal/config"
	"labkit.local/apps/cli/internal/ui"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		printCLIError(err)
		os.Exit(1)
	}
}

func newRootCommand() *commands.RootCommand {
	return commands.NewRootCommand(&commands.Dependencies{
		ConfigDir: config.ResolveDir(),
	})
}

func printCLIError(err error) {
	theme := ui.DefaultTheme()
	msg := strings.TrimSpace(err.Error())
	prefix := theme.ErrorStyle.Render("✗") + " " + theme.TitleStyle.Render("Error")
	fmt.Fprintln(os.Stderr, prefix)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  "+theme.MutedStyle.Render(msg))
}
```

Note: `commands.NewRootCommand` returns `*cobra.Command`, not a named type. Keep the return type as `*cobra.Command`:

```go
func newRootCommand() *cobra.Command {
```

And add the cobra import:
```go
"github.com/spf13/cobra"
```

Full corrected `main.go`:

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"labkit.local/apps/cli/internal/commands"
	"labkit.local/apps/cli/internal/config"
	"labkit.local/apps/cli/internal/ui"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		printCLIError(err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	return commands.NewRootCommand(&commands.Dependencies{
		ConfigDir: config.ResolveDir(),
	})
}

func printCLIError(err error) {
	theme := ui.DefaultTheme()
	msg := strings.TrimSpace(err.Error())
	prefix := theme.ErrorStyle.Render("✗") + " " + theme.TitleStyle.Render("Error")
	fmt.Fprintln(os.Stderr, prefix)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  "+theme.MutedStyle.Render(msg))
}
```

- [ ] **Step 2: Build to confirm**

```bash
cd /home/starrydream/ICS2/LabKit && go build ./apps/cli/...
```

Expected: no errors.

- [ ] **Step 3: Run all tests**

```bash
cd /home/starrydream/ICS2/LabKit && go test ./apps/cli/...
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add apps/cli/cmd/labkit/main.go
git commit -m "feat(cli): structured error output with ✗ Error format"
```

---

## Self-Review

### Spec Coverage

| Spec requirement | Task |
|---|---|
| Tokyo Night palette | Task 1 |
| Relative timestamps | Task 1 (`RelativeTime`) |
| `✓ / ● / ✗` title prefix format | Tasks 5, 7, 8, 9 |
| `╷ … ╵` ResultBlock for submit | Task 2, Task 5 |
| 4-line live progress bar | Task 4 |
| Stage indicator row (packed/queued/running/scoring) | Task 4 |
| Leaderboard medals (🥇🥈🥉) | Task 6 |
| Leaderboard BgFillRow | Tasks 3, 6 |
| Colored STATUS in history | Task 7 |
| Auth `● Authorizing` + `✓ Authorized` | Task 8 |
| Keys separator + relative time | Task 7 |
| Revoke confirmation | Task 7 |
| Nick/Track short confirmations | Task 7 |
| `✗ Error` format | Task 9 |
| Non-TTY plain output for submit | Preserved by keeping existing non-TTY path |

### Known Deviations from Spec

- **History SCORE column**: The `historyResponse` API does not return score values, only `status`, `verdict`, `message`. The implementation shows `status`, `verdict`, `submitted` — no score column.
- **Submit stage timing**: Per-stage elapsed times in the static completion output are omitted (API doesn't return them). The live renderer shows real-time elapsed during polling; the final output shows total elapsed only.
- **`labkit web` output**: Not explicitly tasked — the current output (`opening browser…`) is acceptable; no design change required.
