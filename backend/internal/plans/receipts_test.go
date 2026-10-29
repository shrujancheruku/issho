package plans

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/push"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// TestIntegrationPushReceipts: pushes Expo accepted are checked 15 minutes
// later. A delivered one is done; "DeviceNotRegistered" drops the token;
// another refusal is logged and done; one without a receipt yet waits,
// until it's given up on after a day.
func TestIntegrationPushReceipts(t *testing.T) {
	f := newFixture(t, "zz-plans-receipts", "UTC", 4)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}, Audience: 6, Score: 1},
		Window{Start: day(7), Days: 14, Deadline: day(14)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var invited []matching.UserID
	for _, u := range f.users {
		invited = append(invited, matching.UserID(u.String()))
	}
	if _, _, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 1, StartsAt: day(12).Add(14 * time.Hour), ConfirmBy: day(11), Now: day(5),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: invited, Quorum: 2, MinAttendance: 2, Score: 1}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	tok := func(s string) string { return "ExponentPushToken[zz-receipts-" + s + "]" }
	for i, name := range []string{"ok", "gone", "refused", "late"} {
		if err := f.q.UpsertPushToken(ctx, dbgen.UpsertPushTokenParams{Token: tok(name), UserID: f.users[i], Platform: "android"}); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := uistrings.Load()
	if err != nil {
		t.Fatal(err)
	}
	sender := &fakeSender{receipts: map[string]push.Receipt{
		tok("ok"):      {OK: true},
		tok("gone"):    {Error: "DeviceNotRegistered"},
		tok("refused"): {Error: "InvalidCredentials", Message: "bad FCM key"},
		// tok("late") has no receipt yet.
	}}
	n := &Notifier{Plans: &Service{DB: f.pool}, Sender: sender, Catalog: catalog, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	sent := day(5)
	if rep, err := n.Send(ctx, sent); err != nil || rep.Sent != 4 {
		t.Fatalf("send = %+v, %v", rep, err)
	}
	tickets := func() (n int) {
		t.Helper()
		if err := f.pool.QueryRow(ctx, "select count(*) from app.push_tickets where token like 'ExponentPushToken[zz-receipts-%'").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := tickets(); got != 4 {
		t.Fatalf("%d tickets stored, want 4", got)
	}

	// Expired isn't compared: the test's clock is in 2030, so other tickets
	// in a shared local database look a day old to it.
	if rep, err := n.CheckReceipts(ctx, sent.Add(10*time.Minute)); err != nil || rep.Delivered+rep.Refused+rep.Unregistered != 0 || tickets() != 4 {
		t.Errorf("after 10 minutes: %+v, %v; want nothing checked yet", rep, err)
	}
	rep, err := n.CheckReceipts(ctx, sent.Add(ReceiptAfter+time.Minute))
	if rep.Expired = 0; err != nil || rep != (ReceiptReport{Delivered: 1, Refused: 1, Unregistered: 1}) {
		t.Errorf("after 16 minutes: %+v, %v", rep, err)
	}
	var gone bool
	if err := f.pool.QueryRow(ctx, "select not exists (select 1 from app.push_tokens where token = $1)", tok("gone")).Scan(&gone); err != nil || !gone {
		t.Errorf("uninstalled device's token still there (%v)", err)
	}
	if got := tickets(); got != 1 {
		t.Errorf("%d tickets left, want 1 (the late one)", got)
	}
	if rep, err := n.CheckReceipts(ctx, sent.Add(ReceiptKeep+time.Minute)); err != nil || rep.Expired < 1 || tickets() != 0 {
		t.Errorf("after a day: %+v, %v; want the late one given up", rep, err)
	}
}
