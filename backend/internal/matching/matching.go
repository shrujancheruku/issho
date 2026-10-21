// Package matching proposes outings to groups of people. It is pure: plain
// structs in, results out, no I/O, no clock and no randomness, so the same
// input always gives the same output. Callers (the worker, the simulator)
// load the data and store the results.
//
// Run chains the steps:
//
//  1. Bucketing (Buckets): who could meet whom, where, when and in which
//     language.
//  2. Candidate plans (Plans): activities ranked by demand × feasibility.
//  3. Group assembly (Assemble): greedy placement, then local search.
//  4. Invitations: a few more invites than needed, confirmed on quorum.
package matching

import (
	"cmp"
	"slices"
	"time"
)

// UserID identifies a person (app.users.id).
type UserID string

// AreaID identifies an area of a city pack (app.areas.id).
type AreaID string

// ActivityID identifies a catalogue activity (app.activities.id).
type ActivityID string

// CategoryID identifies a venue category (app.venue_categories.id).
type CategoryID string

// Block is a part of the day, as in app.availability_slots.block.
type Block string

const (
	Morning   Block = "morning"
	Afternoon Block = "afternoon"
	Evening   Block = "evening"
)

// Slot is a recurring weekly time window: "Sunday afternoon".
type Slot struct {
	Weekday time.Weekday
	Block   Block
}

// blockOrder sorts a day's blocks chronologically rather than alphabetically.
var blockOrder = map[Block]int{Morning: 0, Afternoon: 1, Evening: 2}

func compareSlots(a, b Slot) int {
	return cmp.Or(cmp.Compare(a.Weekday, b.Weekday), cmp.Compare(blockOrder[a.Block], blockOrder[b.Block]))
}

// Gender as in app.profiles.gender.
type Gender string

const (
	Woman       Gender = "woman"
	Man         Gender = "man"
	Nonbinary   Gender = "nonbinary"
	Unspecified Gender = "unspecified"
)

// SizePref is a person's preferred group size (app.profiles.size_pref).
type SizePref string

const (
	Small  SizePref = "small"
	Medium SizePref = "medium"
	Large  SizePref = "large"
)

// Preference weights, as in app.user_activity_prefs.weight.
const (
	OpenToTry = 0
	Like      = 1
	Love      = 2
)

// Language is one language a person speaks (app.user_languages).
type Language struct {
	Code   string // ISO 639-1
	Fluent bool   // false = learning (not used for matching yet)
	Level  int    // for learners: 1 beginner … 3 advanced; 0 if fluent
}

// User is what the engine needs to know about one person. The caller leaves
// out deleted and unverified accounts, and anyone who already has an event
// in the week being matched.
type User struct {
	ID             UserID
	HomeArea       AreaID
	MaxTravelMin   int // the longest trip from HomeArea they'll make
	Languages      []Language
	Availability   []Slot
	Gender         Gender
	SameGenderOnly bool
	Size           SizePref
	Budget         int                // 0 free … 3 expensive, like Activity.CostLevel
	Prefs          map[ActivityID]int // OpenToTry, Like or Love; missing = not interested
	Reliability    float64            // 0..1: how often they turn up after accepting
	EventsAttended int                // 0 = newcomer
}

func (u *User) fluent(lang string) bool {
	for _, l := range u.Languages {
		if l.Code == lang && l.Fluent {
			return true
		}
	}
	return false
}

// Activity is a catalogue activity as enabled in the city. The caller leaves
// out activities that are disabled or out of season for the week matched.
type Activity struct {
	ID              ActivityID
	Category        string
	CostLevel       int
	Conversational  bool
	MinSize         int
	MaxSize         int
	VenueCategories []CategoryID // empty = needs only a meeting point
	Blocks          []Block      // parts of the day it suits; empty = any
}

// preferredMin is the size a group should reach: the activity's own minimum
// or cfg.PreferredMinSize, whichever is larger. Smaller groups are allowed
// (down to cfg.MinGroupSize) but score lower.
func (a *Activity) preferredMin(cfg Config) int {
	return max(a.MinSize, cfg.PreferredMinSize, cfg.MinGroupSize)
}

func (a *Activity) suits(b Block) bool { return len(a.Blocks) == 0 || slices.Contains(a.Blocks, b) }

// Venues counts the approved venues of each category in each area.
type Venues map[AreaID]map[CategoryID]int

// count returns how many venues the activity could use in the area.
func (v Venues) count(area AreaID, a *Activity) int {
	n := 0
	for _, c := range a.VenueCategories {
		n += v[area][c]
	}
	return n
}

// AreaPair is a directed trip between two areas.
type AreaPair struct{ From, To AreaID }

// TravelTimes holds minutes between areas (app.area_travel_times). The caller
// keeps the fastest of the city's modes for each pair: someone who'd walk 20
// minutes won't drive 25.
type TravelTimes map[AreaPair]int

// Minutes returns the travel time from one area to another. Staying in the
// same area is always 0. A pair with no row has no route (e.g. an island
// with no road, or areas too far apart to walk in a walk-only city).
func (t TravelTimes) Minutes(from, to AreaID) (int, bool) {
	if from == to {
		return 0, true
	}
	m, ok := t[AreaPair{from, to}]
	return m, ok
}

// Pair is an unordered pair of people. Build it with MakePair.
type Pair struct{ A, B UserID }

// MakePair orders the two ids so (a, b) and (b, a) are the same key.
func MakePair(a, b UserID) Pair {
	if b < a {
		a, b = b, a
	}
	return Pair{a, b}
}

// Relations are what people said about each other after past events.
type Relations struct {
	// Mutual holds pairs who both picked each other, with a strength in
	// (0, 1]. Picks count for less from people whose picks are rarely
	// returned (M7).
	Mutual map[Pair]float64
	// Blocked holds pairs who must never be grouped (either side blocked).
	Blocked map[Pair]bool
}

// Input is everything one matching run needs, for one city and one week.
type Input struct {
	Users      []User
	Areas      []AreaID
	Travel     TravelTimes
	Activities []Activity
	Venues     Venues
	Relations  Relations
}

// Run executes all the steps and returns the week's proposals, sorted by
// slot, area and plan. Each person is invited to at most one.
func Run(in Input, cfg Config) []Proposal {
	buckets := Buckets(in.Users, in.Areas, in.Travel, cfg)
	users := make(map[UserID]*User, len(in.Users))
	for i := range in.Users {
		users[in.Users[i].ID] = &in.Users[i]
	}
	acts := make(map[ActivityID]*Activity, len(in.Activities))
	for i := range in.Activities {
		acts[in.Activities[i].ID] = &in.Activities[i]
	}
	var cands []candidate
	for i := range buckets {
		for _, p := range Plans(buckets[i], users, in.Activities, in.Venues, cfg) {
			cands = append(cands, candidate{bucket: &buckets[i], plan: p})
		}
	}
	return Assemble(cands, users, acts, in.Relations, cfg)
}
