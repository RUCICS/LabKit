package commands

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
	"labkit.local/apps/cli/internal/ui"
)

const (
	liveCursorUp   = "\x1b[4A" // move cursor up 4 lines (one per live block line)
	liveClearLine  = "\r\x1b[2K"
	liveClearToEnd = "\x1b[J"
	liveBarWidth   = 24 // fallback / minimum bar width in characters
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

// submitStatusToStageIndex maps a submission status to the index of the currently active stage.
// 0=packed, 1=queued, 2=running, 3=scoring/done
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
	barWidth   int  // cached at construction; adapts to terminal width
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
			if barWidth > 48 {
				barWidth = 48
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
	_, err := fmt.Fprint(r.out, liveCursorUp+liveClearToEnd)
	return err
}

// renderLines returns the 4 lines that make up the live block (no cursor-movement sequences).
// [0] blank spacing line
// [1] spinner + status + elapsed
// [2] progress bar + percentage
// [3] stage indicators
func (r *submitLiveRenderer) renderLines() [4]string {
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
	filled := int(fraction * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := theme.InfoStyle.Render(strings.Repeat("█", filled)) +
		theme.MutedStyle.Render(strings.Repeat("░", barWidth-filled))
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

	return [4]string{"", line1, line2, line3}
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
