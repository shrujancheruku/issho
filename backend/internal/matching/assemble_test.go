package matching

import (
	"strings"
	"testing"
)

// person makes a user in area a, free on Sunday afternoons, fluent in
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

func sameGenderOnly(us []User) []User {
	for i := range us {
		us[i].SameGenderOnly = true
	}
	return us
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
		{"the only man is a preference, not a rule", []*User{&w1, &w2, &m1, &nb}, ""},
		{"a pair is allowed", []*User{&w1, &m1}, ""},
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

// The preferences count in the score: balance and the preferred minimum.
func TestScorePreferences(t *testing.T) {
	cfg := DefaultConfig()
	w1, w2, m1, m2 := person("w1", Woman), person("w2", Woman), person("m1", Man), person("m2", Man)
	w3 := person("w3", Woman)
	walk := act("walk", 3, 8)
	g := &group{act: &walk, target: 4, interestOf: func(*User) float64 { return 2 }}
	if lone, balanced := g.score([]*User{&w1, &m1, &m2}, Relations{}, cfg), g.score([]*User{&w1, &w2, &m1, &m2}, Relations{}, cfg); lone >= balanced {
		t.Errorf("lone woman %.2f should score below balanced %.2f", lone, balanced)
	}
	if pair, trio := g.score([]*User{&w1, &w2}, Relations{}, cfg), g.score([]*User{&w1, &w2, &w3}, Relations{}, cfg); pair >= trio {
		t.Errorf("pair %.2f should score below trio %.2f", pair, trio)
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
	for _, raw := range []string{`{"wieghts": {}}`, `{"min_group_size": 1}`, `{"language_pairs": [["ja", "ja"]]}`} {
		if _, err := ParseConfig([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", raw)
		}
	}
	if _, err := ParseConfig([]byte(`{}`)); err != nil {
		t.Errorf("empty overrides: %v", err)
	}
}
