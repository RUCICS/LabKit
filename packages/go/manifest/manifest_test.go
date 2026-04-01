package manifest

import (
	"strings"
	"testing"
)

func TestParseValidSingleMetricLab(t *testing.T) {
	data := `
[lab]
id = "datalab-2026"
name = "Data Lab"

[submit]
files = ["bits.c"]

[eval]
image = "registry.example.edu/datalab-eval:2026sp"

[quota]
daily = 10

[[metric]]
id = "ops"
sort = "asc"

[board]
pick = false

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	m, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got := m.Metrics[0].Name; got != "ops" {
		t.Fatalf("metric name = %q, want %q", got, "ops")
	}
	if got := m.Metrics[0].Unit; got != "" {
		t.Fatalf("metric unit = %q, want empty", got)
	}
	if got := m.Board.RankBy; got != "ops" {
		t.Fatalf("rank_by = %q, want ops", got)
	}
	if got := m.Schedule.Visible; !got.Equal(m.Schedule.Open) {
		t.Fatalf("visible = %v, want open = %v", got, m.Schedule.Open)
	}
}

func TestParseValidMultiMetricLab(t *testing.T) {
	data := `
[lab]
id = "malloc-2026"
name = "Malloc Lab"

[submit]
files = ["mm.c"]

[eval]
image = "registry.example.edu/malloc-eval:2026sp"

[quota]
daily = 5

[[metric]]
id = "throughput"
sort = "desc"

[[metric]]
id = "utilization"
sort = "desc"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	m, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got := m.Board.RankBy; got != "throughput" {
		t.Fatalf("rank_by = %q, want first metric id", got)
	}
	if got := m.Metrics[0].Name; got != "throughput" {
		t.Fatalf("first metric name = %q, want throughput", got)
	}
	if got := m.Metrics[1].Name; got != "utilization" {
		t.Fatalf("second metric name = %q, want utilization", got)
	}
}

func TestParseRejectsDuplicateMetricIDs(t *testing.T) {
	data := `
[lab]
id = "dup-2026"
name = "Dup Lab"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/dup-eval:2026sp"

[quota]
daily = 1

[[metric]]
id = "score"
sort = "desc"

[[metric]]
id = "score"
sort = "desc"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	_, err := Parse([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Parse() error = %v, want duplicate metric id error", err)
	}
}

func TestParseRejectsNonURLSafeLabID(t *testing.T) {
	data := `
[lab]
id = "bad lab id"
name = "Bad Lab ID"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/bad-eval:2026sp"

[quota]
daily = 1

[[metric]]
id = "score"
sort = "desc"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	_, err := Parse([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "lab.id") {
		t.Fatalf("Parse() error = %v, want lab.id url-safe error", err)
	}
}

func TestParseRejectsInvalidRankBy(t *testing.T) {
	data := `
[lab]
id = "bad-rank-2026"
name = "Bad Rank"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/bad-eval:2026sp"

[quota]
daily = 1

[[metric]]
id = "score"
sort = "desc"

[board]
rank_by = "missing"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	_, err := Parse([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "rank_by") {
		t.Fatalf("Parse() error = %v, want rank_by error", err)
	}
}

func TestParseRejectsScheduleOrdering(t *testing.T) {
	data := `
[lab]
id = "bad-schedule-2026"
name = "Bad Schedule"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/bad-eval:2026sp"

[quota]
daily = 1

[[metric]]
id = "score"
sort = "desc"

[schedule]
open = 2026-04-15T23:59:59+08:00
close = 2026-03-01T00:00:00+08:00
`

	_, err := Parse([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "schedule") {
		t.Fatalf("Parse() error = %v, want schedule ordering error", err)
	}
}

func TestParseRejectsVisibleAfterOpen(t *testing.T) {
	data := `
[lab]
id = "bad-visible-2026"
name = "Bad Visible"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/bad-eval:2026sp"

[quota]
daily = 1

[[metric]]
id = "score"
sort = "desc"

[schedule]
visible = 2026-04-15T23:59:59+08:00
open = 2026-03-01T00:00:00+08:00
close = 2026-04-20T23:59:59+08:00
`

	_, err := Parse([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "visible") {
		t.Fatalf("Parse() error = %v, want visible ordering error", err)
	}
}

func TestParseRejectsUnsupportedSortDirection(t *testing.T) {
	data := `
[lab]
id = "bad-sort-2026"
name = "Bad Sort"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/bad-eval:2026sp"

[quota]
daily = 1

[[metric]]
id = "score"
sort = "up"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	_, err := Parse([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "sort") {
		t.Fatalf("Parse() error = %v, want sort direction error", err)
	}
}

func TestParseNormalizesQuotaFreeToEmptySlice(t *testing.T) {
	data := `
[lab]
id = "quota-free-2026"
name = "Quota Free"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/quota-eval:2026sp"

[quota]
daily = 1

[[metric]]
id = "score"
sort = "desc"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	m, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if m.Quota.Free == nil {
		t.Fatalf("quota.free = nil, want empty slice")
	}
	if len(m.Quota.Free) != 0 {
		t.Fatalf("quota.free len = %d, want 0", len(m.Quota.Free))
	}
}

func TestParseRejectsUnsupportedQuotaFreeVerdict(t *testing.T) {
	data := `
[lab]
id = "bad-free-2026"
name = "Bad Free"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/bad-free-eval:2026sp"

[quota]
daily = 1
free = ["scored"]

[[metric]]
id = "score"
sort = "desc"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	_, err := Parse([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "quota.free") {
		t.Fatalf("Parse() error = %v, want quota.free verdict error", err)
	}
}

func TestPublicProjectionDoesNotAliasSubmissionFilesOrQuotaFree(t *testing.T) {
	data := `
[lab]
id = "public-alias-2026"
name = "Public Alias"

[submit]
files = ["x.c", "y.c"]

[eval]
image = "registry.example.edu/public-alias-eval:2026sp"

[quota]
daily = 1
free = ["build_failed"]

[[metric]]
id = "score"
sort = "desc"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	m, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	pub := m.Public()
	pub.Submit.Files[0] = "changed.c"
	pub.Quota.Free[0] = "rejected"

	if m.Submit.Files[0] != "x.c" {
		t.Fatalf("source submit files mutated: %v", m.Submit.Files)
	}
	if m.Quota.Free[0] != "build_failed" {
		t.Fatalf("source quota.free mutated: %v", m.Quota.Free)
	}
}

func TestParseRejectsMissingRequiredSections(t *testing.T) {
	data := `
[lab]
id = "missing-2026"
name = "Missing"
`

	_, err := Parse([]byte(data))
	if err == nil || !strings.Contains(err.Error(), "submit") {
		t.Fatalf("Parse() error = %v, want missing section error", err)
	}
}

func TestPublicProjectionOmitsEvalImage(t *testing.T) {
	data := `
[lab]
id = "public-2026"
name = "Public Lab"

[submit]
files = ["x.c"]

[eval]
image = "registry.example.edu/public-eval:2026sp"
timeout = 123

[quota]
daily = 1

[[metric]]
id = "score"
sort = "desc"

[schedule]
open = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
`

	m, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	pub := m.Public()
	if pub.Eval.Image != "" {
		t.Fatalf("public eval image = %q, want empty", pub.Eval.Image)
	}
	if pub.Eval.Timeout != 123 {
		t.Fatalf("public eval timeout = %d, want 123", pub.Eval.Timeout)
	}
}
