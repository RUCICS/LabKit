package commands

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFormatSubmitLiveBlockContainsSpinnerStatusAndElapsed(t *testing.T) {
	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 4, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "running")

	lines := r.renderLines()

	combined := strings.Join(lines[:], "\n")
	plain := stripANSIForTest(combined)
	for _, want := range []string{"running", "4s"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderLines() plain = %q, missing %q", plain, want)
		}
	}
}

func TestFormatSubmitLiveBlockContainsProgressBar(t *testing.T) {
	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 2, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "queued")

	lines := r.renderLines()
	barLine := stripANSIForTest(lines[2])
	if !strings.Contains(barLine, "█") && !strings.Contains(barLine, "%") {
		t.Fatalf("bar line = %q, want progress bar characters", barLine)
	}
}

func TestFormatSubmitLiveBlockUsesThinLineGlyphsWhenSupported(t *testing.T) {
	previous := submitLiveSupportsThinLineBlocks
	submitLiveSupportsThinLineBlocks = func() bool { return true }
	defer func() { submitLiveSupportsThinLineBlocks = previous }()

	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 2, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "queued")

	barLine := stripANSIForTest(r.renderLines()[2])
	if !strings.ContainsAny(barLine, "─╴") {
		t.Fatalf("bar line = %q, want thin-line glyphs in hi-res mode", barLine)
	}
}

func TestFormatSubmitLiveBlockProgressBarAnimatesAcrossFrames(t *testing.T) {
	previous := submitLiveSupportsThinLineBlocks
	submitLiveSupportsThinLineBlocks = func() bool { return true }
	defer func() { submitLiveSupportsThinLineBlocks = previous }()

	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 2, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "running")

	first := r.renderLines()[2]
	r.frameIndex++
	second := r.renderLines()[2]

	if first == second {
		t.Fatalf("bar line did not change across frames:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestFormatSubmitLiveBlockUsesGradientANSIStyling(t *testing.T) {
	previous := submitLiveSupportsThinLineBlocks
	submitLiveSupportsThinLineBlocks = func() bool { return true }
	defer func() { submitLiveSupportsThinLineBlocks = previous }()

	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 2, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "running")

	barLine := r.renderLines()[2]
	if strings.Count(barLine, "\x1b[38;2;") < 4 {
		t.Fatalf("bar line = %q, want multiple truecolor foreground segments for continuous gradient", barLine)
	}
}

func TestFormatSubmitLiveBlockPulseHasNoticeableBrightnessContrast(t *testing.T) {
	previous := submitLiveSupportsThinLineBlocks
	submitLiveSupportsThinLineBlocks = func() bool { return true }
	defer func() { submitLiveSupportsThinLineBlocks = previous }()

	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 2, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "running")

	colors := extractTruecolorTriples(r.renderLines()[2])
	if len(colors) == 0 {
		t.Fatal("no truecolor segments found in progress bar")
	}
	minLum, maxLum := 1<<30, 0
	for _, c := range colors {
		lum := c[0] + c[1] + c[2]
		if lum < minLum {
			minLum = lum
		}
		if lum > maxLum {
			maxLum = lum
		}
	}
	if maxLum-minLum < 55 {
		t.Fatalf("brightness contrast too subtle: min=%d max=%d diff=%d", minLum, maxLum, maxLum-minLum)
	}
}

func TestFormatSubmitLiveBlockFallsBackWithoutThinLineSupport(t *testing.T) {
	previous := submitLiveSupportsThinLineBlocks
	submitLiveSupportsThinLineBlocks = func() bool { return false }
	defer func() { submitLiveSupportsThinLineBlocks = previous }()

	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 2, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "running")

	barLine := stripANSIForTest(r.renderLines()[2])
	if strings.ContainsAny(barLine, "╴") {
		t.Fatalf("bar line = %q, want fallback renderer without thin-line partial glyphs", barLine)
	}
}

func TestFormatSubmitLiveBlockStageIndicators(t *testing.T) {
	r := newSubmitLiveRenderer(nil, func() time.Time {
		return time.Date(2026, 4, 1, 12, 0, 1, 0, time.UTC)
	}, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "running")

	lines := r.renderLines()
	stageLine := stripANSIForTest(lines[3])
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
	// Must contain cursor-up-4 to rewrite in place
	if !strings.Contains(out, "\x1b[4A") {
		t.Fatalf("output = %q, want cursor-up-4 escape", out)
	}
	// Must contain clear-to-end-of-screen on Stop
	if !strings.Contains(out, "\x1b[J") {
		t.Fatalf("output = %q, want clear-to-end escape on stop", out)
	}
	// Must contain status text in the output
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

func extractTruecolorTriples(s string) [][3]int {
	re := regexp.MustCompile(`\x1b\[38;2;([0-9]+);([0-9]+);([0-9]+)m`)
	matches := re.FindAllStringSubmatch(s, -1)
	out := make([][3]int, 0, len(matches))
	for _, m := range matches {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		out = append(out, [3]int{r, g, b})
	}
	return out
}
