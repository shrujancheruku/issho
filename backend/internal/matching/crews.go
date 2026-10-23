package matching

import (
	"slices"
	"strings"
)

// CrewMinSize is the fewest people a crew is offered to. The bar for a
// crew is deliberately high: it's a standing group chat, and one that
// doesn't click is worse than none.
const CrewMinSize = 3

// CrewGraph is who has met whom, and who shouldn't be offered a crew
// together, beyond Relations.
type CrewGraph struct {
	// Met holds pairs who both said they went to the same outing.
	Met map[Pair]bool
	// Averse holds pairs where one declined or left a crew the other was in.
	Averse map[Pair]bool
}

// CrewBasis is what a crew has in common, which names it: an activity
// they all like ("Football crew") or the area they all live in ("Kichijoji
// crew"). Exactly one is set.
type CrewBasis struct {
	Activity ActivityID
	Area     AreaID
}

// Crew is a crew to offer: its members (sorted) and what they share.
type Crew struct {
	Members []UserID
	Basis   CrewBasis
}

// FindCrews finds groups of people to offer a crew, from the relationships
// between them. Two people are compatible when they've met at an outing
// and neither minded the other: no "rather not" or report (Blocked), and
// neither declined or left a crew with the other (Averse). A crew is a set
// of at least CrewMinSize people who are all compatible with each other
// (a clique, so everyone has met everyone, possibly at different outings),
// where each has a mutual meet-again pick with someone else in it, and who
// all share something: an activity in `offered` they all like or love, or
// their home area. Someone who anyone in a set minded is simply left out of
// that set.
//
// For each largest such set (maximal clique) the best crew in it is
// offered: the most people, then an activity over an area, then the
// strongest shared liking, then by id. Of crews on the same basis sharing
// two or more people, only the best is kept. The result is sorted and
// doesn't depend on input order.
func FindCrews(users []User, g CrewGraph, rel Relations, offered []ActivityID) []Crew {
	byID := make(map[UserID]*User, len(users))
	for i := range users {
		byID[users[i].ID] = &users[i]
	}
	compatible := func(a, b UserID) bool {
		p := MakePair(a, b)
		return g.Met[p] && !rel.Blocked[p] && !g.Averse[p]
	}
	// Only people with a compatible mutual pick can be in a crew.
	adj := map[UserID]map[UserID]bool{}
	for p := range g.Met {
		if byID[p.A] == nil || byID[p.B] == nil || !compatible(p.A, p.B) {
			continue
		}
		for _, e := range [][2]UserID{{p.A, p.B}, {p.B, p.A}} {
			if adj[e[0]] == nil {
				adj[e[0]] = map[UserID]bool{}
			}
			adj[e[0]][e[1]] = true
		}
	}
	mutual := func(a, b UserID) bool { return rel.Mutual[MakePair(a, b)] > 0 }
	var nodes []UserID
	for u, ns := range adj {
		for n := range ns {
			if mutual(u, n) {
				nodes = append(nodes, u)
				break
			}
		}
	}
	slices.Sort(nodes)

	var crews []Crew
	for _, clique := range maximalCliques(nodes, adj) {
		if len(clique) < CrewMinSize {
			continue
		}
		if c, ok := bestCrew(clique, byID, offered, mutual); ok {
			crews = append(crews, c)
		}
	}
	return dedupeCrews(crews)
}

// bestCrew picks the best crew within a clique (see FindCrews).
func bestCrew(clique []UserID, byID map[UserID]*User, offered []ActivityID, mutual func(a, b UserID) bool) (Crew, bool) {
	var best Crew
	bestWeight := 0
	consider := func(members []UserID, basis CrewBasis, weight int) {
		members = withMutualPicks(members, mutual)
		if len(members) < CrewMinSize {
			return
		}
		better := len(members) > len(best.Members)
		if len(members) == len(best.Members) {
			switch {
			case basis.Activity != "" && best.Basis.Activity == "":
				better = true
			case (basis.Activity != "") == (best.Basis.Activity != ""):
				better = weight > bestWeight || weight == bestWeight && basisKey(basis) < basisKey(best.Basis)
			}
		}
		if better {
			best, bestWeight = Crew{Members: members, Basis: basis}, weight
		}
	}
	for _, a := range offered {
		var members []UserID
		weight := 0
		for _, u := range clique {
			if w, ok := byID[u].Prefs[a]; ok && w >= Like {
				members = append(members, u)
				weight += w
			}
		}
		consider(members, CrewBasis{Activity: a}, weight)
	}
	areas := map[AreaID][]UserID{}
	for _, u := range clique {
		areas[byID[u].HomeArea] = append(areas[byID[u].HomeArea], u)
	}
	for area, members := range areas {
		consider(members, CrewBasis{Area: area}, 0)
	}
	return best, best.Members != nil
}

