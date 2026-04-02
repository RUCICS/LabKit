package commands

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
	"labkit.local/apps/cli/internal/config"
	keycrypto "labkit.local/apps/cli/internal/crypto"
	"labkit.local/packages/go/auth"
	"labkit.local/packages/go/manifest"
)

func TestSubmitCommandRejectsFilesNotInManifestBeforePosting(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	if err := writeCLIConfig(t, configDir, keyPath, "", 11); err != nil {
		t.Fatalf("writeCLIConfig() error = %v", err)
	}

	var postCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			postCalls++
			http.Error(w, "unexpected submit call", http.StatusTeapot)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	filePath := filepath.Join(t.TempDir(), "main.c")
	if err := os.WriteFile(filePath, []byte("int main(void) { return 0; }\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", filePath})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "README.md") {
		t.Fatalf("Execute() error = %v, want missing README.md error", err)
	}
	if postCalls != 0 {
		t.Fatalf("postCalls = %d, want 0", postCalls)
	}
}

func TestRenderSubmissionTaskStartNewFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := renderSubmissionTaskStart(&buf, "matrix-mul"); err != nil {
		t.Fatalf("error = %v", err)
	}
	got := buf.String()
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

func TestRenderBoardShowsRankBadgesAndBgFill(t *testing.T) {
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
	plain := stripANSIForTest(buf.String())
	for _, want := range []string{"1ST", "alice", "95.5", "2ND", "bob", "ago"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderBoard() plain = %q, missing %q", plain, want)
		}
	}
	for _, unwanted := range []string{"🥇", "🥈", "🥉"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("renderBoard() plain = %q, want no emoji rank markers", plain)
		}
	}
}

func TestRenderBoardHighlightsCurrentUserRow(t *testing.T) {
	var buf bytes.Buffer
	lab := manifest.PublicManifest{}
	board := boardResponse{
		SelectedMetric: "score",
		Metrics:        []boardMetricResponse{{ID: "score", Name: "score"}},
		Rows: []boardRowResponse{
			{Rank: 1, Nickname: "alice", Scores: []boardScoreResponse{{MetricID: "score", Value: 95.5}}, UpdatedAt: time.Now().Add(-2 * time.Hour)},
			{Rank: 4, Nickname: "huanc", CurrentUser: true, Scores: []boardScoreResponse{{MetricID: "score", Value: 88.3}}, UpdatedAt: time.Now().Add(-1 * time.Hour)},
		},
	}
	if err := renderBoard(&buf, lab, board); err != nil {
		t.Fatalf("error = %v", err)
	}
	got := buf.String()
	for _, want := range []string{"you (huanc)", "88.3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderBoard() = %q, missing %q", got, want)
		}
	}
	if strings.Contains(stripANSIForTest(got), "▏") {
		t.Fatalf("renderBoard() plain = %q, want no dedicated current-user marker", stripANSIForTest(got))
	}
}

func TestRenderBoardKeepsBadgeForCurrentUserTopThree(t *testing.T) {
	var buf bytes.Buffer
	lab := manifest.PublicManifest{}
	board := boardResponse{
		SelectedMetric: "score",
		Metrics:        []boardMetricResponse{{ID: "score", Name: "score"}},
		Rows: []boardRowResponse{
			{Rank: 1, Nickname: "alice", CurrentUser: true, Scores: []boardScoreResponse{{MetricID: "score", Value: 95.5}}, UpdatedAt: time.Now().Add(-2 * time.Hour)},
			{Rank: 2, Nickname: "bob", Scores: []boardScoreResponse{{MetricID: "score", Value: 80.0}}, UpdatedAt: time.Now().Add(-1 * time.Hour)},
		},
	}
	if err := renderBoard(&buf, lab, board); err != nil {
		t.Fatalf("error = %v", err)
	}

	plain := stripANSIForTest(buf.String())
	if !strings.Contains(plain, "1ST") {
		t.Fatalf("renderBoard() plain = %q, want rank badge retained for top-three current user", plain)
	}
	if strings.Contains(plain, "▏") {
		t.Fatalf("renderBoard() plain = %q, want no dedicated current-user marker", plain)
	}
	if strings.Contains(plain, "🥇") {
		t.Fatalf("renderBoard() plain = %q, want emoji medal removed", plain)
	}

	lines := strings.Split(plain, "\n")
	var aliceLine, bobLine string
	for _, line := range lines {
		switch {
		case strings.Contains(line, "you (alice)"):
			aliceLine = line
		case strings.Contains(line, "bob"):
			bobLine = line
		}
	}
	if aliceLine == "" || bobLine == "" {
		t.Fatalf("renderBoard() plain = %q, want both alice and bob rows", plain)
	}
	alicePrefixWidth := runewidth.StringWidth(strings.SplitN(aliceLine, "you (alice)", 2)[0])
	bobPrefixWidth := runewidth.StringWidth(strings.SplitN(bobLine, "bob", 2)[0])
	if alicePrefixWidth != bobPrefixWidth {
		t.Fatalf("nickname columns misaligned:\nalice: %q\nbob:   %q", aliceLine, bobLine)
	}
}

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

func TestRenderHistoryShowsFullSubmissionID(t *testing.T) {
	var buf bytes.Buffer
	submissionID := "11111111-1111-7111-8111-111111111111"
	history := historyResponse{
		Submissions: []historyItemResponse{
			{ID: submissionID, Status: "completed", Verdict: "scored", CreatedAt: time.Now().Add(-1 * time.Hour)},
		},
	}
	if err := renderHistory(&buf, history); err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(buf.String(), submissionID) {
		t.Fatalf("renderHistory() = %q, want full submission id", buf.String())
	}
}

