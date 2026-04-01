package evaluator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"labkit.local/packages/go/manifest"
)

// ExtractLastLine returns the JSON-bearing final line, allowing one trailing
// line terminator but rejecting an extra blank trailing line.
func ExtractLastLine(stdout []byte) ([]byte, error) {
	if len(stdout) == 0 {
		return nil, fmt.Errorf("stdout is empty")
	}

	trimmed := stdout
	if stdout[len(stdout)-1] == '\n' {
		trimmed = stdout[:len(stdout)-1]
		if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\n' {
			return nil, fmt.Errorf("stdout last line is empty")
		}
	}
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("stdout last line is empty")
	}

	lines := bytes.Split(trimmed, []byte("\n"))
	last := bytes.TrimSuffix(lines[len(lines)-1], []byte("\r"))
	if len(last) == 0 {
		return nil, fmt.Errorf("stdout last line is empty")
	}
	return last, nil
}

// ParseResult extracts the last stdout line, decodes it, and validates it.
func ParseResult(m *manifest.Manifest, stdout []byte) (Result, error) {
	line, err := ExtractLastLine(stdout)
	if err != nil {
		return Result{}, err
	}

	var result Result
	if err := json.Unmarshal(line, &result); err != nil {
		return Result{}, fmt.Errorf("parse evaluator result JSON: %w", err)
	}
	if err := ValidateResult(m, result); err != nil {
		return Result{}, err
	}
	return result, nil
}

// ValidateResult validates the evaluator payload against the protocol contract.
func ValidateResult(m *manifest.Manifest, result Result) error {
	if m == nil {
		return fmt.Errorf("manifest is required")
	}
	if err := validateVerdict(result.Verdict); err != nil {
		return err
	}
	if err := validateDetail(result.Detail); err != nil {
		return err
	}

	if result.Verdict == VerdictScored {
		if err := ValidateScores(m, result.Scores); err != nil {
			return err
		}
	} else if len(result.Scores) > 0 {
		return fmt.Errorf("scores are only allowed for verdict %q", VerdictScored)
	}

	return nil
}

// ValidateScores checks that scored metrics exactly match the manifest declaration.
func ValidateScores(m *manifest.Manifest, scores map[string]float64) error {
	if m == nil {
		return fmt.Errorf("manifest is required")
	}
	if len(scores) == 0 {
		return fmt.Errorf("scores are required for verdict %q", VerdictScored)
	}

	declared := make(map[string]struct{}, len(m.Metrics))
	for _, metric := range m.Metrics {
		declared[metric.ID] = struct{}{}
		if _, ok := scores[metric.ID]; !ok {
			return fmt.Errorf("missing score for metric %q", metric.ID)
		}
	}

	for key := range scores {
		if _, ok := declared[key]; !ok {
			return fmt.Errorf("unexpected metric %q", key)
		}
	}

	return nil
}

func validateVerdict(verdict Verdict) error {
	switch verdict {
	case VerdictBuildFailed, VerdictRejected, VerdictScored, VerdictError:
		return nil
	case "":
		return fmt.Errorf("verdict is required")
	default:
		return fmt.Errorf("verdict %q is invalid", verdict)
	}
}

func validateDetail(detail *Detail) error {
	if detail == nil {
		return nil
	}

	switch detail.Format {
	case DetailFormatText, DetailFormatMarkdown:
		return nil
	case "":
		return fmt.Errorf("detail.format is required when detail is present")
	default:
		return fmt.Errorf("detail.format %q is invalid; want one of %s", detail.Format, strings.Join([]string{string(DetailFormatText), string(DetailFormatMarkdown)}, ", "))
	}
}
