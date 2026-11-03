package plans

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationLoadWaiting: loading several plans' signups at once (the
// scheduling stage's three queries, #185) gives each plan what LoadSignups
// gives it alone, and keeps one plan's people and times out of another's.
func TestIntegrationLoadWaiting(t *testing.T) {
	f := newFixture(t, "zz-plans-waiting", "UTC", 4)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "en")
	}
	uid := func(i int) matching.UserID { return matching.UserID(f.users[i].String()) }
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	w := Window{Start: day(7), Days: 14, Deadline: day(20)}
	suggestion := matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "en"}, Audience: 6, Score: 1}

	var plans []uuid.UUID
	for range 3 {
		id, err := Create(ctx, f.q, f.city, suggestion, w, nil)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, id)
	}
	// The first two share a person with different times; the third has no one.
	signups := map[int][]matching.Signup{
		0: {
			{User: uid(0), Pattern: []matching.Slot{{Weekday: time.Monday, Block: matching.Morning}}, Unavailable: []matching.Occasion{{Date: day(9)}}},
			{User: uid(1)},
		},
		1: {
			{User: uid(0), Pattern: []matching.Slot{{Weekday: time.Sunday, Block: matching.Evening}}, Unavailable: []matching.Occasion{{Date: day(12), Block: matching.Evening}}},
			{User: uid(2)},
			{User: uid(3)},
		},
	}
	for i, list := range signups {
		for _, s := range list {
			if err := SaveSignup(ctx, f.pool, plans[i], s); err != nil {
				t.Fatal(err)
			}
		}
	}

	got, err := LoadWaiting(ctx, f.q, plans)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range plans {
		alone, err := LoadSignups(ctx, f.q, id, w.Start)
		if err != nil {
			t.Fatal(err)
		}
		if len(alone) == 0 {
			if len(got[id]) != 0 {
				t.Errorf("plan %d: LoadWaiting = %+v, want nobody", i, got[id])
			}
			continue
		}
		if !reflect.DeepEqual(got[id], alone) {
			t.Errorf("plan %d: LoadWaiting = %+v, but LoadSignups = %+v", i, got[id], alone)
		}
	}
	if len(got[plans[0]]) != 2 || len(got[plans[1]]) != 3 {
		t.Errorf("waiting per plan = %d and %d, want 2 and 3", len(got[plans[0]]), len(got[plans[1]]))
	}
}