func TestSubmitCommandSignsMultipartSubmission(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
	mustWriteFile(t, readmePath, []byte("# sorting\n"))

	var stdout bytes.Buffer
	var captured submitCapture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if maybeServeSubmitPrecheck(t, w, r, pub) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			captured = captureSubmitRequest(t, r, pub)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "11111111-1111-7111-8111-111111111111",
				"status":       "queued",
				"artifact_key": "sorting/7/a.tar.gz",
				"content_hash": "hash",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", "--no-wait", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if captured.method != http.MethodPost {
		t.Fatalf("method = %q, want POST", captured.method)
	}
	if captured.path != "/api/labs/sorting/submit" {
		t.Fatalf("path = %q, want /api/labs/sorting/submit", captured.path)
	}
	if captured.keyFingerprint == "" {
		t.Fatal("keyFingerprint was empty")
	}
	if captured.timestamp.IsZero() {
		t.Fatal("timestamp was empty")
	}
	if captured.nonce == "" {
		t.Fatal("nonce was empty")
	}
	expectedPayload := auth.NewPayload("sorting", captured.timestamp, captured.nonce, []string{"main.c", "README.md"}).WithContentHash(captured.contentHash)
	signingBytes, err := expectedPayload.SigningBytes()
	if err != nil {
		t.Fatalf("SigningBytes() error = %v", err)
	}
	if !ed25519.Verify(pub, signingBytes, captured.signature) {
		t.Fatal("signature did not verify")
	}
	if captured.files["main.c"] != "int main(void) { return 0; }\n" {
		t.Fatalf("main.c = %q, want source contents", captured.files["main.c"])
	}
	if captured.files["README.md"] != "# sorting\n" {
		t.Fatalf("README.md = %q, want README contents", captured.files["README.md"])
	}
	if !strings.Contains(stdout.String(), "● Submitted  (detached)") {
		t.Fatalf("stdout = %q, want compact detached summary", stdout.String())
	}
	if !strings.Contains(stdout.String(), "  id  11111111-1111-7111-8111-111111111111") {
		t.Fatalf("stdout = %q, want compact id line", stdout.String())
	}
	if strings.Contains(stdout.String(), "Submission result") {
		t.Fatalf("stdout = %q, want no card-style result heading", stdout.String())
	}
	for _, bad := range []string{"\r", "\x1b[2K"} {
		if strings.Contains(stdout.String(), bad) {
			t.Fatalf("stdout = %q, want non-TTY fallback without live control sequence %q", stdout.String(), bad)
		}
	}
}

