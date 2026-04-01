package manifest

import (
	"fmt"
	"regexp"
	"strings"
)

var urlSafeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Validate checks that a manifest is complete and internally consistent.
func (m *Manifest) Validate() error {
	var problems []string

	if m.Lab.ID == "" {
		problems = append(problems, "lab.id is required")
	} else if !urlSafeID.MatchString(m.Lab.ID) {
		problems = append(problems, "lab.id must be URL-safe")
	}
	if m.Lab.Name == "" {
		problems = append(problems, "lab.name is required")
	}
	if len(m.Submit.Files) == 0 {
		problems = append(problems, "submit.files is required")
	}
	if m.Eval.Image == "" {
		problems = append(problems, "eval.image is required")
	}
	if m.Quota.Daily <= 0 {
		problems = append(problems, "quota.daily must be positive")
	}
	for i, verdict := range m.Quota.Free {
		if verdict != "build_failed" && verdict != "rejected" {
			problems = append(problems, fmt.Sprintf("quota.free[%d] must be build_failed or rejected", i))
		}
	}
	if len(m.Metrics) == 0 {
		problems = append(problems, "at least one metric is required")
	}
	if m.Schedule.Open.IsZero() {
		problems = append(problems, "schedule.open is required")
	}
	if m.Schedule.Close.IsZero() {
		problems = append(problems, "schedule.close is required")
	}

	seen := map[string]struct{}{}
	for i := range m.Metrics {
		metric := &m.Metrics[i]
		if metric.ID == "" {
			problems = append(problems, fmt.Sprintf("metric[%d].id is required", i))
		}
		if metric.Sort != MetricSortAsc && metric.Sort != MetricSortDesc {
			problems = append(problems, fmt.Sprintf("metric[%d].sort must be asc or desc", i))
		}
		if metric.ID != "" {
			if _, ok := seen[metric.ID]; ok {
				problems = append(problems, fmt.Sprintf("duplicate metric id %q", metric.ID))
			}
			seen[metric.ID] = struct{}{}
		}
	}

	if m.Board.RankBy == "" {
		problems = append(problems, "board.rank_by is required")
	} else if _, ok := seen[m.Board.RankBy]; !ok {
		problems = append(problems, fmt.Sprintf("board.rank_by %q does not match any metric id", m.Board.RankBy))
	}

	if !m.Schedule.Open.IsZero() && !m.Schedule.Close.IsZero() && m.Schedule.Open.After(m.Schedule.Close) {
		problems = append(problems, "schedule.open must be before or equal to schedule.close")
	}
	if !m.Schedule.Visible.IsZero() && !m.Schedule.Open.IsZero() && m.Schedule.Visible.After(m.Schedule.Open) {
		problems = append(problems, "schedule.visible must be before or equal to schedule.open")
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid manifest: %s", strings.Join(problems, "; "))
	}
	return nil
}
