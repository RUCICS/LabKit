package evaluator

// Verdict is the terminal evaluator outcome.
type Verdict string

const (
	VerdictBuildFailed Verdict = "build_failed"
	VerdictRejected    Verdict = "rejected"
	VerdictScored      Verdict = "scored"
	VerdictError       Verdict = "error"
)

// DetailFormat controls how detail content should be rendered.
type DetailFormat string

const (
	DetailFormatText     DetailFormat = "text"
	DetailFormatMarkdown DetailFormat = "markdown"
)

// Detail is the optional structured evaluator payload shown to students.
type Detail struct {
	Format  DetailFormat `json:"format"`
	Content string       `json:"content"`
}

// Result is the evaluator protocol payload emitted on stdout's last line.
type Result struct {
	Verdict Verdict            `json:"verdict"`
	Scores  map[string]float64 `json:"scores,omitempty"`
	Detail  *Detail            `json:"detail,omitempty"`
	Message string             `json:"message,omitempty"`
}
