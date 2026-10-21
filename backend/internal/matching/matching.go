// Package matching proposes outings to groups of people. It is pure: plain
// structs in, results out, no I/O, no clock and no randomness, so the same
// input always gives the same output. Callers (the worker, the simulator)
// load the data and store the results.
//
// The engine runs in steps:
//
//  1. Bucketing (Buckets): who could meet whom, where, when and in which
//     language.
//  2. Candidate plans: activities ranked by demand × feasibility. (planned)
//  3. Group assembly: greedy placement, then local search. (planned)
//  4. Invitations: over-invite, confirm on quorum. (planned)
package matching

import "time"

// UserID identifies a person (app.users.id).
type UserID string

// AreaID identifies an area of a city pack (app.areas.id).
type AreaID string

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

func (s Slot) less(o Slot) bool {
	if s.Weekday != o.Weekday {
		return s.Weekday < o.Weekday
	}
	return blockOrder[s.Block] < blockOrder[o.Block]
}

// Language is one language a person speaks (app.user_languages).
type Language struct {
	Code   string // ISO 639-1
	Fluent bool   // false = learning
	Level  int    // for learners: 1 beginner … 3 advanced; 0 if fluent
}

// User is what the engine needs to know about one person. The caller leaves
// out deleted and unverified accounts.
type User struct {
	ID           UserID
	HomeArea     AreaID
	MaxTravelMin int // the longest trip from HomeArea they'll make
	Languages    []Language
	Availability []Slot
}

func (u User) fluent(lang string) bool {
	for _, l := range u.Languages {
		if l.Code == lang && l.Fluent {
			return true
		}
	}
	return false
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

// Config holds the engine's tunable settings. Cities can override them
// (app.cities.matching_weights); start from DefaultConfig.
type Config struct {
	// MinBucketSize drops buckets with fewer people than this: they can't
	// produce even the smallest group.
	MinBucketSize int
	// ExchangeMinPerSide is how many people each side of a language exchange
	// needs (fluent in one language and learning the other), so nobody is
	// the only native speaker practising with everyone else.
	ExchangeMinPerSide int
}

// DefaultConfig returns the settings used when a city doesn't override them.
func DefaultConfig() Config {
	return Config{
		MinBucketSize:      3,
		ExchangeMinPerSide: 2,
	}
}
