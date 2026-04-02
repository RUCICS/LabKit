package commands

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
	"labkit.local/apps/cli/internal/ui"
)

const (
	liveClearLine  = "\r\x1b[2K"
	liveClearToEnd = "\x1b[J"
	liveBarWidth   = 30 // fallback / minimum bar width in characters
	liveBlockLines = 6
)

var submitLiveSpinnerFrames = spinnerFrames

type submitBarColor struct {
	r int
	g int
	b int
}

var (
	submitLiveBarStart = submitBarColor{r: 122, g: 162, b: 247}
	submitLiveBarEnd   = submitBarColor{r: 115, g: 218, b: 202}
	submitLivePulse    = submitBarColor{r: 210, g: 245, b: 255}
)

var submitLiveThinLineGlyphs = []string{" ", "╶", "─"}

var submitOutputIsTTY = func(out io.Writer) bool {
	fdProvider, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return term.IsTerminal(int(fdProvider.Fd()))
}

var submitLiveSupportsThinLineBlocks = func() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	return runewidth.StringWidth("╶") == 1 && runewidth.StringWidth("─") == 1
}

// submitLiveStages defines the 4 pipeline stages in display order.
var submitLiveStages = []string{"submitting", "queued", "running", "scoring"}

// submitStatusToStageIndex maps a submission status to the index of the currently active stage.
// 0=submitting, 1=queued, 2=running, 3=scoring/done
func submitStatusToStageIndex(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "submitting", "", "contacting", "sending", "prepared", "uploaded":
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
	barWidth   int  // cached at construction; adapts to terminal width
	promptLine string
	hintLine   string
}

func newSubmitLiveRenderer(out io.Writer, now func() time.Time, startedAt time.Time, status string) *submitLiveRenderer {
	if out == nil {
		out = io.Discard
	}
	if now == nil {
		now = time.Now
	}
	barWidth := liveBarWidth
	if fdProvider, ok := out.(interface{ Fd() uintptr }); ok {
		if w, _, err := term.GetSize(int(fdProvider.Fd())); err == nil && w > 40 {
			if computed := w/3 - 4; computed > liveBarWidth {
				barWidth = computed
			}
			if barWidth > 56 {
				barWidth = 56
			}
		}
	}
	return &submitLiveRenderer{
		out:       out,
		now:       now,
		startedAt: startedAt,
		status:    strings.TrimSpace(status),
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
		barWidth:  barWidth,
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
	_, err := fmt.Fprint(r.out, liveCursorUpSequence(liveBlockLines)+liveClearToEnd)
	return err
}

func (r *submitLiveRenderer) SetPrompt(message, hint string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.promptLine = strings.TrimSpace(message)
	r.hintLine = strings.TrimSpace(hint)
	return r.renderCurrentFrameLocked()
}

func (r *submitLiveRenderer) ClearPrompt() error {
	return r.SetPrompt("", "")
}

// renderLines returns the live block lines (no cursor-movement sequences).
// [0] blank spacing line
// [1] spinner + status + elapsed
// [2] progress bar + percentage
// [3] stage indicators
// [4] optional prompt line
// [5] optional prompt hint
func (r *submitLiveRenderer) renderLines() [liveBlockLines]string {
	theme := ui.DefaultTheme()
	stageIdx := submitStatusToStageIndex(r.status)
	elapsed := r.now().Sub(r.startedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	elapsedStr := theme.MutedStyle.Render(elapsed.Round(time.Second).String())

	frame := submitLiveSpinnerFrames[r.frameIndex%len(submitLiveSpinnerFrames)]
	spinnerStr := theme.InfoStyle.Render(frame)

	statusDisplay := r.status
	if statusDisplay == "" {
		statusDisplay = "waiting"
	}

	// Line 1: "  ◐ running                          4s"
	line1content := fmt.Sprintf("  %s %s", spinnerStr, theme.ValueStyle.Render(statusDisplay))
	const spinnerLineWidth = 50
	visWidth := len([]rune(ui.StripANSI(line1content)))
	if visWidth < spinnerLineWidth {
		line1content += strings.Repeat(" ", spinnerLineWidth-visWidth)
	}
	line1 := line1content + "  " + elapsedStr

	// Line 2: progress bar — completedStages/totalStages (active stage not counted as done)
	fraction := float64(stageIdx) / float64(len(submitLiveStages))
	barWidth := r.barWidth
	bar := renderAnimatedSubmitBar(theme, barWidth, fraction, r.frameIndex)
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
			parts[i] = theme.MutedStyle.Render("○ " + s)
		}
	}
	line3 := "  " + strings.Join(parts, "   ")

	line4 := ""
	if r.promptLine != "" {
		line4 = "  " + theme.WarningStyle.Render(r.promptLine)
	}
	line5 := ""
	if r.hintLine != "" {
		line5 = "  " + theme.MutedStyle.Render(r.hintLine)
	}

	return [liveBlockLines]string{"", line1, line2, line3, line4, line5}
}

