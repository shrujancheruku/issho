package matching

import (
	"fmt"
	"slices"
	"testing"
)

var (
	act = func(id string, minSize, maxSize int, venues ...CategoryID) Activity {
		return Activity{ID: ActivityID(id), Category: "outdoors", Conversational: true, MinSize: minSize, MaxSize: maxSize, VenueCategories: venues}
	}
	testVenues = Venues{"a": {"cafe": 3, "bar": 1, "gym": 2}}
)

// fan makes n users in area a, fluent in Japanese, free on Sunday
// afternoons, with the same preferences and budget.
func fan(prefix string, n, budget int, prefs map[ActivityID]int) []User {
	var us []User
	for i := range n {
		us = append(us, User{
			ID: UserID(fmt.Sprintf("%s%d", prefix, i)), HomeArea: "a", Languages: fluent("ja"),
			Availability: []Slot{sunPM}, Budget: budget, Prefs: prefs, Gender: Unspecified, Size: Medium, Reliability: 1,
		})
	}
	return us
}

func suggestIDs(ss []Suggestion) []string {
	var out []string
	for _, s := range ss {
		id := string(s.Activity)
		if s.FollowUp != "" {
			id += ">" + string(s.FollowUp)
		}
		out = append(out, fmt.Sprintf("%s@%s", id, s.Area))
	}
	return out
}

