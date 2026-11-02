// Package notifications owns push notifications end to end: the outbox
// (app.notifications) that other modules write to in their own
// transaction, the worker's send pass through Expo, the tickets and
// receipts that follow, and dropping the device tokens Expo says are gone.
//
// What a notification says is not decided here. Each module that queues
// kinds registers a Renderer for them (plans for event and plan kinds,
// crews for crew kinds), so this package imports neither.
package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/push"
)

// Ref is what a notification is about: exactly one id is set (the table
// enforces it). The app opens the event, the feed (a plan) or the crew.
type Ref struct {
	EventID, PlanID, CrewID uuid.UUID
}

func nullable(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil} }

func fromNullable(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return uuid.UUID(id.Bytes)
}

// Enqueue queues a notification of the given kind about ref for each user.
// Pass the caller's transaction as q: it's an outbox, so a change that
// rolls back notifies nobody.
func Enqueue(ctx context.Context, q *dbgen.Queries, kind string, users []uuid.UUID, ref Ref) error {
	if len(users) == 0 {
		return nil
	}
	return q.InsertNotifications(ctx, dbgen.InsertNotificationsParams{
		UserIds: users, Kind: kind,
		EventID: nullable(ref.EventID), PlanID: nullable(ref.PlanID), CrewID: nullable(ref.CrewID),
	})
}

// Pending is a queued notification waiting to be rendered.
type Pending struct {
	ID       int64
	UserID   uuid.UUID
	Kind     string
	Ref      Ref
	Language string // the person's display language
}

// Content is what a notification says.
type Content struct {
	Title, Body string
}

// Renderer writes the notifications of the kinds it was registered for.
// It gets every pending item of those kinds in a pass at once, so it can
// look things up once for items about the same plan or crew, and returns
// one Content per item, in order. An error stops the pass and leaves the
// notifications queued.
type Renderer interface {
	Render(ctx context.Context, items []Pending) ([]Content, error)
}

// Notifier sends the notifications queued in app.notifications.
type Notifier struct {
	DB     *pgxpool.Pool
	Sender push.Sender
	Log    *slog.Logger
	// Batch is how many notifications one pass sends at most; 0 = 500.
	Batch int

	renderers map[string]Renderer
}

// Register makes r write notifications of these kinds. Call it at startup,
// before the first pass; registering a kind twice panics, since two
// modules writing the same kind is a bug.
func (n *Notifier) Register(r Renderer, kinds ...string) {
	if n.renderers == nil {
		n.renderers = map[string]Renderer{}
	}
	for _, k := range kinds {
		if _, dup := n.renderers[k]; dup {
			panic("notifications: renderer registered twice for kind " + k)
		}
		n.renderers[k] = r
	}
}

// Report counts what a pass did: notifications sent, finished without a
// device to send to, and to retry; and device tokens dropped because Expo
// says the app is gone.
type Report struct {
	Sent, NoDevice, Failed, Unregistered int
}

// data is what the app gets with a notification: the kind and what it is
// about, so a tap opens the event, the feed or the crew.
func data(p Pending) map[string]string {
	d := map[string]string{"kind": p.Kind}
	if p.Ref.EventID != uuid.Nil {
		d["eventId"] = p.Ref.EventID.String()
	}
	if p.Ref.PlanID != uuid.Nil {
		d["planId"] = p.Ref.PlanID.String()
	}
	if p.Ref.CrewID != uuid.Nil {
		d["crewId"] = p.Ref.CrewID.String()
	}
	return d
}

// render writes the content of each item, asking each renderer once with
// all its items. An item whose kind has no renderer is returned in
// unrendered and logged; it counts as failed (a deploy that forgot to
// register a kind shouldn't spin forever: the attempts run out).
func (n *Notifier) render(ctx context.Context, items []Pending) (content map[int]Content, unrendered []int, err error) {
	byRenderer := map[Renderer][]int{}
	var order []Renderer // first-seen, so passes behave the same each time
	for i, p := range items {
		r, ok := n.renderers[p.Kind]
		if !ok {
			n.Log.Error("no renderer for notification kind", "kind", p.Kind, "id", p.ID)
			unrendered = append(unrendered, i)
			continue
		}
		if _, seen := byRenderer[r]; !seen {
			order = append(order, r)
		}
		byRenderer[r] = append(byRenderer[r], i)
	}
	content = map[int]Content{}
	for _, r := range order {
		idx := byRenderer[r]
		batch := make([]Pending, len(idx))
		for j, i := range idx {
			batch[j] = items[i]
		}
		out, err := r.Render(ctx, batch)
		if err != nil {
			return nil, nil, err
		}
		if len(out) != len(batch) {
			return nil, nil, fmt.Errorf("notifications: renderer returned %d contents for %d items", len(out), len(batch))
		}
		for j, i := range idx {
			content[i] = out[j]
		}
	}
	return content, unrendered, nil
}