func TestSubmitCommandWaitsForFinalStatusAndRendersScores(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
	mustWriteFile(t, readmePath, []byte("# sorting\n"))

	var stdout bytes.Buffer
	var detailCalls int
	submissionID := "11111111-1111-7111-8111-111111111111"
	finishedAt := time.Date(2026, 3, 31, 12, 10, 0, 0, time.UTC)
	createdAt := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if maybeServeSubmitPrecheck(t, w, r, pub) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
					{ID: "latency", Name: "Latency", Sort: manifest.MetricSortAsc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			captureSubmitRequest(t, r, pub)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     submissionID,
				"status": "queued",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submissions/"+submissionID:
			detailCalls++
			payload := map[string]any{
				"id":          submissionID,
				"lab_id":      "sorting",
				"status":      "queued",
				"verdict":     "",
				"message":     "",
				"detail":      nil,
				"scores":      []map[string]any{},
				"created_at":  createdAt.Format(time.RFC3339),
				"finished_at": nil,
			}
			switch detailCalls {
			case 1:
				payload["status"] = "queued"
			case 2:
				payload["status"] = "running"
			default:
				payload["status"] = "done"
				payload["verdict"] = "scored"
				payload["message"] = "all good"
				payload["detail"] = map[string]any{"format": "markdown", "content": "great"}
				payload["scores"] = []map[string]any{
					{"metric_id": "throughput", "value": 88},
					{"metric_id": "latency", "value": 35},
				}
				payload["finished_at"] = finishedAt.Format(time.RFC3339)
			}
			_ = json.NewEncoder(w).Encode(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if detailCalls < 3 {
		t.Fatalf("detailCalls = %d, want at least 3", detailCalls)
	}
	for _, want := range []string{
		"● Submitting  sorting",
		"state   queued -> running -> done",
		"✓ Submitted",
		"╷",
		"│",
		"╵",
		"PASSED   10m0s",
		submissionID,
		"Scores",
		"Throughput   88",
		"Latency      35",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, bad := range []string{
		"steps   prepared -> uploaded -> waiting",
		"Submission update",
		"Submission result",
		"detail (markdown):",
		"• ",
		"\n  queued\n",
		"\n  running\n",
		"\n  done\n",
		"\r",
		"\x1b[2K",
	} {
		if strings.Contains(stdout.String(), bad) {
			t.Fatalf("stdout = %q, want no loose progress fragment %q", stdout.String(), bad)
		}
	}
	if got := strings.Count(stdout.String(), submissionID); got != 1 {
		t.Fatalf("stdout = %q, submission id count = %d, want 1", stdout.String(), got)
	}
}

func TestSubmitCommandShowsInitialFeedbackBeforeServerAcceptsSubmission(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
	mustWriteFile(t, readmePath, []byte("# sorting\n"))

	var stdout bytes.Buffer
	postStarted := make(chan struct{})
	releasePost := make(chan struct{})
	releasePostOnce := false
	release := func() {
		if releasePostOnce {
			return
		}
		releasePostOnce = true
		close(releasePost)
	}
	defer release()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if maybeServeSubmitPrecheck(t, w, r, pub) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			captureSubmitRequest(t, r, pub)
			close(postStarted)
			<-releasePost
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "11111111-1111-7111-8111-111111111111",
				"status": "queued",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", "--no-wait", mainPath, readmePath})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	select {
	case <-postStarted:
	case <-time.After(time.Second):
		t.Fatal("submit POST did not start")
	}

	deadline := time.Now().Add(time.Second)
	for !strings.Contains(stdout.String(), "● Submitting  sorting") {
		if time.Now().After(deadline) {
			release()
			t.Fatalf("stdout = %q, want initial feedback before submit request completes", stdout.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	release()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("submit command did not finish after server response")
	}
}

func TestSubmitCommandDuplicateTTYEnterContinues(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mainContent := []byte("int main(void) { return 0; }\n")
	readmeContent := []byte("# sorting\n")
	mustWriteFile(t, mainPath, mainContent)
	mustWriteFile(t, readmePath, readmeContent)
	archiveHash, err := submissionArchiveHash([]submissionFile{
		{Name: "main.c", Content: mainContent},
		{Name: "README.md", Content: readmeContent},
	})
	if err != nil {
		t.Fatalf("submissionArchiveHash() error = %v", err)
	}

	oldSubmitOutputIsTTY := submitOutputIsTTY
	submitOutputIsTTY = func(io.Writer) bool { return true }
	t.Cleanup(func() { submitOutputIsTTY = oldSubmitOutputIsTTY })

	var stdout bytes.Buffer
	var submitCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submit/precheck":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/submit/precheck", nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"quota": map[string]any{"daily": 3, "used": 1, "left": 2, "reset_hint": "00:00 Asia/Shanghai"},
				"latest_submission": map[string]any{
					"content_hash": archiveHash,
					"created_at":   "2026-03-31T11:48:00Z",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			submitCalls++
			captureSubmitRequest(t, r, pub)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "11111111-1111-7111-8111-111111111111",
				"status": "queued",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		In:         strings.NewReader("\n"),
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", "--no-wait", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if submitCalls != 1 {
		t.Fatalf("submitCalls = %d, want 1", submitCalls)
	}
	plain := stripANSIForTest(stdout.String())
	for _, want := range []string{
		"Matches your latest submission",
		"Press Enter to submit anyway, or n to cancel",
		"Submitted",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("stdout = %q, want %q", plain, want)
		}
	}
}

func TestSubmitCommandDuplicateTTYNoCancels(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mainContent := []byte("int main(void) { return 0; }\n")
	readmeContent := []byte("# sorting\n")
	mustWriteFile(t, mainPath, mainContent)
	mustWriteFile(t, readmePath, readmeContent)
	archiveHash, err := submissionArchiveHash([]submissionFile{
		{Name: "main.c", Content: mainContent},
		{Name: "README.md", Content: readmeContent},
	})
	if err != nil {
		t.Fatalf("submissionArchiveHash() error = %v", err)
	}

	oldSubmitOutputIsTTY := submitOutputIsTTY
	submitOutputIsTTY = func(io.Writer) bool { return true }
	t.Cleanup(func() { submitOutputIsTTY = oldSubmitOutputIsTTY })

	var stdout bytes.Buffer
	var submitCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submit/precheck":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/submit/precheck", nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"latest_submission": map[string]any{
					"content_hash": archiveHash,
					"created_at":   "2026-03-31T11:48:00Z",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			submitCalls++
			http.Error(w, "unexpected submit", http.StatusTeapot)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		In:         strings.NewReader("no\n"),
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", "--no-wait", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if submitCalls != 0 {
		t.Fatalf("submitCalls = %d, want 0", submitCalls)
	}
	plain := stripANSIForTest(stdout.String())
	if !strings.Contains(plain, "Submission cancelled") {
		t.Fatalf("stdout = %q, want cancellation message", plain)
	}
}

func TestSubmitCommandDuplicateNonTTYWarnsAndContinues(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mainContent := []byte("int main(void) { return 0; }\n")
	readmeContent := []byte("# sorting\n")
	mustWriteFile(t, mainPath, mainContent)
	mustWriteFile(t, readmePath, readmeContent)
	archiveHash, err := submissionArchiveHash([]submissionFile{
		{Name: "main.c", Content: mainContent},
		{Name: "README.md", Content: readmeContent},
	})
	if err != nil {
		t.Fatalf("submissionArchiveHash() error = %v", err)
	}

	var stdout bytes.Buffer
	var submitCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submit/precheck":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/submit/precheck", nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"latest_submission": map[string]any{
					"content_hash": archiveHash,
					"created_at":   "2026-03-31T11:48:00Z",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			submitCalls++
			captureSubmitRequest(t, r, pub)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "11111111-1111-7111-8111-111111111111",
				"status": "queued",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", "--no-wait", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if submitCalls != 1 {
		t.Fatalf("submitCalls = %d, want 1", submitCalls)
	}
	plain := stripANSIForTest(stdout.String())
	if !strings.Contains(plain, "Matches your latest submission") {
		t.Fatalf("stdout = %q, want duplicate warning", plain)
	}
}

func TestSubmitCommandContinuesWhenPrecheckFails(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
	mustWriteFile(t, readmePath, []byte("# sorting\n"))

	var stdout bytes.Buffer
	var submitCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submit/precheck":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/submit/precheck", nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			http.Error(w, "temporary precheck failure", http.StatusInternalServerError)
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			submitCalls++
			captureSubmitRequest(t, r, pub)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "11111111-1111-7111-8111-111111111111",
				"status": "queued",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", "--no-wait", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if submitCalls != 1 {
		t.Fatalf("submitCalls = %d, want 1", submitCalls)
	}
	if !strings.Contains(stripANSIForTest(stdout.String()), "Precheck unavailable, continuing submit") {
		t.Fatalf("stdout = %q, want fallback warning", stripANSIForTest(stdout.String()))
	}
}

func TestSubmitCommandDetachAndNoWaitSkipPolling(t *testing.T) {
	for _, flag := range []string{"--detach", "--no-wait"} {
		t.Run(flag, func(t *testing.T) {
			configDir := t.TempDir()
			keyPath := filepath.Join(configDir, "id_ed25519")
			pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

			mainPath := filepath.Join(t.TempDir(), "main.c")
			readmePath := filepath.Join(t.TempDir(), "README.md")
			mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
			mustWriteFile(t, readmePath, []byte("# sorting\n"))

			var stdout bytes.Buffer
			var getCalls int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if maybeServeSubmitPrecheck(t, w, r, pub) {
					return
				}
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
					writeLabManifest(t, w, manifest.Manifest{
						Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
						Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
						Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
						Quota:  manifest.QuotaSection{Daily: 3},
						Metrics: []manifest.MetricSection{
							{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
						},
						Board:    manifest.BoardSection{RankBy: "throughput"},
						Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
					})
				case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
					captureSubmitRequest(t, r, pub)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"id":     "11111111-1111-7111-8111-111111111111",
						"status": "queued",
					})
				case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/labs/sorting/submissions/"):
					getCalls++
					http.Error(w, "unexpected polling", http.StatusTeapot)
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			t.Setenv("LABKIT_SERVER_URL", srv.URL)

			if err := config.Write(configDir, config.Config{
				ServerURL: "",
				KeyPath:   keyPath,
				KeyID:     11,
			}); err != nil {
				t.Fatalf("config.Write() error = %v", err)
			}
			if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
				t.Fatalf("WritePrivateKey() error = %v", err)
			}

			deps := &Dependencies{
				ConfigDir:  configDir,
				HTTPClient: srv.Client(),
				Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
				Out:        &stdout,
				Err:        io.Discard,
			}

			cmd := NewRootCommand(deps)
			cmd.SetArgs([]string{"--lab", "sorting", "submit", flag, mainPath, readmePath})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if getCalls != 0 {
				t.Fatalf("getCalls = %d, want 0", getCalls)
			}
			for _, want := range []string{"● Submitted  (detached)", "  id  11111111-1111-7111-8111-111111111111"} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want %q", stdout.String(), want)
				}
			}
			if strings.Contains(stdout.String(), "Submission result") {
				t.Fatalf("stdout = %q, want compact detached summary", stdout.String())
			}
			for _, bad := range []string{"\r", "\x1b[2K"} {
				if strings.Contains(stdout.String(), bad) {
					t.Fatalf("stdout = %q, want non-TTY fallback without live control sequence %q", stdout.String(), bad)
				}
			}
		})
	}
}

