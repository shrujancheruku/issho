package notifications

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

const (
	// ReceiptAfter is how long after sending a push its receipt is checked:
	// Expo suggests waiting about 15 minutes for Apple and Google to answer.
	ReceiptAfter = 15 * time.Minute
	// ReceiptKeep is how long a ticket waits for its receipt; Expo keeps
	// receipts for 24 hours.
	ReceiptKeep = 24 * time.Hour
	// receiptBatch is how many tickets one pass checks at most.
	receiptBatch = 1000
)

// ReceiptReport counts what a receipts pass did: pushes that reached Apple
// or Google, ones they refused, device tokens dropped because the app is
// gone, and tickets given up on (no receipt within a day).
type ReceiptReport struct {
	Delivered, Refused, Unregistered int
	Expired                          int64
}

// CheckReceipts reads the receipts for pushes sent at least ReceiptAfter
// ago. A "DeviceNotRegistered" receipt deletes that device's token (and its
// other tickets with it). Other refusals are logged: they're about our
// credentials or the message, so resending wouldn't help. A ticket whose
// receipt isn't ready is checked again on a later pass, until ReceiptKeep.
func (n *Notifier) CheckReceipts(ctx context.Context, now time.Time) (ReceiptReport, error) {
	var rep ReceiptReport
	q := dbgen.New(n.DB)
	expired, err := q.DeleteExpiredPushTickets(ctx, pgtype.Timestamptz{Time: now.Add(-ReceiptKeep), Valid: true})
	if err != nil {
		return rep, err
	}
	rep.Expired = expired
	due, err := q.ListDuePushTickets(ctx, dbgen.ListDuePushTicketsParams{Ready: pgtype.Timestamptz{Time: now.Add(-ReceiptAfter), Valid: true}, Max: receiptBatch})
	if err != nil || len(due) == 0 {
		return rep, err
	}
	ids := make([]string, len(due))
	for i, t := range due {
		ids[i] = t.ID
	}
	receipts, err := n.Sender.Receipts(ctx, ids)
	if err != nil {
		// Checked again on the next pass.
		return rep, fmt.Errorf("push receipts: %w", err)
	}
	var checked, gone []string
	for _, t := range due {
		r, ok := receipts[t.ID]
		if !ok {
			continue // not ready yet
		}
		checked = append(checked, t.ID)
		switch {
		case r.OK:
			rep.Delivered++
		case r.Error == "DeviceNotRegistered":
			if !slices.Contains(gone, t.Token) {
				gone = append(gone, t.Token)
			}
		default:
			rep.Refused++
			n.Log.Warn("push refused", "error", r.Error, "message", r.Message)
		}
	}
	rep.Unregistered = len(gone)
	if len(gone) > 0 {
		if err := q.DeletePushTokens(ctx, gone); err != nil {
			return rep, err
		}
	}
	return rep, q.DeletePushTickets(ctx, checked)
}