// withMutualPicks drops people without a mutual pick among the others,
// repeatedly, since each drop can strand someone else.
func withMutualPicks(members []UserID, mutual func(a, b UserID) bool) []UserID {
	members = slices.Clone(members)
	for {
		kept := members[:0:0]
		for _, u := range members {
			if slices.ContainsFunc(members, func(o UserID) bool { return o != u && mutual(u, o) }) {
				kept = append(kept, u)
			}
		}
		if len(kept) == len(members) {
			return kept
		}
		members = kept
	}
}

func basisKey(b CrewBasis) string { return string(b.Activity) + "|" + string(b.Area) }

// dedupeCrews keeps the best of crews that overlap: biggest first (then by
// basis and members), dropping any that shares two or more people with one
// already kept on the same basis. Two near-identical crews would split the
// same friends between two chats.
func dedupeCrews(crews []Crew) []Crew {
	key := func(c Crew) string {
		ids := make([]string, len(c.Members))
		for i, m := range c.Members {
			ids[i] = string(m)
		}
		return basisKey(c.Basis) + ":" + strings.Join(ids, ",")
	}
	slices.SortFunc(crews, func(a, b Crew) int {
		if len(a.Members) != len(b.Members) {
			return len(b.Members) - len(a.Members)
		}
		return strings.Compare(key(a), key(b))
	})
	var out []Crew
	for _, c := range crews {
		if !slices.ContainsFunc(out, func(o Crew) bool {
			return o.Basis == c.Basis && shared(c.Members, o.Members) >= 2
		}) {
			out = append(out, c)
		}
	}
	return out
}

func shared(a, b []UserID) int {
	n := 0
	for _, u := range a {
		if slices.Contains(b, u) {
			n++
		}
	}
	return n
}

// maximalCliques lists the maximal cliques of the graph (Bron–Kerbosch
// with pivoting), each sorted. The graph is sparse (people meet a few
// others per outing), so this stays fast at city scale.
func maximalCliques(nodes []UserID, adj map[UserID]map[UserID]bool) [][]UserID {
	var out [][]UserID
	var bk func(r, p, x []UserID)
	bk = func(r, p, x []UserID) {
		if len(p) == 0 && len(x) == 0 {
			c := slices.Clone(r)
			slices.Sort(c)
			out = append(out, c)
			return
		}
		// Pivot: the node with the most neighbours in p.
		var pivot UserID
		most := -1
		for _, u := range append(slices.Clone(p), x...) {
			n := 0
			for _, v := range p {
				if adj[u][v] {
					n++
				}
			}
			if n > most {
				pivot, most = u, n
			}
		}
		for _, v := range slices.Clone(p) {
			if adj[pivot][v] {
				continue
			}
			var np, nx []UserID
			for _, w := range p {
				if adj[v][w] {
					np = append(np, w)
				}
			}
			for _, w := range x {
				if adj[v][w] {
					nx = append(nx, w)
				}
			}
			bk(append(slices.Clone(r), v), np, nx)
			p = slices.DeleteFunc(p, func(w UserID) bool { return w == v })
			x = append(x, v)
		}
	}
	in := map[UserID]bool{}
	for _, n := range nodes {
		in[n] = true
	}
	// Restrict the graph to the candidate nodes.
	sub := make(map[UserID]map[UserID]bool, len(nodes))
	for _, n := range nodes {
		sub[n] = map[UserID]bool{}
		for m := range adj[n] {
			if in[m] {
				sub[n][m] = true
			}
		}
	}
	adj = sub
	bk(nil, slices.Clone(nodes), nil)
	return out
}
