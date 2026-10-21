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

func user(id string, home AreaID, maxMin int, langs []Language, slots ...Slot) User {
	return User{ID: UserID(id), HomeArea: home, MaxTravelMin: maxMin, Languages: langs, Availability: slots}
}

// describe renders buckets compactly so expectations read as one line each.
func describe(bs []Bucket) []string {
	var out []string
	for _, b := range bs {
		lang := b.Primary
		if b.IsExchange() {
			lang += "+" + b.Secondary
		}
		out = append(out, fmt.Sprintf("%s %s@%s %s %v", b.Slot.Weekday.String()[:3], b.Slot.Block, b.Area, lang, b.Members))
	}
	return out
}

func TestBuckets(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		name  string
		users []User
		want  []string
	}{
		{
			name: "neighbours share every area they can all reach",
			users: []User{
				user("u1", "a", 20, fluent("ja"), sunPM),
				user("u2", "a", 20, fluent("ja"), sunPM),
				user("u3", "b", 20, fluent("ja"), sunPM),
			},
			want: []string{
				"Sun afternoon@a ja [u1 u2 u3]",
				"Sun afternoon@b ja [u1 u2 u3]",
			},
		},
		{
			name: "too far: max travel time is respected",
			users: []User{
				user("u1", "a", 30, fluent("ja"), sunPM),
				user("u2", "a", 30, fluent("ja"), sunPM),
				user("u3", "c", 30, fluent("ja"), sunPM), // c is 40 min away
			},
			want: nil,
		},
		{
			name: "only overlapping slots",
			users: []User{
				user("u1", "a", 0, fluent("ja"), sunPM, satAM),
				user("u2", "a", 0, fluent("ja"), sunPM),
				user("u3", "a", 0, fluent("ja"), sunPM, satAM),
			},
			want: []string{"Sun afternoon@a ja [u1 u2 u3]"},
		},
		{
			name: "a shared fluent language, any language; learners don't count",
			users: []User{
				user("u1", "a", 0, fluent("es", "ja"), sunPM),
				user("u2", "a", 0, fluent("es"), sunPM),
				user("u3", "a", 0, []Language{{Code: "es", Fluent: true}, learning("ja", 3)}, sunPM),
				user("u4", "a", 0, fluent("ja"), sunPM),
			},
			want: []string{"Sun afternoon@a es [u1 u2 u3]"},
		},
		{
			name: "language exchange: complementary pairs, primary is the bigger side",
			users: []User{
				user("ja1", "a", 0, []Language{{Code: "ja", Fluent: true}, learning("es", 1)}, sunPM),
				user("ja2", "a", 0, []Language{{Code: "ja", Fluent: true}, learning("es", 2)}, sunPM),
				user("ja3", "a", 0, []Language{{Code: "ja", Fluent: true}, learning("es", 2)}, sunPM),
				user("es1", "a", 0, []Language{{Code: "es", Fluent: true}, learning("ja", 1)}, sunPM),
				user("es2", "a", 0, []Language{{Code: "es", Fluent: true}, learning("ja", 3)}, sunPM),
				// Bilingual: in both regular buckets, not in the exchange.
				user("bi", "a", 0, fluent("es", "ja"), sunPM),
			},
			want: []string{
				"Sun afternoon@a es [bi es1 es2]",
				"Sun afternoon@a ja [bi ja1 ja2 ja3]",
				"Sun afternoon@a ja+es [es1 es2 ja1 ja2 ja3]",
			},
		},
		{
			name: "exchange needs enough people on each side",
			users: []User{
				user("ja1", "a", 0, []Language{{Code: "ja", Fluent: true}, learning("es", 1)}, sunPM),
				user("ja2", "a", 0, []Language{{Code: "ja", Fluent: true}, learning("es", 1)}, sunPM),
				user("ja3", "a", 0, []Language{{Code: "ja", Fluent: true}, learning("es", 1)}, sunPM),
				user("es1", "a", 0, []Language{{Code: "es", Fluent: true}, learning("ja", 1)}, sunPM),
			},
			want: []string{"Sun afternoon@a ja [ja1 ja2 ja3]"},
		},
		{
			name: "no route: unreachable even with a generous limit",
			users: []User{
				user("u1", "a", 180, fluent("ja"), sunPM),
				user("u2", "a", 180, fluent("ja"), sunPM),
				user("u3", "z", 180, fluent("ja"), sunPM), // z isn't in the matrix
			},
			want: nil, // u3 is in no bucket, leaving u1 and u2: too few anywhere
		},
		{
			name: "sorted by day, then chronological block",
			users: []User{
				user("u1", "a", 0, fluent("ja"), Slot{time.Monday, Evening}, Slot{time.Monday, Morning}, sunPM),
				user("u2", "a", 0, fluent("ja"), Slot{time.Monday, Evening}, Slot{time.Monday, Morning}, sunPM),
				user("u3", "a", 0, fluent("ja"), Slot{time.Monday, Evening}, Slot{time.Monday, Morning}, sunPM, sunPM),
			},
			want: []string{
				"Sun afternoon@a ja [u1 u2 u3]",
				"Mon morning@a ja [u1 u2 u3]",
				"Mon evening@a ja [u1 u2 u3]",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describe(Buckets(tt.users, testAreas, testTravel, cfg))
			if !slices.Equal(got, tt.want) {
				t.Errorf("got\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func TestBucketsDeterministic(t *testing.T) {
	var users []User
	for i := range 30 {
		users = append(users, user(fmt.Sprintf("u%02d", i), testAreas[i%3], 40, fluent("ja", "en"), sunPM, satAM))
	}
	first := describe(Buckets(users, testAreas, testTravel, DefaultConfig()))
	for range 20 {
		if got := describe(Buckets(users, testAreas, testTravel, DefaultConfig())); !slices.Equal(got, first) {
			t.Fatal("Buckets output changed between runs")
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
