package plans

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// countingPlaces fails the test's expectation if Cards ever asks Google.
type countingPlaces struct{ calls int }

func (c *countingPlaces) Details(context.Context, string, string) (maps.PlaceDetails, error) {
	c.calls++
	return maps.PlaceDetails{}, errors.New("unexpected Places call")
}

// Over the cap is refused before anything is looked up (no database needed).
func TestCardsTooManyIDs(t *testing.T) {
	ids := make([]uuid.UUID, MaxPlanCards+1)
	for i := range ids {
		ids[i] = uuid.New()
	}
	var inv *InvalidError
	if _, err := (&Service{}).Cards(context.Background(), uuid.New(), ids, time.Now()); !errors.As(err, &inv) {
		t.Fatalf("%d ids: %v, want InvalidError", len(ids), err)
	}
}

// GetPlanCards returns what GetPlan would let the caller read (open plans
// and recently closed ones in their city), leaves out the rest, and never
// calls Google.
func TestIntegrationCards(t *testing.T) {
	f := newFixture(t, "zz-plans-cards", "UTC", 2)
	other := newFixture(t, "zz-plans-cards-b", "UTC", 1)
	ctx := context.Background()
	ja := f.users[0]
	f.addProfile(t, ja, "ja")

	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	w := Window{Start: day(7), Days: 14, Deadline: day(14)}
	mk := func(fx *fixture, area matching.AreaID) uuid.UUID {
		act := &matching.Activity{ID: matching.ActivityID(fx.act)}
		id, err := Create(ctx, fx.q, fx.city, matching.Suggestion{
			Plan: matching.Plan{Area: area, Activity: act.ID, Primary: "ja"}, Audience: 6, Score: 1,
		}, w, map[matching.ActivityID]*matching.Activity{act.ID: act})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	open, closed, old, elsewhere := mk(f, matching.AreaID(f.area)), mk(f, matching.AreaID(f.area)), mk(f, matching.AreaID(f.area)), mk(other, matching.AreaID(other.area))
	now := time.Now()
	if _, err := f.pool.Exec(ctx, `update app.plans set status = 'closed', closed_reason = 'no_group', closed_at = $2 where id = $1`, closed, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `update app.plans set status = 'closed', closed_reason = 'done', closed_at = $2 where id = $1`, old, now.Add(-15*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `insert into app.plan_signups (plan_id, user_id) values ($1, $2)`, closed, ja); err != nil {
		t.Fatal(err)
	}

	places := &countingPlaces{}
	svc := &Service{DB: f.pool, Places: places}
	got, err := svc.Cards(ctx, ja, []uuid.UUID{open, closed, old, elsewhere, uuid.New(), open}, now)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[uuid.UUID]FeedItem{}
	for _, it := range got {
		byID[it.ID] = it
	}
	if len(got) != 2 || len(byID) != 2 {
		t.Fatalf("cards = %d (%v), want the open and the recently closed plan only", len(got), got)
	}
	if it := byID[open]; it.Title == "" || it.ClosedReason != "" || it.Level != matching.Top {
		t.Errorf("open card = %+v", it)
	}
	if it := byID[closed]; it.Title == "" || it.ClosedReason != ClosedNoGroup || !it.SignedUp || it.Minutes != -1 {
		t.Errorf("closed card = %+v", it)
	}
	if places.calls != 0 {
		t.Errorf("Cards called Places %d times, want 0", places.calls)
	}
	// The same ids through GetPlan agree on what is readable.
	for _, id := range []uuid.UUID{old, elsewhere} {
		if _, _, _, err := svc.Plan(ctx, ja, id, now); !errors.Is(err, ErrNotFound) {
			t.Errorf("Plan(%s) = %v; the cards should leave out only what Plan refuses", id, err)
		}
	}
	if got, err := svc.Cards(ctx, ja, nil, now); err != nil || len(got) != 0 {
		t.Errorf("no ids: %v, %v", got, err)
	}
	if _, err := svc.Cards(ctx, f.users[1], []uuid.UUID{open}, now); !errors.Is(err, ErrNoProfile) {
		t.Errorf("no profile: %v, want ErrNoProfile", err)
	}
}

// The cap counts distinct ids: 51 ids with repeats but 50 plans is allowed,
// 51 different plans is not.
func TestCardsCapCountsDistinctIDs(t *testing.T) {
	distinct := make([]uuid.UUID, MaxPlanCards)
	for i := range distinct {
		distinct[i] = uuid.New()
	}
	withRepeat := append(append([]uuid.UUID{}, distinct...), distinct[0])
	if got, err := distinctCardIDs(withRepeat); err != nil || len(got) != MaxPlanCards {
		t.Errorf("%d ids, %d distinct: %d, %v; want allowed", len(withRepeat), MaxPlanCards, len(got), err)
	}
	var inv *InvalidError
	if _, err := distinctCardIDs(append(distinct, uuid.New())); !errors.As(err, &inv) {
		t.Errorf("51 distinct ids: %v, want InvalidError", err)
	}
}
