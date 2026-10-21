package plans

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

func TestSignupFromProto(t *testing.T) {
	m := &isshov1.Signup{
		Pattern: []*isshov1.AvailabilitySlot{{Weekday: 6, Block: isshov1.TimeBlock_TIME_BLOCK_MORNING}},
		Unavailable: []*isshov1.UnavailableTime{
			{Date: "2030-01-09"}, // the whole day
			{Date: "2030-01-12", Block: isshov1.TimeBlock_TIME_BLOCK_EVENING},
		},
	}
	got, err := signupFromProto(m)
	if err != nil {
		t.Fatal(err)
	}
	want := matching.Signup{
		Pattern: []matching.Slot{{Weekday: time.Saturday, Block: matching.Morning}},
		Unavailable: []matching.Occasion{
			{Date: time.Date(2030, 1, 9, 0, 0, 0, 0, time.UTC)},
			{Date: time.Date(2030, 1, 12, 0, 0, 0, 0, time.UTC), Block: matching.Evening},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if back := signupToProto(got); !proto.Equal(back, m) {
		t.Errorf("round trip: %v, want %v", back, m)
	}
	if s, err := signupFromProto(nil); err != nil || s.Pattern != nil || s.Unavailable != nil {
		t.Errorf("nil signup = %+v, %v; want the usual availability", s, err)
	}

	slot := func(day int32, b isshov1.TimeBlock) *isshov1.AvailabilitySlot {
		return &isshov1.AvailabilitySlot{Weekday: day, Block: b}
	}
	bad := map[string]*isshov1.Signup{
		"weekday 7":      {Pattern: []*isshov1.AvailabilitySlot{slot(7, isshov1.TimeBlock_TIME_BLOCK_MORNING)}},
		"no block":       {Pattern: []*isshov1.AvailabilitySlot{slot(1, isshov1.TimeBlock_TIME_BLOCK_UNSPECIFIED)}},
		"duplicate slot": {Pattern: []*isshov1.AvailabilitySlot{slot(1, isshov1.TimeBlock_TIME_BLOCK_MORNING), slot(1, isshov1.TimeBlock_TIME_BLOCK_MORNING)}},
		"not a date":     {Unavailable: []*isshov1.UnavailableTime{{Date: "9 January"}}},
		"unknown block":  {Unavailable: []*isshov1.UnavailableTime{{Date: "2030-01-09", Block: 9}}},
		"duplicate date": {Unavailable: []*isshov1.UnavailableTime{{Date: "2030-01-09"}, {Date: "2030-01-09"}}},
	}
	for name, m := range bad {
		if _, err := signupFromProto(m); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// fakePlaces answers Place Details from a map of id → language → details.
type fakePlaces map[string]map[string]maps.PlaceDetails

func (f fakePlaces) Details(_ context.Context, id, lang string) (maps.PlaceDetails, error) {
	if d, ok := f[id][lang]; ok {
		return d, nil
	}
	return maps.PlaceDetails{}, errors.New("not found")
}

// TestIntegrationService runs PlanService's logic for three people: one who
// speaks the plan's language, one who doesn't, and one without a profile.
// The plan has a follow-up, and venues are picked for both.
func TestIntegrationService(t *testing.T) {
	f := newFixture(t, "zz-plans-service", "UTC", 3)
	ctx := context.Background()
	ja, en, none := f.users[0], f.users[1], f.users[2]
	f.addProfile(t, ja, "ja")
	f.addProfile(t, en, "en")
	// Three cafés in the area: the rejected one must never be picked, and the
	// approved one wins over a plain candidate.
	for _, v := range [][]any{{"zz-cafe-rejected", "rejected"}, {"zz-cafe-candidate", "candidate"}, {"zz-cafe-approved", "approved"}} {
		if _, err := f.pool.Exec(ctx, `insert into app.venues (google_place_id, city_id, area_id, category_id, status)
			values ($1, $2, $3, 'cafe', $4)`, v[0], f.city, f.area, v[1]); err != nil {
			t.Fatal(err)
		}
	}
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	w := Window{Start: day(7), Days: 14, Deadline: day(14)}
	act := &matching.Activity{ID: matching.ActivityID(f.act), VenueCategories: []matching.CategoryID{"cafe"}}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{
		Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: act.ID, FollowUp: act.ID, Primary: "ja"}, Audience: 6, Score: 1,
	}, w, map[matching.ActivityID]*matching.Activity{act.ID: act})
	if err != nil {
		t.Fatal(err)
	}
	cafe := maps.PlaceDetails{Name: "カフェ", Address: "吉祥寺1-1", MapsURL: "https://maps.example/1", Location: maps.LatLng{Lat: 35.7, Lng: 139.58}}
	svc := &Service{DB: f.pool, Places: fakePlaces{"zz-cafe-approved": {"ja": cafe}}}

	feed, err := svc.Feed(ctx, ja)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 || feed[0].ID != planID || feed[0].Level != matching.Top || feed[0].Minutes != 0 || feed[0].SignedUp {
		t.Fatalf("feed = %+v", feed)
	}
	if feed[0].Venue != "zz-cafe-approved" || feed[0].FollowUpVenue != "zz-cafe-approved" {
		t.Errorf("venues = %q, %q; want the approved café for both", feed[0].Venue, feed[0].FollowUpVenue)
	}
	// No translator: the English title (the fixture's activity is "Test").
	if feed[0].Title != "Test, then test" || feed[0].SecondaryTitle != "" {
		t.Errorf("title = %q / %q", feed[0].Title, feed[0].SecondaryTitle)
	}
	if feed, err := svc.Feed(ctx, en); err != nil || len(feed) != 1 || feed[0].Level != matching.NotForYou {
		t.Errorf("feed for an English speaker = %+v, %v; want not for you", feed, err)
	}
	if _, err := svc.Feed(ctx, none); !errors.Is(err, ErrNoProfile) {
		t.Errorf("feed without a profile: %v, want ErrNoProfile", err)
	}
	if _, err := svc.SignUp(ctx, en, planID, matching.Signup{}); !errors.Is(err, ErrNotForYou) {
		t.Errorf("signup by an English speaker: %v, want ErrNotForYou", err)
	}
	if _, _, _, err := svc.Plan(ctx, ja, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown plan: %v, want ErrNotFound", err)
	}
	var inv *InvalidError
	if _, err := svc.SignUp(ctx, ja, planID, matching.Signup{Unavailable: []matching.Occasion{{Date: day(21)}}}); !errors.As(err, &inv) {
		t.Errorf("signup blocking a date after the window: %v, want InvalidError", err)
	}

	signup := matching.Signup{
		Pattern:     []matching.Slot{{Weekday: time.Sunday, Block: matching.Morning}},
		Unavailable: []matching.Occasion{{Date: day(20), Block: matching.Morning}}, // the window's last day
	}
	item, err := svc.SignUp(ctx, ja, planID, signup)
	if err != nil {
		t.Fatal(err)
	}
	if !item.SignedUp || item.Signups != 1 {
		t.Errorf("after signing up: %+v", item)
	}
	// Signing up again only replaces the times: still one signup.
	if item, err = svc.SignUp(ctx, ja, planID, signup); err != nil || item.Signups != 1 {
		t.Errorf("signing up again: %+v, %v", item, err)
	}
	item, steps, got, err := svc.Plan(ctx, ja, planID)
	if err != nil {
		t.Fatal(err)
	}
	// Both stops at the same café, in the caller's language (ja, no second).
	wantSteps := []Step{{Activity: act.ID, Venue: cafe}, {Activity: act.ID, Venue: cafe, WalkMinutes: 1}}
	if !reflect.DeepEqual(steps, wantSteps) {
		t.Errorf("steps = %+v, want %+v", steps, wantSteps)
	}
	signup.User = matching.UserID(ja.String())
	if !item.SignedUp || item.Signups != 1 || got == nil || !reflect.DeepEqual(*got, signup) {
		t.Errorf("Plan = %+v, %+v; want signed up with %+v", item, got, signup)
	}

	if err := svc.Withdraw(ctx, ja, planID); err != nil {
		t.Fatal(err)
	}
	if item, _, got, err := svc.Plan(ctx, ja, planID); err != nil || item.SignedUp || item.Signups != 0 || got != nil {
		t.Errorf("after withdrawing: %+v, %+v, %v", item, got, err)
	}
	if err := Close(ctx, f.q, planID, ClosedWithdrawn, day(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SignUp(ctx, ja, planID, matching.Signup{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("signup for a closed plan: %v, want ErrNotFound", err)
	}
}

func TestTitle(t *testing.T) {
	for _, c := range []struct{ act, follow, want string }{
		{"Anime and manga", "", "Anime and manga"},
		{"Climbing and bouldering", "Lunch or dinner out", "Climbing and bouldering, then lunch or dinner out"},
		{"Hiking", "BBQ night", "Hiking, then BBQ night"},
		{"Hiking", "Ébisu tour", "Hiking, then ébisu tour"},
	} {
		if got := Title(c.act, c.follow); got != c.want {
			t.Errorf("Title(%q, %q) = %q, want %q", c.act, c.follow, got, c.want)
		}
	}
}

func TestWalkMinutes(t *testing.T) {
	kichijoji := maps.LatLng{Lat: 35.7030, Lng: 139.5796}
	// About 1 km east: 1,000 m × 1.3 / 80 m a minute ≈ 17 minutes.
	east := maps.LatLng{Lat: 35.7030, Lng: 139.5907}
	if got := walkMinutes(kichijoji, east); got < 15 || got > 18 {
		t.Errorf("walkMinutes ≈ 1 km = %d, want about 17", got)
	}
	if walkMinutes(kichijoji, maps.LatLng{}) != 0 || walkMinutes(kichijoji, kichijoji) != 1 {
		t.Error("unknown location should be 0, the same place 1")
	}
}
