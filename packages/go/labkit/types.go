package labkit

// MetricSort indicates whether higher or lower values rank first.
type MetricSort string

const (
	MetricSortAsc  MetricSort = "asc"
	MetricSortDesc MetricSort = "desc"
)

// Lab is the core lab identity shared across API, CLI, and worker code.
type Lab struct {
	ID   string
	Name string
	Tags map[string]string
}

// Metric defines a leaderboard column.
type Metric struct {
	ID   string
	Name string
	Sort MetricSort
	Unit string
}

// SubmissionStatus tracks the lifecycle of a submission.
type SubmissionStatus string

const (
	SubmissionStatusQueued  SubmissionStatus = "queued"
	SubmissionStatusRunning SubmissionStatus = "running"
	SubmissionStatusDone    SubmissionStatus = "done"
	SubmissionStatusTimeout SubmissionStatus = "timeout"
	SubmissionStatusError   SubmissionStatus = "error"
)

// Verdict is the evaluator outcome reported by the worker.
type Verdict string

const (
	VerdictBuildFailed Verdict = "build_failed"
	VerdictRejected    Verdict = "rejected"
	VerdictScored      Verdict = "scored"
	VerdictError       Verdict = "error"
)

// Submission is the shared domain record for a student submission.
type Submission struct {
	ID          string
	LabID       string
	UserID      string
	Status      SubmissionStatus
	Verdict     Verdict
	ContentHash string
}
