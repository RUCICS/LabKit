package submissions

import (
	"time"

	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/jackc/pgx/v5/pgtype"
)

type QuotaSummary struct {
	Daily     int    `json:"daily"`
	Used      int    `json:"used"`
	Left      int    `json:"left"`
	ResetHint string `json:"reset_hint"`
}

type LatestSubmissionHint struct {
	ContentHash string    `json:"content_hash"`
	CreatedAt   time.Time `json:"created_at"`
}

type SubmitPrecheck struct {
	Quota            *QuotaSummary         `json:"quota,omitempty"`
	LatestSubmission *LatestSubmissionHint `json:"latest_submission,omitempty"`
}

func DefaultQuotaLocation() *time.Location {
	return defaultQuotaLocation()
}

func QuotaWindowForTime(now time.Time, location *time.Location) (time.Time, time.Time) {
	return quotaWindowForTime(now, location)
}

func QuotaLocationName(location *time.Location) string {
	return quotaLocationName(location)
}

func ResetHintForLocation(location *time.Location) string {
	return "00:00 " + QuotaLocationName(location)
}

func BuildQuotaSummary(m *manifest.Manifest, used int, location *time.Location) *QuotaSummary {
	if m == nil || m.Quota.Daily <= 0 {
		return nil
	}
	left := m.Quota.Daily - used
	if left < 0 {
		left = 0
	}
	return &QuotaSummary{
		Daily:     m.Quota.Daily,
		Used:      used,
		Left:      left,
		ResetHint: ResetHintForLocation(location),
	}
}

func CountQuotaUsage(rows []sqlc.Submissions, start, end time.Time) int {
	used := 0
	for _, row := range rows {
		if row.QuotaState != "pending" && row.QuotaState != "charged" {
			continue
		}
		if !withinQuotaWindow(row.CreatedAt, start, end) {
			continue
		}
		used++
	}
	return used
}

func LatestSubmissionHintFromRow(row sqlc.Submissions) *LatestSubmissionHint {
	if row.ID == [16]byte{} {
		return nil
	}
	return &LatestSubmissionHint{
		ContentHash: row.ContentHash,
		CreatedAt:   row.CreatedAt.Time.UTC(),
	}
}

func withinQuotaWindow(createdAt pgtype.Timestamptz, start, end time.Time) bool {
	if !createdAt.Valid {
		return false
	}
	t := createdAt.Time
	return !t.Before(start) && t.Before(end)
}