// Send sends pending notifications, oldest first. A notification is marked
// sent once Expo accepted it for at least one of the person's devices, or
// when they have no device (nothing to retry). If Expo can't be reached
// the batch is retried on a later pass, a few times at most. If it fails
// partway, the messages Expo had already accepted count as sent (so are
// not delivered twice) and only the rest are retried.
func (n *Notifier) Send(ctx context.Context, now time.Time) (Report, error) {
	var rep Report
	q := dbgen.New(n.DB)
	max := n.Batch
	if max == 0 {
		max = 500
	}
	rows, err := q.ListPendingNotifications(ctx, int32(max))
	if err != nil || len(rows) == 0 {
		return rep, err
	}
	pending := make([]Pending, len(rows))
	users := make([]uuid.UUID, 0, len(rows))
	seen := map[uuid.UUID]bool{}
	for i, r := range rows {
		pending[i] = Pending{
			ID: r.ID, UserID: r.UserID, Kind: r.Kind, Language: r.PrimaryLanguage,
			Ref: Ref{EventID: fromNullable(r.EventID), PlanID: fromNullable(r.PlanID), CrewID: fromNullable(r.CrewID)},
		}
		if !seen[r.UserID] {
			seen[r.UserID] = true
			users = append(users, r.UserID)
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

	// Only people with a device are rendered for.
	var withDevice []Pending
	var done []int64
	for _, p := range pending {
		if len(tokens[p.UserID]) == 0 {
			rep.NoDevice++
			done = append(done, p.ID)
			continue
		}
		withDevice = append(withDevice, p)
	}
	content, unrendered, err := n.render(ctx, withDevice)
	if err != nil {
		return rep, err
	}

	var msgs []push.Message
	var owner []int // index in withDevice of each message's notification
	var failed []int64
	for i, p := range withDevice {
		c, ok := content[i]
		if !ok {
			continue
		}
		for _, tok := range tokens[p.UserID] {
			msgs = append(msgs, push.Message{To: tok, Title: c.Title, Body: c.Body, Sound: "default", ChannelID: "default", Data: data(p)})
			owner = append(owner, i)
		}
	}
	for _, i := range unrendered {
		failed = append(failed, withDevice[i].ID)
	}

	at := pgtype.Timestamptz{Time: now, Valid: true}
	var tickets []push.Ticket
	var sendErr error
	if len(msgs) > 0 {
		// Tickets come back for the messages sent before an error too (see
		// push.Sender), so what Expo accepted is recorded either way.
		tickets, sendErr = n.Sender.Send(ctx, msgs)
		if len(tickets) > len(msgs) {
			return rep, fmt.Errorf("push: %d tickets for %d messages", len(tickets), len(msgs))
		}
	}
	var unregistered, ticketIDs, ticketTokens []string
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
	for _, i := range slices.Compact(slices.Clone(owner)) {
		p := withDevice[i]
		// A rejection that isn't a lost device (e.g. bad credentials) would
		// fail the same way again; only lost devices end the attempts early.
		// Messages after a failed batch have no ticket, so a notification
		// with any of them is neither delivered nor all-gone: it retries
		// (unless another of its devices already took it).
		switch {
		case delivered[i]:
			done = append(done, p.ID)
			rep.Sent++
		case allUnregistered(i, owner, msgs, unregistered, len(tickets)):
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
	if err := q.MarkNotificationsSent(ctx, dbgen.MarkNotificationsSentParams{Ids: done, At: at}); err != nil {
		return rep, err
	}
	if sendErr != nil {
		return rep, fmt.Errorf("send notifications: %w", sendErr)
	}
	return rep, nil
}

// allUnregistered reports whether every message for notification i got a
// ticket (the first `ticketed` messages) and went to a device that's no
// longer registered.
func allUnregistered(i int, owner []int, msgs []push.Message, unregistered []string, ticketed int) bool {
	for j, o := range owner {
		if o == i && (j >= ticketed || !slices.Contains(unregistered, msgs[j].To)) {
			return false
		}
	}
	return true
}