func TestSubmitCommandRendersQuotaSummary(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
	mustWriteFile(t, readmePath, []byte("# sorting\n"))

	var stdout bytes.Buffer
	submissionID := "11111111-1111-7111-8111-111111111111"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submit/precheck":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/submit/precheck", nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			captureSubmitRequest(t, r, pub)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     submissionID,
				"status": "queued",
				"quota":  map[string]any{"daily": 3, "used": 1, "left": 2, "reset_hint": "00:00 Asia/Shanghai"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submissions/"+submissionID:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          submissionID,
				"lab_id":      "sorting",
				"status":      "done",
				"verdict":     "scored",
				"message":     "all good",
				"created_at":  "2026-03-31T12:00:00Z",
				"finished_at": "2026-03-31T12:10:00Z",
				"quota":       map[string]any{"daily": 3, "used": 1, "left": 2, "reset_hint": "00:00 Asia/Shanghai"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stripANSIForTest(stdout.String()), "Quota  2 left today · 1/3 used") {
		t.Fatalf("stdout = %q, want quota summary", stripANSIForTest(stdout.String()))
	}
}

func TestSubmitCommandRendersFreeVerdictQuotaSummary(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
	mustWriteFile(t, readmePath, []byte("# sorting\n"))

	var stdout bytes.Buffer
	submissionID := "11111111-1111-7111-8111-111111111111"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submit/precheck":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/submit/precheck", nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			captureSubmitRequest(t, r, pub)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     submissionID,
				"status": "queued",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submissions/"+submissionID:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          submissionID,
				"lab_id":      "sorting",
				"status":      "done",
				"verdict":     "build_failed",
				"message":     "compile failed",
				"quota_state": "free",
				"created_at":  "2026-03-31T12:00:00Z",
				"finished_at": "2026-03-31T12:02:00Z",
				"quota":       map[string]any{"daily": 3, "used": 1, "left": 2, "reset_hint": "00:00 Asia/Shanghai"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stripANSIForTest(stdout.String()), "Quota  2 left today · build_failed is free") {
		t.Fatalf("stdout = %q, want free quota summary", stripANSIForTest(stdout.String()))
	}
}

func TestSubmitCommandRendersFailureMessageAndDetailInResult(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
	mustWriteFile(t, readmePath, []byte("# sorting\n"))

	var stdout bytes.Buffer
	var detailCalls int
	submissionID := "11111111-1111-7111-8111-111111111111"
	finishedAt := time.Date(2026, 3, 31, 12, 2, 30, 0, time.UTC)
	createdAt := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if maybeServeSubmitPrecheck(t, w, r, pub) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			captureSubmitRequest(t, r, pub)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     submissionID,
				"status": "queued",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submissions/"+submissionID:
			detailCalls++
			payload := map[string]any{
				"id":          submissionID,
				"lab_id":      "sorting",
				"status":      "running",
				"verdict":     "",
				"message":     "",
				"detail":      nil,
				"scores":      []map[string]any{},
				"created_at":  createdAt.Format(time.RFC3339),
				"finished_at": nil,
			}
			if detailCalls > 1 {
				payload["status"] = "failed"
				payload["verdict"] = "compile_error"
				payload["message"] = "Compilation failed"
				payload["detail"] = map[string]any{
					"format":  "markdown",
					"content": "main.c:1:1: error: expected ';'",
				}
				payload["finished_at"] = finishedAt.Format(time.RFC3339)
			}
			_ = json.NewEncoder(w).Encode(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", mainPath, readmePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{
		"● Submitting  sorting",
		"state   queued -> running -> failed",
		"✗ Submitted",
		"╷",
		"FAILED   2m30s",
		submissionID,
		"detail (markdown):",
		"main.c:1:1: error: expected ';'",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, bad := range []string{"steps   prepared -> uploaded -> waiting", "Submission update", "Submission result", "\n  running\n", "\n  failed\n", "\r", "\x1b[2K"} {
		if strings.Contains(stdout.String(), bad) {
			t.Fatalf("stdout = %q, want compact failure report without loose fragment %q", stdout.String(), bad)
		}
	}
}

func TestSubmitCommandInterruptsWaitingWithHint(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)

	mainPath := filepath.Join(t.TempDir(), "main.c")
	readmePath := filepath.Join(t.TempDir(), "README.md")
	mustWriteFile(t, mainPath, []byte("int main(void) { return 0; }\n"))
	mustWriteFile(t, readmePath, []byte("# sorting\n"))

	var stdout bytes.Buffer
	var detailCalls int
	var cancelWait context.CancelFunc
	oldInterruptContext := submitInterruptContext
	t.Cleanup(func() { submitInterruptContext = oldInterruptContext })
	submitInterruptContext = func(parent context.Context) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(parent)
		cancelWait = cancel
		return ctx, cancel
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if maybeServeSubmitPrecheck(t, w, r, pub) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c", "README.md"}, MaxSize: "1MB"},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/labs/sorting/submit":
			captureSubmitRequest(t, r, pub)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "11111111-1111-7111-8111-111111111111",
				"status": "queued",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submissions/11111111-1111-7111-8111-111111111111":
			detailCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "11111111-1111-7111-8111-111111111111",
				"lab_id":     "sorting",
				"status":     "queued",
				"verdict":    "",
				"message":    "",
				"detail":     nil,
				"scores":     []map[string]any{},
				"created_at": time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			})
			if cancelWait != nil {
				cancelWait()
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "submit", mainPath, readmePath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want interrupt error")
	}
	if !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("Execute() error = %v, want interrupt hint", err)
	}
	if detailCalls != 1 {
		t.Fatalf("detailCalls = %d, want 1", detailCalls)
	}
	if !strings.Contains(stdout.String(), "Waiting interrupted") {
		t.Fatalf("stdout = %q, want interrupt hint", stdout.String())
	}
	if !strings.Contains(stdout.String(), "history") && !strings.Contains(stdout.String(), "board") {
		t.Fatalf("stdout = %q, want follow-up hint", stdout.String())
	}
}

