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
	boulder.Conversational = false
	ramen := act("ramen", 3, 8, "cafe")
	ramen.Category = "food"
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
			name:  "a non-conversational activity gets a food follow-up",
			users: fan("x", 4, 3, map[ActivityID]int{"bouldering": Love, "ramen": Like}),
			acts:  []Activity{boulder, ramen},
			want:  []string{"bouldering>ramen@a", "ramen@a"},
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
