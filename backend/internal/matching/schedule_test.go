package matching

import (
	"slices"
	"testing"
	"time"
)

var (
	sat = time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	sun = time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	// window is one weekend.
	window   = Occasions(sat, 2)
	walkPlan = Plan{Area: "a", Activity: "walk", Primary: "ja"}
)

// schedule signs everyone up with their profile availability (or the given
// signups) and schedules the walk.
func schedule(t *testing.T, users []User, signups []Signup, rel Relations, cfg Config, force bool) (Scheduled, bool) {
	t.Helper()
	walk := act("walk", 3, 8)
	idx := map[UserID]*User{}
	for i := range users {
		idx[users[i].ID] = &users[i]
		if signups == nil || !slices.ContainsFunc(signups, func(s Signup) bool { return s.User == users[i].ID }) {
			signups = append(signups, Signup{User: users[i].ID})
		}
	}
	got, ok := Schedule(ScheduleInput{Plan: walkPlan, Activity: &walk, Signups: signups, Users: idx, Relations: rel, Window: window}, cfg, force)
	// Invariants for every result: hard rules hold, nobody is placed twice,
	// everyone invited is free then.
	seen := map[UserID]bool{}
	for _, g := range got.Groups {
		var ms []*User
		for _, id := range g.Invited {
			if seen[id] {
				t.Errorf("%s placed twice", id)
			}
			seen[id] = true
			ms = append(ms, idx[id])
			for _, s := range signups {
				if s.User == id && !s.Available(idx[id], got.Occasion) {
					t.Errorf("%s isn't free on %v", id, got.Occasion)
				}
			}
		}
		if err := Violation(ms, rel, cfg); err != nil {
			t.Errorf("group breaks a rule: %v", err)
		}
	}
	return got, ok
}

func sizes(s Scheduled) (invited, quorum []int) {
	for _, g := range s.Groups {
		invited = append(invited, len(g.Invited))
		quorum = append(quorum, g.Quorum)
	}
	return invited, quorum
}

func TestScheduleReadiness(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name        string
		users       []User
		force       bool
		wantReady   bool
		wantInvited []int
		wantQuorum  []int
	}{
		{"enough for the target: ready, with extra invites", people("u", 8, Unspecified), false, true, []int{8}, []int{6}},
		{"short of the target: keep collecting", people("u", 4, Unspecified), false, false, nil, nil},
		{"at the deadline: go ahead smaller", people("u", 4, Unspecified), true, true, []int{4}, []int{2}},
		{"a group of three survives a decline", people("u", 3, Unspecified), true, true, []int{3}, []int{2}},
		{"one person can't be scheduled even at the deadline", people("u", 1, Unspecified), true, false, nil, nil},
		{"two groups, and the rest wait", people("u", 18, Unspecified), false, true, []int{8, 8}, []int{6, 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := schedule(t, tt.users, nil, Relations{}, cfg, tt.force)
			if ok != tt.wantReady {
				t.Fatalf("ready = %v", ok)
			}
			inv, q := sizes(got)
			if !slices.Equal(inv, tt.wantInvited) || !slices.Equal(q, tt.wantQuorum) {
				t.Errorf("invited %v quorum %v; want %v %v", inv, q, tt.wantInvited, tt.wantQuorum)
			}
			if ok && got.Groups[0].MinAttendance != 2 {
				t.Errorf("min attendance %d", got.Groups[0].MinAttendance)
			}
			if len(tt.users) == 18 && len(got.Waiting) != 2 {
				t.Errorf("waiting: %v", got.Waiting)
			}
		})
	}
}

func TestScheduleChoosesTheOccasion(t *testing.T) {
	cfg := DefaultConfig()
	us := people("u", 8, Unspecified)

	// Everyone is free both weekend afternoons: the earlier one wins.
	both := []Slot{{time.Saturday, Afternoon}, sunPM}
	var signups []Signup
	for _, u := range us {
		signups = append(signups, Signup{User: u.ID, Pattern: both})
	}
	got, ok := schedule(t, us, signups, Relations{}, cfg, false)
	if !ok || !got.Occasion.Date.Equal(sat) || got.Occasion.Block != Afternoon {
		t.Errorf("ties go to the earlier occasion: got %v, %v", got.Occasion, ok)
	}

	// Two people block out Saturday: Sunday has more people.
	signups[0].Unavailable = []Occasion{{Date: sat}}
	signups[1].Unavailable = []Occasion{{Date: sat}}
	got, _ = schedule(t, us, signups, Relations{}, cfg, false)
	if !got.Occasion.Date.Equal(sun) {
		t.Errorf("blocked-out dates: got %v", got.Occasion)
	}

	// An evening-only activity can't use afternoon availability.
	walk := act("walk", 3, 8)
	walk.Blocks = []Block{Evening}
	idx := map[UserID]*User{}
	for i := range us {
		idx[us[i].ID] = &us[i]
	}
	if _, ok := Schedule(ScheduleInput{Plan: walkPlan, Activity: &walk, Signups: signups, Users: idx, Window: window}, cfg, true); ok {
		t.Error("scheduled outside the activity's parts of the day")
	}
}

func TestScheduleRules(t *testing.T) {
	cfg := DefaultConfig()
	t.Run("a lone woman is still invited rather than left out", func(t *testing.T) {
		got, _ := schedule(t, append(people("m", 5, Man), people("w", 1, Woman)...), nil, Relations{}, cfg, true)
		if len(got.Waiting) != 0 {
			t.Errorf("left out: %v", got.Waiting)
		}
	})
	t.Run("same-gender only is a hard rule", func(t *testing.T) {
		got, _ := schedule(t, append(people("m", 3, Man), sameGenderOnly(people("w", 1, Woman))...), nil, Relations{}, cfg, true)
		if !slices.Equal(got.Waiting, []UserID{"wa"}) {
			t.Errorf("waiting %v, groups %+v", got.Waiting, got.Groups)
		}
	})
	t.Run("blocked pairs end up apart", func(t *testing.T) {
		// schedule() fails the test if they're grouped together.
		schedule(t, people("u", 16, Unspecified), nil, Relations{Blocked: map[Pair]bool{MakePair("ua", "ub"): true}}, cfg, false)
	})
	t.Run("people who don't speak the language are ignored", func(t *testing.T) {
		us := people("u", 8, Unspecified)
		us[0].Languages = fluent("es")
		got, _ := schedule(t, us, nil, Relations{}, cfg, true)
		if !slices.Equal(got.Ineligible, []UserID{"ua"}) {
			t.Errorf("ineligible: %v", got.Ineligible)
		}
	})
	t.Run("one known pair is grouped together", func(t *testing.T) {
		c := cfg
		c.ExtraInvites = 0
		c.SizeTargets = map[SizePref]int{Small: 3, Medium: 3, Large: 3}
		got, _ := schedule(t, people("p", 6, Unspecified), nil, Relations{Mutual: map[Pair]float64{MakePair("pa", "pb"): 1}}, c, false)
		together := false
		for _, g := range got.Groups {
			together = together || (slices.Contains(g.Invited, "pa") && slices.Contains(g.Invited, "pb"))
		}
		if len(got.Groups) != 2 || !together {
			t.Errorf("mutual pair not grouped together: %+v", got.Groups)
		}
	})
}
