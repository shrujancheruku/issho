package plans

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationSaveEventAfterAWithdrawal: a person withdraws after the
// pass read the signups (LoadWaiting) but before it saves the round. The save
// finds them gone under the plan's lock and writes nothing: nobody is placed
// or notified, and the plan stays open with the people still waiting.
func TestIntegrationSaveEventAfterAWithdrawal(t *testing.T) {
	f := newFixture(t, "zz-plans-stale", "UTC", 3)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "en")
	}
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	w := Window{Start: day(7), Days: 14, Deadline: day(20)}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "en"}, Audience: 6, Score: 1}, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	var ids []matching.UserID
	for _, u := range f.users {
		ids = append(ids, matching.UserID(u.String()))
		if err := SaveSignup(ctx, f.pool, planID, matching.Signup{User: matching.UserID(u.String())}); err != nil {
			t.Fatal(err)
		}
	}

	waiting, err := LoadWaiting(ctx, f.q, []uuid.UUID{planID}) // the pass reads...
	if err != nil || len(waiting[planID]) != 3 {
		t.Fatalf("LoadWaiting = %v, %v", waiting, err)
	}
	if err := WithdrawSignup(ctx, f.pool, planID, f.users[2]); err != nil { // ...then someone withdraws...
		t.Fatal(err)
	}
	_, _, err = SaveEvent(ctx, f.pool, Event{ // ...and it saves a round that includes them.
		PlanID: planID, Round: 1, ConfirmBy: day(11), Next: Window{Start: day(21), Days: 14, Deadline: day(30)}, Now: day(5), CheckSignups: true,
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(13), Block: matching.Evening},
			Groups:   []matching.Group{{Invited: ids, Quorum: 2, MinAttendance: 2, Score: 1}},
		},
	})
	if !errors.Is(err, ErrSignupsChanged) {
		t.Fatalf("SaveEvent = %v, want ErrSignupsChanged", err)
	}
	counts := map[string]string{
		"events":        "select count(*) from app.events where plan_id = $1",
		"group members": "select count(*) from app.group_members m join app.events e on e.id = m.event_id where e.plan_id = $1",
		"notifications": "select count(*) from app.notifications n join app.events e on e.id = n.event_id where e.plan_id = $1",
		"open plan":     "select count(*) from app.plans where id = $1 and status = 'open' and round = 1",
		"signups":       "select count(*) from app.plan_signups where plan_id = $1",
	}
	want := map[string]int{"events": 0, "group members": 0, "notifications": 0, "open plan": 1, "signups": 2}
	for name, sql := range counts {
		var n int
		if err := f.pool.QueryRow(ctx, sql, planID).Scan(&n); err != nil || n != want[name] {
			t.Errorf("%s = %d, %v; want %d", name, n, err, want[name])
		}
	}
}
