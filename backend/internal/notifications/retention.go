package notifications

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

const (
	// Keep is how long a finished notification stays, for privacy: the
	// queue isn't a history of who was told what.
	Keep = 30 * 24 * time.Hour
	// deleteBatch is how many rows one retention delete removes at most.
	deleteBatch = 1000
)

// DeleteOld deletes notifications older than Keep that were sent or ran
// out of attempts; unsent ones with attempts left stay for the sender.
// It deletes in batches, so a pass never holds a long lock, and returns the
// total.
func DeleteOld(ctx context.Context, q *dbgen.Queries, now time.Time) (int64, error) {
	var total int64
	for {
		n, err := q.DeleteOldNotifications(ctx, dbgen.DeleteOldNotificationsParams{
			Before: pgtype.Timestamptz{Time: now.Add(-Keep), Valid: true}, Max: deleteBatch,
		})
		total += n
		if err != nil || n < deleteBatch {
			return total, err
		}
	}
}
