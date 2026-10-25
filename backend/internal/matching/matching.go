// Package matching suggests outings and, once enough people sign up,
// schedules them and forms the groups. It is pure: plain structs in, results
// out, no I/O, no clock and no randomness, so the same input always gives
// the same output. Callers (the worker, the API, the simulator) load the
// data and store the results.
//
// An outing happens in two phases: the plan first, the time second.
//
//  1. Suggest: plans (an activity in an area, in a language) that enough
//     nearby people would want, with no date yet. Pools groups people by
//     where and in which language they could meet.
//  2. Feed: each person sees every open plan, ranked and labelled by how
//     likely they are to want it, for a feed and a colour-coded map. People
//     sign up, saying which times suit them for that plan.
//  3. Schedule: once a plan's signups can make a good group on some date in
//     its window, pick that date and form the groups (greedy, then local
//     search). Each group invites a few more than it needs and confirms on
//     a quorum. Signups not placed wait for the plan's next round.
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

// Blocks lists the parts of the day in order.
var Blocks = []Block{Morning, Afternoon, Evening}

// Slot is a recurring weekly time window: "Sunday afternoon".
type Slot struct {
	Weekday time.Weekday
	Block   Block
}

// blockOrder sorts a day's blocks chronologically rather than alphabetically.
var blockOrder = map[Block]int{Morning: 0, Afternoon: 1, Evening: 2}

// Occasion is one concrete time an outing could happen: a date and a part
// of that day.
type Occasion struct {
	// Date is the local calendar date in the city, as midnight UTC (e.g.
	// time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)). Only the date matters;
	// UTC keeps Weekday right whatever the server's time zone.
	Date  time.Time
	Block Block // "" in Signup.Unavailable means the whole day
}

// Slot is the weekly slot the occasion falls in.
func (o Occasion) Slot() Slot { return Slot{o.Date.Weekday(), o.Block} }

func compareOccasions(a, b Occasion) int {
	return cmp.Or(a.Date.Compare(b.Date), cmp.Compare(blockOrder[a.Block], blockOrder[b.Block]))
}

// Occasions lists every block of every day from start for the given number
// of days: a plan's scheduling window.
func Occasions(start time.Time, days int) []Occasion {
	var out []Occasion
	for d := range days {
		for _, b := range Blocks {
			out = append(out, Occasion{start.AddDate(0, 0, d), b})
		}
	}
	return out
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
// out deleted and unverified accounts.
type User struct {
	ID             UserID
	HomeArea       AreaID
	MaxTravelMin   int // the longest trip from HomeArea they'll make
	Languages      []Language
	Availability   []Slot // usual weekly availability (profile); the default for signups
	Gender         Gender
	SameGenderOnly bool
	Size           SizePref
	Budget         int                // 0 free … 3 expensive, like Activity.CostLevel
	Prefs          map[ActivityID]int // OpenToTry, Like or Love; missing = no preference
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

// speaks reports whether the person can join an event in these languages.
// It's the only language rule: everyone speaks the event's primary or
// secondary language.
func (u *User) speaks(primary, secondary string) bool {
	return u.fluent(primary) || (secondary != "" && u.fluent(secondary))
}

// Activity is a catalogue activity as enabled in the city. The caller leaves
// out activities that are disabled or out of season for the window.
type Activity struct {
	ID              ActivityID
	Category        string
	CostLevel       int
	Conversational  bool
	MinSize         int
	MaxSize         int
	VenueCategories []CategoryID // empty = needs only a meeting point
	Blocks          []Block      // parts of the day it suits; empty = any
	// Walk means its plans are a walking route through several stops,
	// composed when the plan opens (internal/walks): it needs no venue
	// category of its own and gets no follow-up, since the walk ends
	// somewhere to sit and talk.
	Walk bool
	// Energy is how active it is (1 calm to 3 active), for ordering two
	// activities: the more active first.
	Energy int
	// Hours is how long it usually takes; 0 = DefaultHours. A plan's
	// activities together fill an outing (Config.OutingHours).
	Hours float64
}

// DefaultHours is an activity's length when the catalogue doesn't say.
const DefaultHours = 2.0

func (a *Activity) hours() float64 {
	if a.Hours > 0 {
		return a.Hours
	}
	return DefaultHours
}

// With is the activity a plan of a followed by b asks of its groups: both
// group sizes, a part of the day that suits both, and the dearer of the two.
// For scheduling; b nil = a.
func (a *Activity) With(b *Activity) *Activity {
	if b == nil {
		return a
	}
	c := *a
	c.MinSize = max(a.MinSize, b.MinSize)
	if b.MaxSize > 0 && (c.MaxSize == 0 || b.MaxSize < c.MaxSize) {
		c.MaxSize = b.MaxSize
	}
	c.CostLevel = max(a.CostLevel, b.CostLevel)
	c.Conversational = a.Conversational || b.Conversational
	c.Hours = a.hours() + b.hours()
	switch {
	case len(a.Blocks) == 0:
		c.Blocks = slices.Clone(b.Blocks)
	case len(b.Blocks) > 0:
		c.Blocks = nil
		for _, bl := range a.Blocks {
			if slices.Contains(b.Blocks, bl) {
				c.Blocks = append(c.Blocks, bl)
			}
		}
		if len(c.Blocks) == 0 { // no shared part of the day: the first's
			c.Blocks = slices.Clone(a.Blocks)
		}
	}
	return &c
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

func (t TravelTimes) reachable(u *User, area AreaID) bool {
	m, ok := t.Minutes(u.HomeArea, area)
	return ok && m <= u.MaxTravelMin
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

// Plan is an outing without a time: an activity in an area, in a language.
type Plan struct {
	Area     AreaID
	Activity ActivityID
	// FollowUp is an optional second activity (bouldering, then a meal;
	// a museum, then a garden), chosen when it makes a fuller outing that
	// more people want (see rankActivities). "" = none.
	FollowUp  ActivityID
	Primary   string // the event's language
	Secondary string // its second language (Config.LanguagePairs), or ""
}

// Signup is one person's "I'm in" for an open plan, with the times that
// suit them for it.
type Signup struct {
	User UserID
	// Pattern is the weekly slots that suit them for this plan, e.g. weekday
	// evenings and weekend mornings. nil = their profile availability (the
	// app pre-fills it and lets them change it per plan).
	Pattern []Slot
	// Unavailable are times they can't do within the window: a whole date
	// (Block "") such as "not Wednesday the 11th", or one occasion. The
	// caller also adds the occasions of events they're already going to.
	Unavailable []Occasion
}

// Available reports whether the signup can make the occasion.
func (s *Signup) Available(u *User, o Occasion) bool {
	pattern := s.Pattern
	if pattern == nil {
		pattern = u.Availability
	}
	if !slices.Contains(pattern, o.Slot()) {
		return false
	}
	for _, x := range s.Unavailable {
		if x.Date.Equal(o.Date) && (x.Block == "" || x.Block == o.Block) {
			return false
		}
	}
	return true
}