func TestBoardCommandDisplaysRowsByRequestedMetric(t *testing.T) {
	var stdout bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c"}},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
					{ID: "latency", Name: "Latency", Sort: manifest.MetricSortAsc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput", Pick: true},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/board":
			if got := r.URL.Query().Get("by"); got != "latency" {
				t.Fatalf("by = %q, want latency", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lab_id":          "sorting",
				"selected_metric": "latency",
				"metrics": []map[string]any{
					{"id": "throughput", "name": "Throughput", "sort": "desc"},
					{"id": "latency", "name": "Latency", "sort": "asc", "selected": true},
				},
				"rows": []map[string]any{
					{"rank": 1, "nickname": "Bob", "track": "latency", "scores": []map[string]any{{"metric_id": "throughput", "value": 88}, {"metric_id": "latency", "value": 35}}, "updated_at": "2026-03-31T10:00:00Z"},
					{"rank": 2, "nickname": "Ada", "track": "throughput", "scores": []map[string]any{{"metric_id": "throughput", "value": 92}, {"metric_id": "latency", "value": 50}}, "updated_at": "2026-03-31T11:00:00Z"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	deps := &Dependencies{
		ServerURLOverride: srv.URL,
		HTTPClient:        srv.Client(),
		Out:               &stdout,
		Err:               io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "board", "--by", "latency"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "Leaderboard") {
		t.Fatalf("stdout = %q, want structured board heading", stdout.String())
	}
	for _, want := range []string{"sorted by latency", "Throughput", "Latency", "1ST", "2ND", "ago"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if strings.Index(stdout.String(), "Bob") > strings.Index(stdout.String(), "Ada") {
		t.Fatalf("stdout row order = %q, want Bob before Ada", stdout.String())
	}
	if strings.Contains(stdout.String(), "Selected metric:") {
		t.Fatalf("stdout = %q, want new tab-based metric header", stdout.String())
	}
}

func TestBoardCommandUsesSignedRequestAndHighlightsCurrentUser(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	var stdout bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c"}},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
					{ID: "latency", Name: "Latency", Sort: manifest.MetricSortAsc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput", Pick: true},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/board":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/board?by=latency", nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lab_id":          "sorting",
				"selected_metric": "latency",
				"metrics": []map[string]any{
					{"id": "throughput", "name": "Throughput", "sort": "desc"},
					{"id": "latency", "name": "Latency", "sort": "asc", "selected": true},
				},
				"rows": []map[string]any{
					{"rank": 1, "nickname": "Bob", "track": "latency", "scores": []map[string]any{{"metric_id": "throughput", "value": 88}, {"metric_id": "latency", "value": 35}}, "updated_at": "2026-03-31T10:00:00Z", "current_user": true},
					{"rank": 2, "nickname": "Ada", "track": "throughput", "scores": []map[string]any{{"metric_id": "throughput", "value": 92}, {"metric_id": "latency", "value": 50}}, "updated_at": "2026-03-31T11:00:00Z"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "board", "--by", "latency"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{"you (Bob)", "sorted by latency"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if strings.Contains(stripANSIForTest(stdout.String()), "▏") {
		t.Fatalf("stdout = %q, want no dedicated current-user marker", stdout.String())
	}
}

func TestBoardCommandRendersQuotaSummaryForSignedUser(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	var stdout bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c"}},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/board":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/board", nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lab_id":          "sorting",
				"selected_metric": "throughput",
				"metrics":         []map[string]any{{"id": "throughput", "name": "Throughput", "sort": "desc", "selected": true}},
				"rows":            []map[string]any{{"rank": 1, "nickname": "Bob", "scores": []map[string]any{{"metric_id": "throughput", "value": 88}}, "updated_at": "2026-03-31T10:00:00Z", "current_user": true}},
				"quota":           map[string]any{"daily": 3, "used": 1, "left": 2, "reset_hint": "00:00 Asia/Shanghai"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "board"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stripANSIForTest(stdout.String()), "Quota  2 left today · 1/3 used") {
		t.Fatalf("stdout = %q, want quota summary", stripANSIForTest(stdout.String()))
	}
}

func TestBoardCommandOmitsQuotaSummaryWhenAnonymous(t *testing.T) {
	var stdout bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c"}},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/board":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lab_id":          "sorting",
				"selected_metric": "throughput",
				"metrics":         []map[string]any{{"id": "throughput", "name": "Throughput", "sort": "desc", "selected": true}},
				"rows":            []map[string]any{{"rank": 1, "nickname": "Bob", "scores": []map[string]any{{"metric_id": "throughput", "value": 88}}, "updated_at": "2026-03-31T10:00:00Z"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	deps := &Dependencies{
		ServerURLOverride: srv.URL,
		HTTPClient:        srv.Client(),
		Out:               &stdout,
		Err:               io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "board"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if strings.Contains(stripANSIForTest(stdout.String()), "Quota  ") {
		t.Fatalf("stdout = %q, want no quota summary for anonymous board", stripANSIForTest(stdout.String()))
	}
}

func TestHistoryCommandRendersSubmissionList(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	_, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	var stdout bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/history":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/history", nil, priv.Public().(ed25519.PublicKey)); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"submissions": []map[string]any{
					{
						"id":          "22222222-2222-7222-8222-222222222222",
						"status":      "queued",
						"verdict":     "",
						"message":     "",
						"created_at":  "2026-03-31T12:00:00Z",
						"finished_at": nil,
					},
					{
						"id":          "11111111-1111-7111-8111-111111111111",
						"status":      "done",
						"verdict":     "scored",
						"message":     "all good",
						"created_at":  "2026-03-31T11:00:00Z",
						"finished_at": "2026-03-31T11:10:00Z",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "history"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "Submission history") {
		t.Fatalf("stdout = %q, want structured history heading", stdout.String())
	}
	for _, want := range []string{"queued", "scored", "ago", "2 submissions", "22222222-2", "11111111-1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestHistoryCommandRendersQuotaSummary(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	_, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	var stdout bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/history":
			if err := verifySignedRequest(t, r, "/api/labs/sorting/history", nil, priv.Public().(ed25519.PublicKey)); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"submissions": []map[string]any{
					{
						"id":         "22222222-2222-7222-8222-222222222222",
						"status":     "queued",
						"created_at": "2026-03-31T12:00:00Z",
					},
				},
				"quota": map[string]any{"daily": 3, "used": 2, "left": 1, "reset_hint": "00:00 Asia/Shanghai"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "history"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stripANSIForTest(stdout.String()), "Quota  1 left today · 2/3 used") {
		t.Fatalf("stdout = %q, want quota summary", stripANSIForTest(stdout.String()))
	}
}

func TestHistoryCommandWithSubmissionIDShowsDetail(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	var stdout bytes.Buffer
	submissionID := "11111111-1111-7111-8111-111111111111"
	finishedAt := time.Date(2026, 3, 31, 11, 10, 0, 0, time.UTC)
	createdAt := time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c"}},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput"},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/submissions/"+submissionID:
			if err := verifySignedRequest(t, r, "/api/labs/sorting/submissions/"+submissionID, nil, pub); err != nil {
				t.Fatalf("verifySignedRequest() error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          submissionID,
				"lab_id":      "sorting",
				"status":      "done",
				"verdict":     "scored",
				"message":     "all good",
				"detail":      map[string]any{"format": "markdown", "content": "great"},
				"scores":      []map[string]any{{"metric_id": "throughput", "value": 88}},
				"created_at":  createdAt.Format(time.RFC3339),
				"finished_at": finishedAt.Format(time.RFC3339),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "history", submissionID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, want := range []string{"Submission details", submissionID, "Throughput", "88", "great"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestNickCommandUpdatesNickname(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	_, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	var stdout bytes.Buffer
	var captured updateCapture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/labs/sorting/nickname":
			captured = captureJSONUpdateRequest(t, r, "/api/labs/sorting/nickname", priv.Public().(ed25519.PublicKey))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lab_id":   "sorting",
				"nickname": "Cat",
				"track":    "throughput",
				"pick":     true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "nick", "Cat"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if captured.method != http.MethodPut {
		t.Fatalf("method = %q, want PUT", captured.method)
	}
	if captured.path != "/api/labs/sorting/nickname" {
		t.Fatalf("path = %q, want nickname path", captured.path)
	}
	if captured.body["nickname"] != "Cat" {
		t.Fatalf("nickname body = %#v, want Cat", captured.body["nickname"])
	}
	if !strings.Contains(stdout.String(), "Cat") {
		t.Fatalf("stdout = %q, want nickname confirmation", stdout.String())
	}
}

func TestTrackCommandUpdatesTrack(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	_, priv := mustWriteConfigAndKey(t, configDir, keyPath, "", 11)
	if err := keycrypto.WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}

	var stdout bytes.Buffer
	var captured updateCapture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			writeLabManifest(t, w, manifest.Manifest{
				Lab:    manifest.LabSection{ID: "sorting", Name: "Sorting"},
				Submit: manifest.SubmitSection{Files: []string{"main.c"}},
				Eval:   manifest.EvalSection{Image: "ghcr.io/labkit/sorting:1"},
				Quota:  manifest.QuotaSection{Daily: 3},
				Metrics: []manifest.MetricSection{
					{ID: "throughput", Name: "Throughput", Sort: manifest.MetricSortDesc},
					{ID: "latency", Name: "Latency", Sort: manifest.MetricSortAsc},
				},
				Board:    manifest.BoardSection{RankBy: "throughput", Pick: true},
				Schedule: manifest.ScheduleSection{Open: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Close: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/labs/sorting/track":
			captured = captureJSONUpdateRequest(t, r, "/api/labs/sorting/track", priv.Public().(ed25519.PublicKey))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lab_id":   "sorting",
				"nickname": "Cat",
				"track":    "latency",
				"pick":     true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"--lab", "sorting", "track", "latency"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if captured.method != http.MethodPut {
		t.Fatalf("method = %q, want PUT", captured.method)
	}
	if captured.path != "/api/labs/sorting/track" {
		t.Fatalf("path = %q, want track path", captured.path)
	}
	if captured.body["track"] != "latency" {
		t.Fatalf("track body = %#v, want latency", captured.body["track"])
	}
	if !strings.Contains(stdout.String(), "latency") {
		t.Fatalf("stdout = %q, want track confirmation", stdout.String())
	}
}

type submitCapture struct {
	method         string
	path           string
	keyFingerprint string
	timestamp      time.Time
	nonce          string
	signature      []byte
	contentHash    string
	files          map[string]string
}

type updateCapture struct {
	method string
	path   string
	body   map[string]any
}

func writeCLIConfig(t *testing.T, configDir, keyPath, serverURL string, keyID int64) error {
	t.Helper()
	return config.Write(configDir, config.Config{
		ServerURL: serverURL,
		KeyPath:   keyPath,
		KeyID:     keyID,
	})
}

func mustWriteConfigAndKey(t *testing.T, configDir, keyPath, serverURL string, keyID int64) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pair, err := keycrypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	if err := writeCLIConfig(t, configDir, keyPath, serverURL, keyID); err != nil {
		t.Fatalf("writeCLIConfig() error = %v", err)
	}
	return pair.Public, pair.Private
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func writeLabManifest(t *testing.T, w http.ResponseWriter, m manifest.Manifest) {
	t.Helper()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":                  m.Lab.ID,
		"name":                m.Lab.Name,
		"manifest":            m.Public(),
		"manifest_updated_at": time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
	})
}

func captureSubmitRequest(t *testing.T, r *http.Request, pub ed25519.PublicKey) submitCapture {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if err := r.Body.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	parsedFiles := make([]submissionFile, 0)
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType() error = %v", err)
	}
	if !strings.EqualFold(mediaType, "multipart/form-data") {
		t.Fatalf("Content-Type = %q, want multipart/form-data", mediaType)
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	files := make(map[string]string)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("NextPart() error = %v", err)
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("ReadAll(part) error = %v", err)
		}
		files[part.FileName()] = string(content)
		parsedFiles = append(parsedFiles, submissionFile{Name: part.FileName(), Content: append([]byte(nil), content...)})
	}

	timestamp, err := time.Parse(time.RFC3339Nano, r.Header.Get("X-LabKit-Timestamp"))
	if err != nil {
		t.Fatalf("Parse timestamp: %v", err)
	}
	nonce := r.Header.Get("X-LabKit-Nonce")
	if nonce == "" {
		t.Fatal("nonce was empty")
	}
	if got := r.Header.Get("X-LabKit-Key-ID"); got != "" {
		t.Fatalf("unexpected X-LabKit-Key-ID header = %q", got)
	}
	wantFingerprint, err := keycrypto.PublicKeyFingerprint(pub)
	if err != nil {
		t.Fatalf("PublicKeyFingerprint() error = %v", err)
	}
	if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-Fingerprint")); got != wantFingerprint {
		t.Fatalf("X-LabKit-Key-Fingerprint = %q, want %q", got, wantFingerprint)
	}
	signature, err := base64.StdEncoding.DecodeString(r.Header.Get("X-LabKit-Signature"))
	if err != nil {
		t.Fatalf("DecodeString(signature): %v", err)
	}
	contentHash, err := submissionArchiveHash(parsedFiles)
	if err != nil {
		t.Fatalf("submissionArchiveHash() error = %v", err)
	}
	payload := auth.NewPayload("sorting", timestamp, nonce, submissionFileNames(parsedFiles)).WithContentHash(contentHash)
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		t.Fatalf("SigningBytes() error = %v", err)
	}
	if !ed25519.Verify(pub, signingBytes, signature) {
		t.Fatal("submit signature did not verify")
	}
	return submitCapture{
		method:         r.Method,
		path:           r.URL.Path,
		keyFingerprint: wantFingerprint,
		timestamp:      timestamp,
		nonce:          nonce,
		signature:      signature,
		contentHash:    contentHash,
		files:          files,
	}
}

func captureJSONUpdateRequest(t *testing.T, r *http.Request, path string, pub ed25519.PublicKey) updateCapture {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if err := r.Body.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	timestamp, err := time.Parse(time.RFC3339Nano, r.Header.Get("X-LabKit-Timestamp"))
	if err != nil {
		t.Fatalf("Parse timestamp: %v", err)
	}
	if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-ID")); got != "" {
		t.Fatalf("unexpected X-LabKit-Key-ID header = %q", got)
	}
	wantFingerprint, err := keycrypto.PublicKeyFingerprint(pub)
	if err != nil {
		t.Fatalf("PublicKeyFingerprint() error = %v", err)
	}
	if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-Fingerprint")); got != wantFingerprint {
		t.Fatalf("X-LabKit-Key-Fingerprint = %q, want %q", got, wantFingerprint)
	}
	signature, err := base64.StdEncoding.DecodeString(r.Header.Get("X-LabKit-Signature"))
	if err != nil {
		t.Fatalf("DecodeString(signature): %v", err)
	}
	sum := sha256.Sum256(body)
	payload := auth.NewPayload(strings.ToUpper(r.Method)+" "+path, timestamp, r.Header.Get("X-LabKit-Nonce"), nil).WithContentHash(hex.EncodeToString(sum[:]))
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		t.Fatalf("SigningBytes() error = %v", err)
	}
	if !ed25519.Verify(pub, signingBytes, signature) {
		t.Fatal("update signature did not verify")
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	return updateCapture{
		method: r.Method,
		path:   r.URL.Path,
		body:   decoded,
	}
}

func verifySignedRequest(t *testing.T, r *http.Request, path string, body []byte, pub ed25519.PublicKey) error {
	t.Helper()
	wantFingerprint, err := keycrypto.PublicKeyFingerprint(pub)
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-ID")); got != "" {
		return fmt.Errorf("unexpected key id header %q", got)
	}
	if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-Fingerprint")); got != wantFingerprint {
		return fmt.Errorf("key fingerprint = %q, want %q", got, wantFingerprint)
	}
	timestamp, err := time.Parse(time.RFC3339Nano, r.Header.Get("X-LabKit-Timestamp"))
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(r.Header.Get("X-LabKit-Signature"))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	payload := auth.NewPayload(strings.ToUpper(r.Method)+" "+path, timestamp, r.Header.Get("X-LabKit-Nonce"), nil).WithContentHash(hex.EncodeToString(sum[:]))
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, signingBytes, signature) {
		return errors.New("signature mismatch")
	}
	return nil
}

func maybeServeSubmitPrecheck(t *testing.T, w http.ResponseWriter, r *http.Request, pub ed25519.PublicKey) bool {
	t.Helper()
	if r.Method != http.MethodGet || r.URL.Path != "/api/labs/sorting/submit/precheck" {
		return false
	}
	if err := verifySignedRequest(t, r, "/api/labs/sorting/submit/precheck", nil, pub); err != nil {
		t.Fatalf("verifySignedRequest() error = %v", err)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{})
	return true
}

func mustParseInt64(t *testing.T, raw string) int64 {
	t.Helper()
	var value int64
	if _, err := fmt.Sscan(strings.TrimSpace(raw), &value); err != nil {
		t.Fatalf("parse int64 %q: %v", raw, err)
	}
	return value
}
