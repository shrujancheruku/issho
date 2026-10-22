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
	titles := map[[3]string]string{} // (language, activity, follow-up) → title
	for i, p := range pending {
		if len(tokens[p.UserID]) == 0 {
			rep.NoDevice++
			done = append(done, p.ID)
			continue
		}
		key := [3]string{p.PrimaryLanguage, p.ActivityID, deref(p.FollowUpID)}
		title, ok := titles[key]
		if !ok {
			plan := matching.OpenPlan{Plan: matching.Plan{Activity: matching.ActivityID(p.ActivityID), FollowUp: matching.ActivityID(deref(p.FollowUpID))}}
			t, _, err := n.Plans.titles(ctx, q, p.CityID, Caller{Primary: p.PrimaryLanguage}, []matching.OpenPlan{plan})
			if err != nil {
				return rep, err
			}
			title, titles[key] = t[0], t[0]
		}
		body := n.Catalog.Text(ctx, n.Plans.Translations, p.PrimaryLanguage, notificationText[p.Kind])
		for _, tok := range tokens[p.UserID] {
			msgs = append(msgs, push.Message{
				To: tok, Title: title, Body: body, Sound: "default", ChannelID: "default",
				// The app opens the event when the notification is tapped.
				Data: map[string]string{"eventId": p.EventID.String(), "kind": p.Kind},
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
	delivered := map[int]bool{}
	for j, t := range tickets {
		switch {
		case t.OK:
			delivered[owner[j]] = true
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
	if len(failed) > 0 {
		if err := q.MarkNotificationsFailed(ctx, failed); err != nil {
			return rep, err
		}
	}
	return rep, q.MarkNotificationsSent(ctx, dbgen.MarkNotificationsSentParams{Ids: done, At: at})
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
