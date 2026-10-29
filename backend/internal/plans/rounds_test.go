package plans

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationRoundsWaiting: once a round's invitations go out, a signup
// left by someone matching can't place (no profile in the city) doesn't keep
// the plan going; one by someone who lives there does. The scheduler and an
// approved proposal share this rule (startRound).
func TestIntegrationRoundsWaiting(t *testing.T) {
	f := newFixture(t, "zz-plans-rounds", "UTC", 4)
	ctx := context.Background()
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	a, b, c, gone := f.users[0], f.users[1], f.users[2], f.users[3]
	for _, u := range []uuid.UUID{a, b, c} {
		f.addProfile(t, u, "ja")
	}
	uid := func(u uuid.UUID) matching.UserID { return matching.UserID(u.String()) }
	w := Window{Start: day(7), Days: 14, Deadline: day(14)}
	next := Window{Start: day(21), Days: 14, Deadline: day(28)}
	round := func(name string, invited, waiting []matching.UserID) (bool, Open) {
		t.Helper()
		plan := matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja", Secondary: name}
		id, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 4, Score: 1}, w, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range append(append([]matching.UserID{}, invited...), waiting...) {
			if err := SaveSignup(ctx, f.pool, id, matching.Signup{User: u}); err != nil {
				t.Fatal(err)
			}
		}
		_, closed, err := SaveEvent(ctx, f.pool, Event{
			PlanID: id, Round: 1, ConfirmBy: day(11), Next: next, Now: day(5),
			Scheduled: matching.Scheduled{
				Occasion: matching.Occasion{Date: day(12), Block: matching.Afternoon},
				Groups:   []matching.Group{{Invited: invited, Quorum: 2, MinAttendance: 2, Score: 1}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		open, err := ListOpen(ctx, f.q, f.city)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range open {
			if o.ID == id {
				return closed, o
			}
		}
		return closed, Open{}
	}
	if closed, o := round("en", []matching.UserID{uid(a), uid(b)}, []matching.UserID{uid(gone)}); !closed || o.Round != 0 {
		t.Errorf("only someone who left still waiting: closed %v, open %+v", closed, o)
	}
	if closed, o := round("ko", []matching.UserID{uid(a), uid(b)}, []matching.UserID{uid(c), uid(gone)}); closed || o.Round != 2 || !o.Window.Start.Equal(next.Start) {
		t.Errorf("someone in the city still waiting: closed %v, open %+v", closed, o)
	}
}
