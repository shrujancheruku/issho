package matching

import (
	"slices"
	"strings"
	"testing"
)

// person makes a user in area a, free on Sunday afternoon, fluent in
// Japanese and keen on walks.
func person(id string, g Gender) User {
	return User{
		ID: UserID(id), HomeArea: "a", MaxTravelMin: 0, Languages: fluent("ja"),
		Availability: []Slot{sunPM}, Gender: g, Size: Medium, Budget: 3,
		Prefs: map[ActivityID]int{"walk": Love}, Reliability: 1, EventsAttended: 1,
	}
}

func people(prefix string, n int, g Gender) []User {
	var us []User
	for i := range n {
		us = append(us, person(prefix+string(rune('a'+i)), g))
	}
	return us
}

func runWalk(users []User, rel Relations, cfg Config) []Proposal {
	return Run(Input{
		Users: users, Areas: []AreaID{"a"}, Travel: TravelTimes{},
		Activities: []Activity{act("walk", 3, 8)}, Relations: rel,
	}, cfg)
}

func asUsers(all []User, ids []UserID) []*User {
	var out []*User
	for _, id := range ids {
		for i := range all {
			if all[i].ID == id {
				out = append(out, &all[i])
			}
		}
	}
	return out
}

func TestViolation(t *testing.T) {
	cfg := DefaultConfig()
	w1, w2, m1, m2 := person("w1", Woman), person("w2", Woman), person("m1", Man), person("m2", Man)
	m3, w3, nb := person("m3", Man), person("w3", Woman), person("nb", Nonbinary)
	sgo := person("sgo", Woman)
	sgo.SameGenderOnly = true
	rel := Relations{
		Blocked: map[Pair]bool{MakePair("m2", "w1"): true},
		Mutual:  map[Pair]float64{MakePair("w2", "m1"): 1, MakePair("m1", "m2"): 1},
	}
	tests := []struct {
		name    string
		members []*User
		want    string // substring of the error; "" = valid
	}{
		{"mixed with two of each", []*User{&w1, &w2, &m1, &nb}, "only man"},
		{"two women, two men", []*User{&w1, &w2, &m1, &m3}, ""},
		{"a lone nonbinary person is allowed", []*User{&w1, &w2, &nb}, ""},
		{"the only woman", []*User{&w1, &m1, &m3}, "only woman"},
		{"blocked", []*User{&w1, &w3, &m2, &m3}, "blocked"},
		{"same-gender only", []*User{&sgo, &w1, &m1, &m3}, "same-gender"},
		{"same-gender only, all women", []*User{&sgo, &w1, &w2}, ""},
		{"one known pair is fine", []*User{&w2, &m1, &w1, &m3}, ""},
		{"three people who know each other", []*User{&w2, &m1, &m2, &w3}, "know each other"},
	}
	for _, tt := range tests {
		err := Violation(tt.members, rel, cfg)
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("%s: unexpected %v", tt.name, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("%s: got %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestRunGroupsAndRules(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name      string
		users     []User
		rel       Relations
		proposals int
		check     func(t *testing.T, ps []Proposal)
	}{
		{
			name:      "target size plus extra invites, confirmed at the target",
			users:     people("u", 8, Unspecified),
			proposals: 1,
			check: func(t *testing.T, ps []Proposal) {
				if len(ps[0].Invited) != 8 || ps[0].Quorum != 6 {
					t.Errorf("got %d invited, quorum %d", len(ps[0].Invited), ps[0].Quorum)
				}
			},
		},
		{
			name:      "fewer invites lower the quorum, not below the minimum",
			users:     people("u", 4, Unspecified),
			proposals: 1,
			check: func(t *testing.T, ps []Proposal) {
				if len(ps[0].Invited) != 4 || ps[0].Quorum != 3 {
					t.Errorf("got %d invited, quorum %d", len(ps[0].Invited), ps[0].Quorum)
				}
			},
		},
		{
			name:      "a lone woman is not proposed into a group of men",
			users:     slices.Concat(people("m", 5, Man), people("w", 1, Woman)),
			proposals: 1,
			check: func(t *testing.T, ps []Proposal) {
				if slices.Contains(ps[0].Invited, "wa") {
					t.Errorf("lone woman placed: %v", ps[0].Invited)
				}
			},
		},
		{
			name:      "too few people once the rules apply",
			users:     slices.Concat(people("m", 1, Man), people("w", 2, Woman)),
			proposals: 0,
		},
		{
			name:  "blocked pairs end up apart",
			users: people("u", 16, Unspecified), // two groups of 6 + 2
			rel:   Relations{Blocked: map[Pair]bool{MakePair("ua", "ub"): true}},
			check: func(t *testing.T, ps []Proposal) {
				for _, p := range ps {
					if slices.Contains(p.Invited, "ua") && slices.Contains(p.Invited, "ub") {
						t.Errorf("blocked pair together: %v", p.Invited)
					}
				}
			},
			proposals: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := runWalk(tt.users, tt.rel, cfg)
			if len(ps) != tt.proposals {
				t.Fatalf("got %d proposals, want %d: %+v", len(ps), tt.proposals, ps)
			}
			seen := map[UserID]bool{}
			for _, p := range ps {
				for _, id := range p.Invited {
					if seen[id] {
						t.Errorf("%s placed twice", id)
					}
					seen[id] = true
				}
				if err := Violation(asUsers(tt.users, p.Invited), tt.rel, cfg); err != nil {
					t.Errorf("proposal breaks a rule: %v", err)
				}
			}
			if tt.check != nil {
				tt.check(t, ps)
			}
		})
	}
}

func TestRunPrefersOneKnownPair(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExtraInvites = 0
	cfg.SizeTargets = map[SizePref]int{Small: 3, Medium: 3, Large: 3}
	// Six people, two groups of three. pa and pb picked each other before.
	users := people("p", 6, Unspecified)
	rel := Relations{Mutual: map[Pair]float64{MakePair("pa", "pb"): 1}}
	ps := runWalk(users, rel, cfg)
	together := false
	for _, p := range ps {
		together = together || (slices.Contains(p.Invited, "pa") && slices.Contains(p.Invited, "pb"))
	}
	if len(ps) != 2 || !together {
		t.Errorf("mutual pair not grouped together: %+v", ps)
	}
}

func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig([]byte(`{"weights": {"mutual": 2}, "size_targets": {"small": 3}, "language_pairs": [["ja", "en"]]}`))
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultConfig()
	if cfg.Weights.Mutual != 2 || cfg.Weights.Interest != def.Weights.Interest {
		t.Errorf("weights not merged: %+v", cfg.Weights)
	}
	if cfg.SizeTargets[Small] != 3 || cfg.SizeTargets[Large] != def.SizeTargets[Large] {
		t.Errorf("size targets not merged: %v", cfg.SizeTargets)
	}
	if len(cfg.LanguagePairs) != 1 {
		t.Errorf("language pairs: %v", cfg.LanguagePairs)
	}
	for _, raw := range []string{`{"wieghts": {}}`, `{"min_bucket_size": 1}`, `{"min_group_size": 2}`, `{"language_pairs": [["ja", "ja"]]}`} {
		if _, err := ParseConfig([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", raw)
		}
	}
	if _, err := ParseConfig([]byte(`{}`)); err != nil {
		t.Errorf("empty overrides: %v", err)
	}
}
