package plans

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/push"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// notificationText is the UI catalogue key for each kind's message. The
// plan's title is the notification's title; dates are left to the app,
// which formats them in the person's language (Go can't).
var notificationText = map[string]string{
	NotifyInvited:   "push.invited",
	NotifyConfirmed: "push.confirmed",
	NotifyCancelled: "push.cancelled",
	NotifyNoGroup:   "push.noGroup",
}

// Notifier sends the notifications queued in app.notifications.
type Notifier struct {
	Plans   *Service // the database, and titles in the person's language
	Sender  push.Sender
	Catalog *uistrings.Catalog
	Log     *slog.Logger
	// Batch is how many notifications one pass sends at most; 0 = 500.
	Batch int
}

// NotifyReport counts what a pass did: notifications sent, finished without
// a device to send to, and to retry; and device tokens dropped because Expo
// says the app is gone.
type NotifyReport struct {
	Sent, NoDevice, Failed, Unregistered int
}

// Send sends pending notifications, oldest first. A notification is marked
// sent once Expo accepted it for at least one of the person's devices, or
// when they have no device (nothing to retry). If Expo can't be reached the
// batch is retried on a later pass, a few times at most.
func (n *Notifier) Send(ctx context.Context, now time.Time) (NotifyReport, error) {
	var rep NotifyReport
	q := dbgen.New(n.Plans.DB)
	max := n.Batch
	if max == 0 {
		max = 500
	}
	pending, err := q.ListPendingNotifications(ctx, int32(max))
	if err != nil || len(pending) == 0 {
		return rep, err
	}
	users := make([]uuid.UUID, 0, len(pending))
	seen := map[uuid.UUID]bool{}
	for _, p := range pending {
		if !seen[p.UserID] {
			seen[p.UserID] = true
			users = append(users, p.UserID)
		}
	}
	tokenRows, err := q.ListPushTokens(ctx, users)
	if err != nil {
		return rep, err
	}
	tokens := map[uuid.UUID][]string{}
	for _, r := range tokenRows {
		tokens[r.UserID] = append(tokens[r.UserID], r.Token)
	}

	var msgs []push.Message
	var owner []int // pending index for each message
	var done []int64
	titles := map[[4]string]string{} // (language, activity, follow-up, a walk's title) → title
	for i, p := range pending {
		if len(tokens[p.UserID]) == 0 {
			rep.NoDevice++
			done = append(done, p.ID)
			continue
		}
		key := [4]string{p.PrimaryLanguage, p.ActivityID, deref(p.FollowUpID), string(p.Title)}
		title, ok := titles[key]
		if !ok {
			plan := titled{matching.Plan{Activity: matching.ActivityID(p.ActivityID), FollowUp: matching.ActivityID(deref(p.FollowUpID))}, p.Title}
			t, _, err := n.Plans.titles(ctx, q, p.CityID, Caller{Primary: p.PrimaryLanguage}, []titled{plan})
			if err != nil {
				return rep, err
			}
			title, titles[key] = t[0], t[0]
		}
		body := n.Catalog.Text(ctx, n.Plans.Translations, p.PrimaryLanguage, notificationText[p.Kind])
		for _, tok := range tokens[p.UserID] {
			msgs = append(msgs, push.Message{
				To: tok, Title: title, Body: body, Sound: "default", ChannelID: "default",
				// The app opens the event when the notification is tapped, or
				// the feed for one about a plan.
				Data: notificationData(p),
			})
			owner = append(owner, i)
		}
	}

	at := pgtype.Timestamptz{Time: now, Valid: true}
	if len(msgs) == 0 {
		return rep, q.MarkNotificationsSent(ctx, dbgen.MarkNotificationsSentParams{Ids: done, At: at})
	}
	tickets, err := n.Sender.Send(ctx, msgs)
	if err != nil {
		var failed []int64
		for i, p := range pending {
			if slices.Contains(owner, i) {
				failed = append(failed, p.ID)
			}
		}
		rep.Failed = len(failed)
		if mErr := q.MarkNotificationsFailed(ctx, failed); mErr != nil {
			return rep, mErr
		}
		if mErr := q.MarkNotificationsSent(ctx, dbgen.MarkNotificationsSentParams{Ids: done, At: at}); mErr != nil {
			return rep, mErr
		}
		return rep, fmt.Errorf("send notifications: %w", err)
	}
	var unregistered []string
	var ticketIDs, ticketTokens []string
	delivered := map[int]bool{}
	for j, t := range tickets {
		switch {
		case t.OK:
			delivered[owner[j]] = true
			if t.ID != "" {
				ticketIDs, ticketTokens = append(ticketIDs, t.ID), append(ticketTokens, msgs[j].To)
			}
		case t.Error == "DeviceNotRegistered":
			unregistered = append(unregistered, msgs[j].To)
		default:
			n.Log.Warn("push rejected", "error", t.Error, "message", t.Message)
		}
	}
	var failed []int64
	for i, p := range pending {
		if !slices.Contains(owner, i) {
			continue
		}
		// A rejection that isn't a lost device (e.g. bad credentials) would
		// fail the same way again; only lost devices end the attempts early.
		switch {
		case delivered[i]:
			done = append(done, p.ID)
			rep.Sent++
		case allUnregistered(i, owner, msgs, unregistered):
			// Every device is gone: nothing left to retry.
			done = append(done, p.ID)
			rep.NoDevice++
		default:
			failed = append(failed, p.ID)
		}
	}
	rep.Failed, rep.Unregistered = len(failed), len(unregistered)
	if len(unregistered) > 0 {
		if err := q.DeletePushTokens(ctx, unregistered); err != nil {
			return rep, err
		}
	}
	if len(ticketIDs) > 0 {
		if err := q.InsertPushTickets(ctx, dbgen.InsertPushTicketsParams{Ids: ticketIDs, Tokens: ticketTokens, At: at}); err != nil {
			return rep, err
		}
	}
	if len(failed) > 0 {
		if err := q.MarkNotificationsFailed(ctx, failed); err != nil {
			return rep, err
		}
	}
	return rep, q.MarkNotificationsSent(ctx, dbgen.MarkNotificationsSentParams{Ids: done, At: at})
}

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
	q := dbgen.New(n.Plans.DB)
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

func notificationData(p dbgen.ListPendingNotificationsRow) map[string]string {
	d := map[string]string{"kind": p.Kind}
	if p.EventID.Valid {
		d["eventId"] = uuid.UUID(p.EventID.Bytes).String()
	}
	if p.PlanID.Valid {
		d["planId"] = uuid.UUID(p.PlanID.Bytes).String()
	}
	return d
}

// allUnregistered reports whether every message for pending notification i
// went to a device that's no longer registered.
func allUnregistered(i int, owner []int, msgs []push.Message, unregistered []string) bool {
	for j, o := range owner {
		if o == i && !slices.Contains(unregistered, msgs[j].To) {
			return false
		}
	}
	return true
}