func (r *submitLiveRenderer) renderCurrentFrameLocked() error {
	lines := r.renderLines()
	if !r.started {
		// First render: print 4 lines with newlines.
		for _, line := range lines {
			if _, err := fmt.Fprintln(r.out, line); err != nil {
				return err
			}
		}
		r.started = true
		return nil
	}
	if _, err := fmt.Fprint(r.out, liveCursorUpSequence(liveBlockLines)); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprint(r.out, liveClearLine+line+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func liveCursorUpSequence(lines int) string {
	return fmt.Sprintf("\x1b[%dA", lines)
}

func renderAnimatedSubmitBar(theme ui.Theme, width int, fraction float64, frameIndex int) string {
	if width <= 0 {
		return ""
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	if !submitLiveSupportsThinLineBlocks() {
		return renderFallbackSubmitBar(theme, width, fraction, frameIndex)
	}

	var b strings.Builder
	totalSubcells := width * 2
	filledSubcells := int(fraction * float64(totalSubcells))
	if filledSubcells < 0 {
		filledSubcells = 0
	}
	if filledSubcells > totalSubcells {
		filledSubcells = totalSubcells
	}
	pulsePos := -1
	if filledSubcells > 0 {
		pulsePos = (frameIndex * 2) % filledSubcells
	}

	for cell := 0; cell < width; cell++ {
		start := cell * 2
		end := start + 2
		subfilled := filledSubcells - start
		if subfilled < 0 {
			subfilled = 0
		}
		if subfilled > 2 {
			subfilled = 2
		}
		if subfilled == 0 {
			b.WriteString(theme.MutedStyle.Render("─"))
			continue
		}
		position := float64(minInt(end, max(1, filledSubcells))) / float64(max(1, filledSubcells))
		base := interpolateSubmitBarColor(submitLiveBarStart, submitLiveBarEnd, position)
		if pulsePos >= 0 {
			center := start + 1
			distance := absInt(center - pulsePos)
			switch {
			case distance == 0:
				base = blendSubmitBarColor(base, submitLivePulse, 0.62)
			case distance <= 2:
				base = blendSubmitBarColor(base, submitLivePulse, 0.34)
			}
		}
		b.WriteString(renderSubmitThinLineCell(base, subfilled))
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func interpolateSubmitBarColor(start, end submitBarColor, t float64) submitBarColor {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return submitBarColor{
		r: int(float64(start.r) + (float64(end.r-start.r) * t)),
		g: int(float64(start.g) + (float64(end.g-start.g) * t)),
		b: int(float64(start.b) + (float64(end.b-start.b) * t)),
	}
}

func blendSubmitBarColor(base, highlight submitBarColor, strength float64) submitBarColor {
	if strength < 0 {
		strength = 0
	}
	if strength > 1 {
		strength = 1
	}
	return submitBarColor{
		r: int(float64(base.r)*(1-strength) + float64(highlight.r)*strength),
		g: int(float64(base.g)*(1-strength) + float64(highlight.g)*strength),
		b: int(float64(base.b)*(1-strength) + float64(highlight.b)*strength),
	}
}

func renderSubmitThinLineCell(color submitBarColor, subfilled int) string {
	if subfilled < 0 {
		subfilled = 0
	}
	if subfilled > 2 {
		subfilled = 2
	}
	glyph := submitLiveThinLineGlyphs[subfilled]
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[39m", color.r, color.g, color.b, glyph)
}

func renderFallbackSubmitBar(theme ui.Theme, width int, fraction float64, frameIndex int) string {
	filled := int(fraction * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	var b strings.Builder
	pulsePos := -1
	if filled > 0 {
		pulsePos = frameIndex % filled
	}
	for i := 0; i < filled; i++ {
		base := interpolateSubmitBarColor(submitLiveBarStart, submitLiveBarEnd, float64(i)/float64(max(1, filled-1)))
		if pulsePos >= 0 && i == pulsePos {
			base = blendSubmitBarColor(base, submitLivePulse, 0.58)
		}
		b.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm─\x1b[39m", base.r, base.g, base.b))
	}
	for i := filled; i < width; i++ {
		b.WriteString(theme.MutedStyle.Render("─"))
	}
	return b.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
