package evaluation

import (
	"context"

	"labkit.local/packages/go/db/sqlc"
)

func refreshLeaderboard(ctx context.Context, tx Tx, submission sqlc.Submissions) error {
	_, err := tx.UpsertLeaderboardEntry(ctx, sqlc.UpsertLeaderboardEntryParams{
		UserID:       submission.UserID,
		LabID:        submission.LabID,
		SubmissionID: submission.ID,
	})
	return err
}
