package matching

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

// Three areas: a and b are 15 minutes apart; c is 40 minutes from both.
var (
	testAreas  = []AreaID{"a", "b", "c"}
	testTravel = TravelTimes{
		{"a", "b"}: 15, {"b", "a"}: 15,
		{"a", "c"}: 40, {"c", "a"}: 40,
		{"b", "c"}: 40, {"c", "b"}: 40,
	}
	sunPM = Slot{time.Sunday, Afternoon}
	satAM = Slot{time.Saturday, Morning}
)

func fluent(codes ...string) []Language {
	var ls []Language
	for _, c := range codes {
		ls = append(ls, Language{Code: c, Fluent: true})
	}
	return ls
}

func learning(code string, level int) Language { return Language{Code: code, Level: level} }

func user(id string, home AreaID, maxMin int, langs []Language) User {
	return User{ID: UserID(id), HomeArea: home, MaxTravelMin: maxMin, Languages: langs}
}

// describe renders pools compactly so expectations read as one line each.
func describe(ps []Pool) []string {
	var out []string
	for _, p := range ps {
		lang := p.Primary
		if p.Bilingual() {
			lang += "+" + p.Secondary
		}
		out = append(out, fmt.Sprintf("%s %s %v", p.Area, lang, p.Members))
	}
	return out
}

func TestPools(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name  string
		users []User
		want  []string
	}{
		{
			name: "neighbours share every area they can all reach",
			users: []User{
				user("u1", "a", 20, fluent("ja")),
				user("u2", "b", 20, fluent("ja")),
			},
			want: []string{"a ja [u1 u2]", "b ja [u1 u2]"},
		},
		{
			name: "max travel time is respected; one person isn't a pool",
			users: []User{
				user("u1", "a", 30, fluent("ja")),
				user("u2", "a", 30, fluent("ja")),
				user("u3", "c", 30, fluent("ja")), // c is 40 min away
			},
			want: []string{"a ja [u1 u2]", "b ja [u1 u2]"},
		},
		{
			name: "a shared fluent language, any language; learners don't count",
			users: []User{
				user("u1", "a", 0, fluent("es", "ja")),
				user("u2", "a", 0, fluent("es")),
				user("u3", "a", 0, []Language{{Code: "es", Fluent: true}, learning("ja", 3)}),
				user("u4", "a", 0, fluent("ja")),
			},
			want: []string{"a es [u1 u2 u3]", "a ja [u1 u4]"},
		},
		{
			name: "no route: unreachable even with a generous limit",
			users: []User{
				user("u1", "a", 180, fluent("ja")),
				user("u2", "z", 180, fluent("ja")), // z isn't in the matrix
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describe(Pools(tt.users, testAreas, testTravel, cfg))
			if !slices.Equal(got, tt.want) {
				t.Errorf("got\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func TestPoolsLanguagePairs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LanguagePairs = [][2]string{{"en", "ja"}}
	users := []User{
		user("ja1", "a", 0, fluent("ja")),
		user("ja2", "a", 0, []Language{{Code: "ja", Fluent: true}, learning("en", 1)}),
		user("ja3", "a", 0, fluent("ja")),
		user("en1", "a", 0, fluent("en")),
		user("bi", "a", 0, fluent("en", "ja")),
		user("es1", "a", 0, fluent("es")), // speaks neither: never in the pair's pool
	}
	// Anyone fluent in either language joins; ja has more speakers (4 vs 2),
	// so it's the primary language even though the pair lists en first.
	want := []string{"a en [bi en1]", "a ja [bi ja1 ja2 ja3]", "a ja+en [bi en1 ja1 ja2 ja3]"}
	if got := describe(Pools(users, testAreas, testTravel, cfg)); !slices.Equal(got, want) {
		t.Errorf("got\n  %q\nwant\n  %q", got, want)
	}
}

func TestPoolsDeterministic(t *testing.T) {
	var users []User
	for i := range 30 {
		users = append(users, user(fmt.Sprintf("u%02d", i), testAreas[i%3], 40, fluent("ja", "en")))
	}
	first := describe(Pools(users, testAreas, testTravel, DefaultConfig()))
	for range 20 {
		if got := describe(Pools(users, testAreas, testTravel, DefaultConfig())); !slices.Equal(got, first) {
			t.Fatal("Pools output changed between runs")
		}
	}
}

func TestTravelTimesMinutes(t *testing.T) {
	if m, ok := testTravel.Minutes("c", "c"); !ok || m != 0 {
		t.Errorf("same area: got %d, %v", m, ok)
	}
	if _, ok := testTravel.Minutes("a", "z"); ok {
		t.Error("missing pair reported as reachable")
	}
}

func TestSignupAvailable(t *testing.T) {
	sun := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC) // a Sunday
	u := &User{Availability: []Slot{sunPM, satAM}}
	tests := []struct {
		name string
		s    Signup
		o    Occasion
		want bool
	}{
		{"profile availability by default", Signup{}, Occasion{sun, Afternoon}, true},
		{"not in the profile", Signup{}, Occasion{sun, Evening}, false},
		{"a pattern for this plan replaces the profile", Signup{Pattern: []Slot{{time.Sunday, Evening}}}, Occasion{sun, Evening}, true},
		{"the profile no longer applies", Signup{Pattern: []Slot{{time.Sunday, Evening}}}, Occasion{sun, Afternoon}, false},
		{"a blocked-out date", Signup{Unavailable: []Occasion{{Date: sun}}}, Occasion{sun, Afternoon}, false},
		{"a blocked-out occasion leaves the rest of the day", Signup{Pattern: []Slot{sunPM, {time.Sunday, Evening}}, Unavailable: []Occasion{{sun, Evening}}}, Occasion{sun, Afternoon}, true},
		{"the same weekday a week later", Signup{Unavailable: []Occasion{{Date: sun}}}, Occasion{sun.AddDate(0, 0, 7), Afternoon}, true},
	}
	for _, tt := range tests {
		if got := tt.s.Available(u, tt.o); got != tt.want {
			t.Errorf("%s: got %v", tt.name, got)
		}
	}
}
