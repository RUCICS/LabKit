package labkit

import (
	"errors"
	"testing"
)

func TestCoreDomainTypesExist(t *testing.T) {
	lab := Lab{ID: "lab-1", Name: "Lab 1"}
	metric := Metric{ID: "throughput", Sort: MetricSortDesc}
	submission := Submission{ID: "sub-1", Status: SubmissionStatusQueued}
	verdict := VerdictScored

	if lab.ID == "" || lab.Name == "" {
		t.Fatalf("lab fields not populated: %+v", lab)
	}
	if metric.Sort != MetricSortDesc {
		t.Fatalf("unexpected metric sort: %v", metric.Sort)
	}
	if submission.Status != SubmissionStatusQueued {
		t.Fatalf("unexpected submission status: %v", submission.Status)
	}
	if verdict != VerdictScored {
		t.Fatalf("unexpected verdict value: %v", verdict)
	}
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorClass
	}{
		{name: "user", err: NewUserError("bad input"), want: ErrorClassUser},
		{name: "system", err: NewSystemError("db down"), want: ErrorClassSystem},
		{name: "evaluator", err: NewEvaluatorError("runner failed"), want: ErrorClassEvaluator},
		{name: "admin", err: NewAdminError("conflict"), want: ErrorClassAdmin},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyError(tc.err); got != tc.want {
				t.Fatalf("classify error = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClassifyErrorNil(t *testing.T) {
	if got := ClassifyError(nil); got != "" {
		t.Fatalf("classify nil = %q, want empty", got)
	}
}

func TestClassifiedErrorWrapsCause(t *testing.T) {
	cause := errors.New("db unavailable")
	err := WrapSystemError("query failed", cause)

	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is did not match wrapped cause")
	}

	var classified interface{ Class() ErrorClass }
	if !errors.As(err, &classified) {
		t.Fatalf("errors.As did not find classified error")
	}
	if got := classified.Class(); got != ErrorClassSystem {
		t.Fatalf("class = %v, want %v", got, ErrorClassSystem)
	}
}

func TestBaseConstructorsDoNotTakeCause(t *testing.T) {
	if got := NewSystemError("query failed"); got == nil {
		t.Fatalf("expected error")
	}
}
