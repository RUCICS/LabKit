package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestLineSpinnerNonTTYFallsBackToStaticLine(t *testing.T) {
	var buf bytes.Buffer
	s := newLineSpinner(&buf, false, "Waiting for authorization…")

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := s.Update("Still waiting…"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Waiting for authorization") {
		t.Fatalf("output = %q, want initial waiting label", got)
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("output = %q, want no ANSI control sequences in non-TTY mode", got)
	}
}

func TestLineSpinnerTTYRewritesLineAndClearsOnStop(t *testing.T) {
	var buf bytes.Buffer
	s := newLineSpinner(&buf, true, "Requesting browser session…")
	s.ticker = time.NewTicker(time.Hour)
	defer s.ticker.Stop()

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	s.mu.Lock()
	s.frameIndex++
	if err := s.renderCurrentFrameLocked(); err != nil {
		s.mu.Unlock()
		t.Fatalf("renderCurrentFrameLocked() error = %v", err)
	}
	s.mu.Unlock()

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "\r\x1b[2K") {
		t.Fatalf("output = %q, want line-clear rewrite sequence", got)
	}
	if !strings.Contains(got, "\x1b[2K") {
		t.Fatalf("output = %q, want clear-on-stop sequence", got)
	}
	plain := stripANSIForTest(got)
	if !strings.Contains(plain, "Requesting browser session") {
		t.Fatalf("plain output = %q, want spinner label", plain)
	}
}