func TestSuggest(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinAudience = 3
	cfg.UsersPerPlan = 1 // these cities are tiny
	evening := act("bar-hop", 3, 8, "bar")
	evening.Blocks = []Block{Evening}
	pricey := act("omakase", 3, 6, "cafe")
	pricey.CostLevel = 3
	boulder := act("bouldering", 3, 8, "gym")
	boulder.Conversational, boulder.Hours = false, 1.5
	ramen := act("ramen", 3, 8, "cafe")
	ramen.Category, ramen.Hours = "food", 1.5
	photoWalk := act("photo-walk", 3, 8)
	photoWalk.Conversational, photoWalk.Walk = false, true
	foodTour := act("food-tour", 3, 8, "cafe")
	foodTour.Category, foodTour.Walk = "food", true

	tests := []struct {
		name  string
		users []User
		acts  []Activity
		open  []Plan
		skip  []Plan
		want  []string
	}{
		{
			name:  "ranked by demand: loves beat likes",
			users: fan("x", 3, 3, map[ActivityID]int{"walk": Love, "cafe-hop": Like}),
			acts:  []Activity{act("cafe-hop", 3, 8, "cafe"), act("walk", 3, 8)},
			want:  []string{"walk@a", "cafe-hop@a"},
		},
		{
			name:  "too small an audience",
			users: slices.Concat(fan("x", 3, 3, map[ActivityID]int{"walk": Love}), fan("y", 2, 3, map[ActivityID]int{"futsal": Love})),
			acts:  []Activity{act("walk", 3, 8), act("futsal", 4, 10)},
			want:  []string{"walk@a"},
		},
		{
			name:  "no venue in the area, or nobody free at a time it suits",
			users: fan("x", 4, 3, map[ActivityID]int{"onsen": Love, "bar-hop": Love, "walk": OpenToTry}),
			acts:  []Activity{act("onsen", 3, 8, "onsen"), evening, act("walk", 3, 8)},
			want:  []string{"walk@a"},
		},
		{
			name:  "few venues and over-budget interest score lower",
			users: fan("x", 4, 1, map[ActivityID]int{"omakase": Love, "pub": Like, "walk": Like}),
			// omakase: 4 × 2 × 0.3 = 2.4; pub (1 of 3 venues): 4 × 1 / 3 = 1.33; walk: 4.
			acts: []Activity{pricey, act("pub", 3, 8, "bar"), act("walk", 3, 8)},
			want: []string{"walk@a", "omakase@a", "pub@a"},
		},
		{
			// Bouldering alone fills 1.5 of a 2-hour outing: 4 × 2 × 0.75 = 6;
			// with ramen, (1.5×2 + 1.5×1)/3 = 1.5 each, 4 × 1.5 × 1.1 = 6.6. A
			// ramen plan of its own would draw on the same people nearby.
			name:  "a short activity gains a meal when people like both",
			users: fan("x", 4, 3, map[ActivityID]int{"bouldering": Love, "ramen": Like}),
			acts:  []Activity{boulder, ramen},
			want:  []string{"bouldering>ramen@a"},
		},
		{
			name:  "plans already open aren't suggested again",
			users: fan("x", 3, 3, map[ActivityID]int{"walk": Love, "cafe-hop": Like}),
			acts:  []Activity{act("cafe-hop", 3, 8, "cafe"), act("walk", 3, 8)},
			open:  []Plan{{Area: "a", Activity: "walk", Primary: "ja"}},
			want:  []string{"cafe-hop@a"},
		},
		{
			name:  "a walk gets no follow-up and isn't one",
			users: fan("x", 4, 3, map[ActivityID]int{"photo-walk": Love, "food-tour": Like}),
			acts:  []Activity{photoWalk, foodTour},
			want:  []string{"photo-walk@a", "food-tour@a"},
		},
		{
			name: "skipped plans make way for the next best, even at the cap",
			// Three people, one plan per three: the skipped walk doesn't use
			// up the city's one plan.
			users: fan("x", 3, 3, map[ActivityID]int{"walk": Love, "cafe-hop": Like}),
			acts:  []Activity{act("cafe-hop", 3, 8, "cafe"), act("walk", 3, 8)},
			skip:  []Plan{{Area: "a", Activity: "walk"}},
			want:  []string{"cafe-hop@a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := cfg
			if tt.skip != nil {
				cfg.UsersPerPlan = 3
			}
			got := suggestIDs(Suggest(SuggestInput{Users: tt.users, Areas: []AreaID{"a"}, Activities: tt.acts, Venues: testVenues, Open: tt.open, Skip: tt.skip}, cfg))
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSuggestPerPoolLimitAndBilingual(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinAudience = 3
	cfg.PlansPerPool = 1
	cfg.UsersPerPlan = 1
	cfg.LanguagePairs = [][2]string{{"ja", "en"}}
	users := slices.Concat(fan("x", 3, 3, map[ActivityID]int{"walk": Love, "chess": Like}))
	for _, u := range fan("e", 3, 3, map[ActivityID]int{"walk": Love, "chess": Love}) {
		u.Languages = fluent("en")
		users = append(users, u)
	}
	// ja pool: walk. en pool: chess or walk, tied, chess first by id. The
	// ja+en pool's best (walk) duplicates the ja walk, so it's skipped, and
	// its next best (chess) duplicates the en chess, so nothing bilingual.
	// walk and chess tie on score (3 × love), so chess sorts first by id.
	got := suggestIDs(Suggest(SuggestInput{Users: users, Areas: []AreaID{"a"}, Activities: []Activity{act("walk", 3, 8), act("chess", 3, 8)}}, cfg))
	if want := []string{"chess@a", "walk@a"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSuggestSpacingAndCap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinAudience = 3
	cfg.UsersPerPlan = 1
	// Fans in a and b (15 min apart) and c (40 min away), all within reach
	// of their own area and anything within 20 minutes.
	var users []User
	for _, area := range []AreaID{"a", "b", "c"} {
		for _, u := range fan(string(area), 3, 3, map[ActivityID]int{"walk": Love}) {
			u.HomeArea, u.MaxTravelMin = area, 20
			users = append(users, u)
		}
	}
	in := SuggestInput{Users: users, Areas: testAreas, Travel: testTravel, Activities: []Activity{act("walk", 3, 8)}}
	// a's pool has a and b's fans (6), so it's first; b is within 20 min of
	// a, so it's skipped; c is far enough for its own walk.
	if got, want := suggestIDs(Suggest(in, cfg)), []string{"walk@a", "walk@c"}; !slices.Equal(got, want) {
		t.Errorf("spacing: got %q, want %q", got, want)
	}
	cfg.UsersPerPlan = 9 // 9 people: one plan
	if got := Suggest(in, cfg); len(got) != 1 {
		t.Errorf("cap: got %d plans", len(got))
	}
}

func TestPairs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinAudience, cfg.UsersPerPlan = 3, 1
	boulder := act("bouldering", 3, 8, "gym")
	boulder.Category, boulder.Conversational, boulder.Hours, boulder.Energy = "sports", false, 1.5, 3
	ramen := act("ramen", 3, 6, "cafe")
	ramen.Category, ramen.Hours, ramen.Energy = "food", 1.5, 1
	festival := act("festival", 3, 8, "cafe")
	festival.Category, festival.Hours = "culture", 3
	museum := act("museum", 3, 8, "cafe")
	museum.Category, museum.Hours, museum.Energy = "culture", 2, 1

	suggest := func(users []User, acts ...Activity) []string {
		return suggestIDs(Suggest(SuggestInput{Users: users, Areas: []AreaID{"a"}, Travel: TravelTimes{}, Activities: acts, Venues: testVenues}, cfg))
	}
	// Liking only one half makes a pair less appealing than that half alone:
	// (1.5×2 + 1.5×0.25)/3 × 4 × 1.1 = 4.95 < 4 × 2 × 0.75 = 6.
	if got := suggest(fan("x", 4, 3, map[ActivityID]int{"bouldering": Love}), boulder, ramen); !slices.Equal(got, []string{"bouldering@a"}) {
		t.Errorf("liked alone: %v", got)
	}
	// An activity that fills an outing stands alone, however liked the meal.
	if got := suggest(fan("x", 4, 3, map[ActivityID]int{"festival": Love, "ramen": Love}), festival, ramen); !slices.Contains(got, "festival@a") {
		t.Errorf("a full activity: %v", got)
	}

	// Order: a meal last; otherwise the more active first.
	if !pairs(&boulder, &ramen, cfg) || pairs(&ramen, &boulder, cfg) || !pairs(&boulder, &museum, cfg) || pairs(&museum, &boulder, cfg) {
		t.Error("pair order")
	}
	// Not two of a kind, not over MaxOutingHours.
	museum2 := museum
	museum2.ID = "gallery"
	long := boulder
	long.ID, long.Hours = "long", 3
	if pairs(&museum, &museum2, cfg) || pairs(&long, &museum, cfg) {
		t.Error("pair limits")
	}

	// A pair's groups fit both: the smaller maximum, a shared part of the day.
	boulder.Blocks, ramen.Blocks = []Block{Afternoon, Evening}, []Block{Evening}
	w := boulder.With(&ramen)
	if w.MaxSize != 6 || !slices.Equal(w.Blocks, []Block{Evening}) || w.Hours != 3 || w.ID != boulder.ID || boulder.With(nil) != &boulder {
		t.Errorf("With: %+v", w)
	}

	// Two plans sharing either activity nearby compete for the same people.
	near := TravelTimes{}
	if !tooClose(Plan{Area: "a", Activity: "museum", FollowUp: "ramen", Primary: "ja"}, Plan{Area: "a", Activity: "bouldering", FollowUp: "ramen", Primary: "ja"}, near, cfg) ||
		!tooClose(Plan{Area: "a", Activity: "ramen", Primary: "ja"}, Plan{Area: "a", Activity: "bouldering", FollowUp: "ramen", Primary: "ja"}, near, cfg) ||
		tooClose(Plan{Area: "a", Activity: "museum", Primary: "ja"}, Plan{Area: "a", Activity: "bouldering", FollowUp: "ramen", Primary: "ja"}, near, cfg) {
		t.Error("spacing by either activity")
	}
}

func TestPlanInterest(t *testing.T) {
	cfg := DefaultConfig()
	a, b := act("museum", 3, 8), act("ramen", 3, 8)
	a.Hours, b.Hours = 2, 1
	acts := map[ActivityID]*Activity{"museum": &a, "ramen": &b}
	p := Plan{Activity: "museum", FollowUp: "ramen"}
	u := &User{Budget: 3, Prefs: map[ActivityID]int{"ramen": Love}}
	// Loves only the meal: (2×0.25 + 1×2)/3; their level follows the meal.
	v, best, pref, ok := PlanInterest(u, p, acts, cfg)
	if !ok || pref != Love || best != 2 || v < 0.83 || v > 0.84 {
		t.Errorf("one liked: %v %v %v %v", v, best, pref, ok)
	}
	if _, _, _, ok := PlanInterest(&User{Prefs: map[ActivityID]int{}}, p, acts, cfg); ok {
		t.Error("neither liked")
	}
}
