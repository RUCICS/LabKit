package commands

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"labkit.local/apps/cli/internal/ui"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type lineSpinner struct {
	out        io.Writer
	tty        bool
	label      string
	theme      ui.Theme
	frames     []string
	interval   time.Duration
	ticker     *time.Ticker
	done       chan struct{}
	stopped    chan struct{}
	mu         sync.Mutex
	frameIndex int
	started    bool
}

func newLineSpinner(out io.Writer, tty bool, label string) *lineSpinner {
	if out == nil {
		out = io.Discard
	}
	return &lineSpinner{
		out:      out,
		tty:      tty,
		label:    strings.TrimSpace(label),
		theme:    ui.DefaultTheme(),
		frames:   spinnerFrames,
		interval: 90 * time.Millisecond,
		done:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

func (s *lineSpinner) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	s.started = true
	if !s.tty {
		_, err := fmt.Fprintln(s.out, "  "+s.label)
		return err
	}
	if s.ticker == nil {
		s.ticker = time.NewTicker(s.interval)
	}
	if err := s.renderCurrentFrameLocked(); err != nil {
		return err
	}
	ticker := s.ticker
	go func() {
		defer close(s.stopped)
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				s.frameIndex++
				_ = s.renderCurrentFrameLocked()
				s.mu.Unlock()
			case <-s.done:
				return
			}
		}
	}()
	return nil
}

func (s *lineSpinner) Update(label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.label = strings.TrimSpace(label)
	if !s.tty || !s.started {
		return nil
	}
	return s.renderCurrentFrameLocked()
}

func (s *lineSpinner) Stop() error {
	s.mu.Lock()
	started := s.started
	tty := s.tty
	ticker := s.ticker
	s.started = false
	s.mu.Unlock()
	if !started {
		return nil
	}
	if !tty {
		return nil
	}
	if ticker != nil {
		ticker.Stop()
	}
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	<-s.stopped
	_, err := fmt.Fprint(s.out, liveClearLine)
	return err
}

func (s *lineSpinner) renderCurrentFrameLocked() error {
	frame := s.frames[s.frameIndex%len(s.frames)]
	line := "  " + s.theme.InfoStyle.Render(frame) + " " + s.theme.ValueStyle.Render(s.label)
	_, err := fmt.Fprint(s.out, liveClearLine+line)
	return err
}

func withLineSpinner(deps *Dependencies, label string, run func() error) error {
	deps = normalizeDependencies(deps)
	spinner := newLineSpinner(deps.Out, deps.IsTTY(), label)
	if err := spinner.Start(); err != nil {
		return err
	}
	runErr := run()
	stopErr := spinner.Stop()
	if runErr != nil {
		return runErr
	}
	return stopErr
}
