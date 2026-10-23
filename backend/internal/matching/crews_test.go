package matching

import (
	"reflect"
	"testing"
)

func TestFindCrews(t *testing.T) {
	football := map[ActivityID]int{"football": Love, "ramen": Like}
	user := func(id UserID, area AreaID, prefs map[ActivityID]int) User {
		return User{ID: id, HomeArea: area, Prefs: prefs}
	}
	pairs := func(ps ...[2]UserID) map[Pair]bool {
		m := map[Pair]bool{}
		for _, p := range ps {
			m[MakePair(p[0], p[1])] = true
		}
		return m
	}
	links := func(ps ...[2]UserID) map[Pair]float64 {
		m := map[Pair]float64{}
		for _, p := range ps {
			m[MakePair(p[0], p[1])] = 1
		}
		return m
	}
	abc := []User{user("a", "kichijoji", football), user("b", "kichijoji", football), user("c", "kichijoji", football)}
	// A met B and C at different outings, and B met C at a third.
	met := pairs([2]UserID{"a", "b"}, [2]UserID{"a", "c"}, [2]UserID{"b", "c"})
	mutual := links([2]UserID{"a", "b"}, [2]UserID{"b", "c"})
	offered := []ActivityID{"football", "ramen", "walk"}
	footballCrew := []Crew{{Members: []UserID{"a", "b", "c"}, Basis: CrewBasis{Activity: "football"}}}

	tests := []struct {
		name  string
		users []User
		g     CrewGraph
		rel   Relations
		want  []Crew
	}{
		{"met at different outings, all like football", abc, CrewGraph{Met: met}, Relations{Mutual: mutual}, footballCrew},
		{"two of them never met", abc, CrewGraph{Met: pairs([2]UserID{"a", "b"}, [2]UserID{"b", "c"})}, Relations{Mutual: mutual}, nil},
		{"someone without a mutual pick", abc, CrewGraph{Met: met}, Relations{Mutual: links([2]UserID{"a", "b"})}, nil},
		{
			// D met everyone and has mutual picks, but C would rather not see
			// D again: D is left out, and A-B-D overlaps the A-B-C crew.
			"someone minded is left out",
			append(abc, user("d", "kichijoji", football)),
			CrewGraph{Met: pairs([2]UserID{"a", "b"}, [2]UserID{"a", "c"}, [2]UserID{"b", "c"}, [2]UserID{"a", "d"}, [2]UserID{"b", "d"}, [2]UserID{"c", "d"})},
			Relations{Mutual: links([2]UserID{"a", "b"}, [2]UserID{"b", "c"}, [2]UserID{"a", "d"}), Blocked: pairs([2]UserID{"c", "d"})},
			footballCrew,
		},
		{"declined a crew with someone", abc, CrewGraph{Met: met, Averse: pairs([2]UserID{"a", "c"})}, Relations{Mutual: mutual}, nil},
		{
			"everyone met everyone: all four",
			append(abc, user("d", "kichijoji", football)),
			CrewGraph{Met: pairs([2]UserID{"a", "b"}, [2]UserID{"a", "c"}, [2]UserID{"b", "c"}, [2]UserID{"a", "d"}, [2]UserID{"b", "d"}, [2]UserID{"c", "d"})},
			Relations{Mutual: links([2]UserID{"a", "b"}, [2]UserID{"c", "d"})},
			[]Crew{{Members: []UserID{"a", "b", "c", "d"}, Basis: CrewBasis{Activity: "football"}}},
		},
		{
			"no shared activity: the shared area, everyone in it",
			append(abc[:2:2], user("c", "kichijoji", map[ActivityID]int{"walk": Love})),
			CrewGraph{Met: met}, Relations{Mutual: mutual},
			[]Crew{{Members: []UserID{"a", "b", "c"}, Basis: CrewBasis{Area: "kichijoji"}}},
		},
		{
			"nothing in common",
			append(abc[:2:2], user("c", "koenji", map[ActivityID]int{"walk": Love})),
			CrewGraph{Met: met}, Relations{Mutual: mutual}, nil,
		},
		{
			"open to try isn't liking",
			[]User{user("a", "x", map[ActivityID]int{"walk": OpenToTry}), user("b", "y", map[ActivityID]int{"walk": OpenToTry}), user("c", "z", map[ActivityID]int{"walk": OpenToTry})},
			CrewGraph{Met: met}, Relations{Mutual: mutual}, nil,
		},
		{"an activity no longer offered", abc, CrewGraph{Met: met}, Relations{Mutual: mutual},
			[]Crew{{Members: []UserID{"a", "b", "c"}, Basis: CrewBasis{Activity: "ramen"}}}},
		{"someone who moved away (not in the city's users)", abc[:2], CrewGraph{Met: met}, Relations{Mutual: mutual}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acts := offered
			if tt.name == "an activity no longer offered" {
				acts = []ActivityID{"ramen"}
			}
			got := FindCrews(tt.users, tt.g, tt.rel, acts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindCrews = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMaximalCliques(t *testing.T) {
	adj := map[UserID]map[UserID]bool{}
	edge := func(a, b UserID) {
		for _, e := range [][2]UserID{{a, b}, {b, a}} {
			if adj[e[0]] == nil {
				adj[e[0]] = map[UserID]bool{}
			}
			adj[e[0]][e[1]] = true
		}
	}
	// A triangle a-b-c, plus c-d.
	edge("a", "b")
	edge("b", "c")
	edge("a", "c")
	edge("c", "d")
	got := maximalCliques([]UserID{"a", "b", "c", "d"}, adj)
	want := [][]UserID{{"a", "b", "c"}, {"c", "d"}}
	if len(got) != 2 || !reflect.DeepEqual(got[0], want[0]) && !reflect.DeepEqual(got[1], want[0]) {
		t.Errorf("cliques = %v, want %v", got, want)
	}
}
