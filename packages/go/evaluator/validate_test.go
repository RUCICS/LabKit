package evaluator

import (
	"strings"
	"testing"
	"time"

	"labkit.local/packages/go/manifest"
)

func TestValidateResultAcceptsValidScoredResult(t *testing.T) {
	lab := testManifest(t, []manifest.MetricSection{
		{ID: "throughput", Sort: manifest.MetricSortDesc},
		{ID: "latency", Sort: manifest.MetricSortAsc},
	})

	result := Result{
		Verdict: VerdictScored,
		Scores: map[string]float64{
			"throughput": 1.82,
			"latency":    1.45,
		},
		Detail: &Detail{
			Format:  DetailFormatMarkdown,
			Content: "### Breakdown\n\nok",
		},
		Message: "compiled with warnings",
	}

	if err := ValidateResult(lab, result); err != nil {
		t.Fatalf("ValidateResult() error = %v", err)
	}
}

func TestValidateResultRejectsMissingScoresForDeclaredMetrics(t *testing.T) {
	lab := testManifest(t, []manifest.MetricSection{
		{ID: "throughput", Sort: manifest.MetricSortDesc},
		{ID: "latency", Sort: manifest.MetricSortAsc},
	})

	result := Result{
		Verdict: VerdictScored,
		Scores: map[string]float64{
			"throughput": 1.82,
		},
	}

	err := ValidateResult(lab, result)
	if err == nil || !strings.Contains(err.Error(), "missing score for metric \"latency\"") {
		t.Fatalf("ValidateResult() error = %v, want missing metric error", err)
	}
}

func TestValidateResultRejectsExtraMetricKeys(t *testing.T) {
	lab := testManifest(t, []manifest.MetricSection{
		{ID: "throughput", Sort: manifest.MetricSortDesc},
	})

	result := Result{
		Verdict: VerdictScored,
		Scores: map[string]float64{
			"throughput": 1.82,
			"bonus":      9.99,
		},
	}

	err := ValidateResult(lab, result)
	if err == nil || !strings.Contains(err.Error(), "unexpected metric \"bonus\"") {
		t.Fatalf("ValidateResult() error = %v, want extra metric error", err)
	}
}

func TestValidateResultRejectsScoresOnNonScoredVerdict(t *testing.T) {
	lab := testManifest(t, []manifest.MetricSection{
		{ID: "throughput", Sort: manifest.MetricSortDesc},
	})

	result := Result{
		Verdict: VerdictRejected,
		Scores: map[string]float64{
			"throughput": 1.82,
		},
	}

	err := ValidateResult(lab, result)
	if err == nil || !strings.Contains(err.Error(), "scores") {
		t.Fatalf("ValidateResult() error = %v, want non-scored scores error", err)
	}
}

func TestValidateResultRejectsInvalidDetailFormat(t *testing.T) {
	lab := testManifest(t, []manifest.MetricSection{
		{ID: "throughput", Sort: manifest.MetricSortDesc},
	})

	result := Result{
		Verdict: VerdictScored,
		Scores: map[string]float64{
			"throughput": 1.82,
		},
		Detail: &Detail{
			Format:  "html",
			Content: "<b>bad</b>",
		},
	}

	err := ValidateResult(lab, result)
	if err == nil || !strings.Contains(err.Error(), "detail.format") {
		t.Fatalf("ValidateResult() error = %v, want detail.format error", err)
	}
}

func TestValidateResultRejectsInvalidVerdict(t *testing.T) {
	lab := testManifest(t, []manifest.MetricSection{
		{ID: "throughput", Sort: manifest.MetricSortDesc},
	})

	result := Result{
		Verdict: "pending",
	}

	err := ValidateResult(lab, result)
	if err == nil || !strings.Contains(err.Error(), "verdict") {
		t.Fatalf("ValidateResult() error = %v, want verdict error", err)
	}
}

func TestParseResultExtractsLastLineJSON(t *testing.T) {
	lab := testManifest(t, []manifest.MetricSection{
		{ID: "throughput", Sort: manifest.MetricSortDesc},
	})

	stdout := "compile log line\nstill running\n{\"verdict\":\"scored\",\"scores\":{\"throughput\":1.82}}\n"

	result, err := ParseResult(lab, []byte(stdout))
	if err != nil {
		t.Fatalf("ParseResult() error = %v", err)
	}
	if result.Verdict != VerdictScored {
		t.Fatalf("ParseResult() verdict = %q, want %q", result.Verdict, VerdictScored)
	}
	if got := result.Scores["throughput"]; got != 1.82 {
		t.Fatalf("ParseResult() throughput = %v, want 1.82", got)
	}
}

func TestParseResultRejectsTrailingBlankLastLine(t *testing.T) {
	lab := testManifest(t, []manifest.MetricSection{
		{ID: "throughput", Sort: manifest.MetricSortDesc},
	})

	stdout := "compile log line\n{\"verdict\":\"scored\",\"scores\":{\"throughput\":1.82}}\n\n"

	_, err := ParseResult(lab, []byte(stdout))
	if err == nil || !strings.Contains(err.Error(), "last line") {
		t.Fatalf("ParseResult() error = %v, want blank last line error", err)
	}
}

func TestParseResultRejectsNilManifest(t *testing.T) {
	stdout := "{\"verdict\":\"scored\",\"scores\":{\"throughput\":1.82}}\n"

	_, err := ParseResult(nil, []byte(stdout))
	if err == nil || !strings.Contains(err.Error(), "manifest is required") {
		t.Fatalf("ParseResult() error = %v, want nil manifest error", err)
	}
}

func TestValidateResultRejectsNilManifest(t *testing.T) {
	result := Result{
		Verdict: VerdictScored,
		Scores: map[string]float64{
			"throughput": 1.82,
		},
	}

	err := ValidateResult(nil, result)
	if err == nil || !strings.Contains(err.Error(), "manifest is required") {
		t.Fatalf("ValidateResult() error = %v, want nil manifest error", err)
	}
}

func TestValidateScoresRejectsNilManifest(t *testing.T) {
	err := ValidateScores(nil, map[string]float64{"throughput": 1.82})
	if err == nil || !strings.Contains(err.Error(), "manifest is required") {
		t.Fatalf("ValidateScores() error = %v, want nil manifest error", err)
	}
}

func testManifest(t *testing.T, metrics []manifest.MetricSection) *manifest.Manifest {
	t.Helper()

	m := &manifest.Manifest{
		Lab: manifest.LabSection{
			ID:   "lab-2026",
			Name: "Lab",
		},
		Submit: manifest.SubmitSection{
			Files: []string{"main.c"},
		},
		Eval: manifest.EvalSection{
			Image: "registry.example.edu/lab-eval:2026sp",
		},
		Quota: manifest.QuotaSection{
			Daily: 1,
		},
		Metrics: metrics,
		Board: manifest.BoardSection{
			RankBy: metrics[0].ID,
		},
		Schedule: manifest.ScheduleSection{
			Open:  mustParseTime(t, "2026-03-01T00:00:00+08:00"),
			Close: mustParseTime(t, "2026-04-15T23:59:59+08:00"),
		},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("manifest.Validate() error = %v", err)
	}
	return m
}

func mustParseTime(t *testing.T, raw string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", raw, err)
	}
	return ts
}
