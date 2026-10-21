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
	planVenues = Venues{"a": {"cafe": 3, "bar": 1, "gym": 2}}
)

// bucketOf makes a bucket in area a on Sunday afternoon with the given users.
func bucketOf(us ...User) (Bucket, map[UserID]*User) {
	b := Bucket{Area: "a", Slot: sunPM, Primary: "ja"}
	idx := map[UserID]*User{}
	for i := range us {
		b.Members = append(b.Members, us[i].ID)
		idx[us[i].ID] = &us[i]
	}
	return b, idx
}

// fan makes n users with the same preferences and budget.
func fan(prefix string, n, budget int, prefs map[ActivityID]int) []User {
	var us []User
	for i := range n {
		us = append(us, User{ID: UserID(fmt.Sprintf("%s%d", prefix, i)), Budget: budget, Prefs: prefs, Gender: Unspecified, Size: Medium, Reliability: 1})
	}
	return us
}

func planIDs(ps []Plan) []string {
	var out []string
	for _, p := range ps {
		s := string(p.Activity)
		if p.FollowUp != "" {
			s += ">" + string(p.FollowUp)
		}
		out = append(out, s)
	}
	return out
}

func TestPlans(t *testing.T) {
	cfg := DefaultConfig()
	evening := act("bar-hop", 3, 8, "bar")
	evening.Blocks = []Block{Evening}
	pricey := act("omakase", 3, 6, "cafe")
	pricey.CostLevel = 3
	boulder := act("bouldering", 3, 8, "gym")
	boulder.Conversational = false
	ramen := act("ramen", 3, 8, "cafe")
	ramen.Category = "food"

	tests := []struct {
		name  string
		users []User
		acts  []Activity
		want  []string
	}{
		{
			name:  "ranked by demand: loves beat likes",
			users: slices.Concat(fan("x", 3, 3, map[ActivityID]int{"walk": Love, "cafe-hop": Like})),
			acts:  []Activity{act("cafe-hop", 3, 8, "cafe"), act("walk", 3, 8)},
			want:  []string{"walk", "cafe-hop"},
		},
		{
			name:  "not enough interested for the minimum size",
			users: slices.Concat(fan("x", 3, 3, map[ActivityID]int{"walk": Love}), fan("y", 3, 3, map[ActivityID]int{"futsal": Love})),
			acts:  []Activity{act("walk", 3, 8), act("futsal", 4, 10)},
			want:  []string{"walk"},
		},
		{
			name:  "no venue in the area, or the wrong part of the day",
			users: fan("x", 4, 3, map[ActivityID]int{"onsen": Love, "bar-hop": Love, "walk": OpenToTry}),
			acts:  []Activity{act("onsen", 3, 8, "onsen"), evening, act("walk", 3, 8)},
			want:  []string{"walk"},
		},
		{
			name:  "few venues and over-budget interest score lower",
			users: fan("x", 4, 1, map[ActivityID]int{"omakase": Love, "pub": Like, "walk": Like}),
			// omakase: 4 × 2 × 0.3 = 2.4; pub (1 of 3 venues): 4 × 1 / 3 = 1.33; walk: 4.
			acts: []Activity{pricey, act("pub", 3, 8, "bar"), act("walk", 3, 8)},
			want: []string{"walk", "omakase", "pub"},
		},
		{
			name:  "a non-conversational activity gets a food follow-up",
			users: fan("x", 4, 3, map[ActivityID]int{"bouldering": Love, "ramen": Like}),
			acts:  []Activity{boulder, ramen},
			want:  []string{"bouldering>ramen", "ramen"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, idx := bucketOf(tt.users...)
			if got := planIDs(Plans(b, idx, tt.acts, planVenues, cfg)); !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlansKeepsTheBest(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PlansPerBucket = 2
	prefs := map[ActivityID]int{"a1": Love, "a2": Like, "a3": OpenToTry}
	b, idx := bucketOf(fan("x", 3, 3, prefs)...)
	acts := []Activity{act("a3", 3, 8), act("a1", 3, 8), act("a2", 3, 8)}
	if got := planIDs(Plans(b, idx, acts, nil, cfg)); !slices.Equal(got, []string{"a1", "a2"}) {
		t.Errorf("got %q", got)
	}
}
