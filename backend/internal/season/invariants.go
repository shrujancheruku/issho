package season

import (
	"fmt"
	"slices"

	"github.com/shrujancheruku/issho/backend/internal/policy"
)

// World is what the season saw of the packages as it ran: only what their
// exported APIs told it. The invariants are checked on it, so a test can break
// one on purpose and see it caught (Broken).
type World struct {
	// SetOutings are the outings seen set, as they were when first seen set.
	SetOutings []SetOuting
	// Sweeps are the going seats at each sweep, in order: a seat cancelled by
	// a sweep is no longer in it.
	Sweeps []Sweep
	// Holders are the members who hold a signup at the end, and whether each is
	// admitted.
	Holders []Holder
	// Attendance is, for each person who answered feedback, whether they were
	// told to have attended (they checked in or said they went; their own no
	// wins) and whether the outing says they did.
	Attendance []AttendanceFact
}

// AttendanceFact is one person's attendance as the season expects it and as
// outings reports it.
type AttendanceFact struct {
	Member    int
	Want, Got bool
}

// SetOuting is an outing as it was when it was first seen set.
type SetOuting struct {
	ID, Date, Part string
	// Languages are the languages the outing says its group shares.
	Languages []string
	Members   []SetMember
}

// SetMember is one person going to a set outing.
type SetMember struct {
	Member int
	// Spoken are the languages on the person's profile, and Ticked whether the
	// signup that formed the group held the outing's date and part of the day.
	Spoken []string
	Ticked bool
}

// Sweep is the seats live (going) when the season looked, after a forming or
// an answer. A seat counts as cancelled from the sweep that sees the cancel.
type Sweep struct{ Live []GoingSeat }

// GoingSeat is a person going to an outing at a date and part of the day.
type GoingSeat struct {
	Member     int
	Outing     string
	Date, Part string
}

// Holder is a member holding a signup.
type Holder struct {
	Member   int
	Admitted bool
}

// CheckInvariants returns what is wrong with the world, one line each, empty
// when nothing is (docs/spec/quality.md, "The season"): every outing that was
// set had 3 to 6 people sharing a language and a time; nobody was double-booked;
// nobody who isn't admitted has a signup.
func CheckInvariants(w World) []string {
	var bad []string
	for _, o := range w.SetOutings {
		n := len(o.Members)
		if n < policy.MinGroupSize || n > policy.MaxGroupSize {
			bad = append(bad, fmt.Sprintf("outing %s %s %s was set with %d going, want %d to %d", o.ID, o.Date, o.Part, n, policy.MinGroupSize, policy.MaxGroupSize))
		}
		if len(o.Languages) == 0 {
			bad = append(bad, fmt.Sprintf("outing %s %s %s shares no language", o.ID, o.Date, o.Part))
		}
		for _, m := range o.Members {
			for _, l := range o.Languages {
				if !slices.Contains(m.Spoken, l) {
					bad = append(bad, fmt.Sprintf("outing %s: member %d does not speak %s, which it says the group shares", o.ID, m.Member, l))
				}
			}
			if !m.Ticked {
				bad = append(bad, fmt.Sprintf("outing %s: member %d did not tick %s %s", o.ID, m.Member, o.Date, o.Part))
			}
		}
		if len(o.Languages) > 0 && !shareOne(o) {
			bad = append(bad, fmt.Sprintf("outing %s: no one language is spoken by everyone going", o.ID))
		}
	}
	type slot struct {
		member     int
		date, part string
	}
	// Each sweep on its own: a later cancel must not hide a double booking
	// that stood at an earlier one.
	for i, sw := range w.Sweeps {
		at := map[slot]string{}
		for _, g := range sw.Live {
			k := slot{g.Member, g.Date, g.Part}
			if prev, ok := at[k]; ok && prev != g.Outing {
				bad = append(bad, fmt.Sprintf("sweep %d: member %d is going to outings %s and %s on %s %s", i, g.Member, prev, g.Outing, g.Date, g.Part))
			}
			at[k] = g.Outing
		}
	}
	for _, h := range w.Holders {
		if !h.Admitted {
			bad = append(bad, fmt.Sprintf("member %d is not admitted and holds a signup", h.Member))
		}
	}
	for _, a := range w.Attendance {
		if a.Want != a.Got {
			bad = append(bad, fmt.Sprintf("member %d: attended is %v, want %v", a.Member, a.Got, a.Want))
		}
	}
	return bad
}

// shareOne is whether one language is on every going member's profile.
func shareOne(o SetOuting) bool {
	if len(o.Members) == 0 {
		return true
	}
	for _, l := range o.Members[0].Spoken {
		all := true
		for _, m := range o.Members[1:] {
			all = all && slices.Contains(m.Spoken, l)
		}
		if all {
			return true
		}
	}
	return false
}

// Broken is a world with one rule broken, for the test that proves the check
// can fail: "double-book", "double-book-mid-run", "unadmitted-signup", "set-too-small", "set-too-big",
// "no-shared-language" or "no-shared-time".
func Broken(bug string) World {
	member := func(i int, spoken ...string) SetMember { return SetMember{Member: i, Spoken: spoken, Ticked: true} }
	good := SetOuting{ID: "o1", Date: "2026-01-14", Part: "evening", Languages: []string{"en"},
		Members: []SetMember{member(1, "en"), member(2, "en", "vi"), member(3, "en")}} //policylint:ignore a scripted number of the season, not that rule
	w := World{SetOutings: []SetOuting{good}, Holders: []Holder{{Member: 1, Admitted: true}}}
	switch bug {
	case "double-book":
		w.Sweeps = []Sweep{{Live: []GoingSeat{
			{Member: 1, Outing: "o1", Date: "2026-01-14", Part: "evening"},
			{Member: 1, Outing: "o2", Date: "2026-01-14", Part: "evening"},
		}}}
	case "double-book-mid-run":
		// Double booked at the first sweep; the second outing's seat is
		// cancelled by the second, so the end state alone looks fine.
		w.Sweeps = []Sweep{
			{Live: []GoingSeat{
				{Member: 1, Outing: "o1", Date: "2026-01-14", Part: "evening"},
				{Member: 1, Outing: "o2", Date: "2026-01-14", Part: "evening"},
			}},
			{Live: []GoingSeat{{Member: 1, Outing: "o1", Date: "2026-01-14", Part: "evening"}}},
		}
	case "unadmitted-signup":
		w.Holders = append(w.Holders, Holder{Member: 9, Admitted: false})
	case "set-too-small":
		w.SetOutings[0].Members = w.SetOutings[0].Members[:2]
	case "set-too-big":
		w.SetOutings[0].Members = append(w.SetOutings[0].Members, member(4, "en"), member(5, "en"), member(6, "en"), member(7, "en")) //policylint:ignore a scripted number of the season, not that rule
	case "no-shared-language":
		w.SetOutings[0].Languages = []string{"fr"}
	case "no-shared-time":
		w.SetOutings[0].Members[1].Ticked = false
	}
	return w
}
