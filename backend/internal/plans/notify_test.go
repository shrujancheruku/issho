package plans

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/push"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// fakeSender records messages and answers like Expo: DeviceNotRegistered
// for tokens in gone, or fails the whole request when down.
type fakeSender struct {
	sent []push.Message
	gone map[string]bool
	down bool
	// receipts are the receipts Expo has ready, by ticket id.
	receipts map[string]push.Receipt
}

func (f *fakeSender) Receipts(_ context.Context, ids []string) (map[string]push.Receipt, error) {
	if f.down {
		return nil, errors.New("expo unreachable")
	}
	out := map[string]push.Receipt{}
	for _, id := range ids {
		if r, ok := f.receipts[id]; ok {
			out[id] = r
		}
	}
	return out, nil
}

func (f *fakeSender) Send(_ context.Context, msgs []push.Message) ([]push.Ticket, error) {
	if f.down {
		return nil, errors.New("expo unreachable")
	}
	tickets := make([]push.Ticket, len(msgs))
	for i, m := range msgs {
		f.sent = append(f.sent, m)
		// The ticket id is the token, so tests can set its receipt.
		tickets[i] = push.Ticket{OK: !f.gone[m.To], ID: m.To}
		if f.gone[m.To] {
			tickets[i].Error = "DeviceNotRegistered"
		}
	}
	return tickets, nil
}

// TestIntegrationNotifications: scheduling queues invitations; the sender
// delivers them to registered devices, drops uninstalled ones and finishes
// people without a device; reaching quorum notifies the others; an Expo
// outage leaves them queued for a retry.
func TestIntegrationNotifications(t *testing.T) {
	f := newFixture(t, "zz-plans-notify", "UTC", 3)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	a, b, c := f.users[0], f.users[1], f.users[2]
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	plan := matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 6, Score: 1}, Window{Start: day(7), Days: 14, Deadline: day(14)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	invited := []matching.UserID{matching.UserID(a.String()), matching.UserID(b.String()), matching.UserID(c.String())}
	eventID, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 1, StartsAt: day(12).Add(14 * time.Hour), ConfirmBy: day(11), Now: day(5),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: invited, Quorum: 2, MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// a has two devices, one of them uninstalled; b has one; c has none.
	for _, r := range []dbgen.UpsertPushTokenParams{
		{Token: "ExponentPushToken[a1]", UserID: a, Platform: "android"},
		{Token: "ExponentPushToken[a-gone]", UserID: a, Platform: "ios"},
		{Token: "ExponentPushToken[b1]", UserID: b, Platform: "android"},
	} {
		if err := f.q.UpsertPushToken(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := uistrings.Load()
	if err != nil {
		t.Fatal(err)
	}
	sender := &fakeSender{gone: map[string]bool{"ExponentPushToken[a-gone]": true}}
	n := &Notifier{Plans: &Service{DB: f.pool}, Sender: sender, Catalog: catalog, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	rep, err := n.Send(ctx, day(5))
	if err != nil {
		t.Fatal(err)
	}
	if rep != (NotifyReport{Sent: 2, NoDevice: 1, Unregistered: 1}) {
		t.Errorf("report = %+v", rep)
	}
	if len(sender.sent) != 3 {
		t.Fatalf("sent %d messages, want 3 (a ×2, b)", len(sender.sent))
	}
	m := sender.sent[0]
	// No translator: English. The fixture's activity is called "Test".
	if m.Title != "Test" || m.Body != "A date is set. Can you go? Open Issho to answer." ||
		m.Data["eventId"] != eventID.String() || m.Data["kind"] != NotifyInvited || m.ChannelID != "default" {
		t.Errorf("message = %+v", m)
	}
	var tokens int
	if err := f.pool.QueryRow(ctx, "select count(*) from app.push_tokens where user_id = $1", a).Scan(&tokens); err != nil || tokens != 1 {
		t.Errorf("a's tokens after an uninstall: %d, %v; want 1", tokens, err)
	}
	if rep, err := n.Send(ctx, day(5)); err != nil || rep != (NotifyReport{}) {
		t.Errorf("second pass = %+v, %v; want nothing left", rep, err)
	}

	// b confirms, then a: the group reaches quorum and b (not a) hears.
	svc := &Service{DB: f.pool}
	for _, u := range []uuid.UUID{b, a} {
		if err := svc.Respond(ctx, u, eventID, Confirm, day(6)); err != nil {
			t.Fatal(err)
		}
	}
	sender.sent, sender.down = nil, true
	if rep, err := n.Send(ctx, day(6)); err == nil || rep.Failed != 1 {
		t.Errorf("with Expo down: %+v, %v; want one failed", rep, err)
	}
	sender.down = false
	if rep, err := n.Send(ctx, day(6)); err != nil || rep.Sent != 1 || len(sender.sent) != 1 ||
		sender.sent[0].To != "ExponentPushToken[b1]" || sender.sent[0].Data["kind"] != NotifyConfirmed {
		t.Errorf("after the outage: %+v, %v, %+v", rep, err, sender.sent)
	}

	// Another plan closes without a group while b waits: b hears, and the
	// notification is about the plan (the app opens the feed).
	other, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}, Audience: 6, Score: 1},
		Window{Start: day(7), Days: 14, Deadline: day(14)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SignUp(ctx, b, other, matching.Signup{}); err != nil {
		t.Fatal(err)
	}
	if err := f.q.ClosePlanNoGroup(ctx, dbgen.ClosePlanNoGroupParams{ID: other, ClosedAt: pgtype.Timestamptz{Time: day(14), Valid: true}}); err != nil {
		t.Fatal(err)
	}
	sender.sent = nil
	if rep, err := n.Send(ctx, day(14)); err != nil || rep.Sent != 1 || len(sender.sent) != 1 {
		t.Fatalf("no-group notice: %+v, %v", rep, err)
	}
	if m := sender.sent[0]; m.To != "ExponentPushToken[b1]" || m.Data["kind"] != NotifyNoGroup || m.Data["planId"] != other.String() || m.Data["eventId"] != "" ||
		m.Body != "Not enough people signed up this time, so it is closed. Open Issho to see other plans." {
		t.Errorf("no-group message = %+v", m)
	}
	// Closing again (already closed) tells nobody twice.
	if err := f.q.ClosePlanNoGroup(ctx, dbgen.ClosePlanNoGroupParams{ID: other, ClosedAt: pgtype.Timestamptz{Time: day(15), Valid: true}}); err != nil {
		t.Fatal(err)
	}
	if rep, _ := n.Send(ctx, day(15)); rep != (NotifyReport{}) {
		t.Errorf("second close notified again: %+v", rep)
	}
}
