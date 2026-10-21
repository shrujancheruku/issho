package matching

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// Config holds the engine's tunable settings. A city overrides any subset of
// them with JSON in app.cities.matching_weights; see ParseConfig.
type Config struct {
	// MinBucketSize drops buckets with fewer people than this: they can't
	// produce even the smallest group.
	MinBucketSize int `json:"min_bucket_size"`
	// MinGroupSize raises activities' own minimums: two people would be a
	// 1:1 meeting, which the platonic rules avoid.
	MinGroupSize int `json:"min_group_size"`
	// LanguagePairs lists the two-language events the city runs, such as
	// [["ja", "en"]] in Tokyo. Anyone fluent in either language can join;
	// language learning isn't used yet. Empty = single-language events only.
	LanguagePairs [][2]string `json:"language_pairs"`
	// BilingualWeight discounts two-language candidates during assembly so
	// people are grouped in a language they all share first (0..1).
	BilingualWeight float64 `json:"bilingual_weight"`

	// PrefValues is how much an OpenToTry, Like and Love counts towards an
	// activity's demand.
	PrefValues [3]float64 `json:"pref_values"`
	// OverBudget scales the interest of someone whose budget is below the
	// activity's cost: they might still go, but less happily.
	OverBudget float64 `json:"over_budget"`
	// VenueChoice is how many venues an area needs for an activity to count
	// as fully feasible there. Fewer venues scale its score down: the one
	// café might be shut or full.
	VenueChoice int `json:"venue_choice"`
	// PlansPerBucket keeps only the best few plans per bucket.
	PlansPerBucket int `json:"plans_per_bucket"`
	// FollowUpCategories are activity categories that can follow a
	// non-conversational main activity (bouldering, then ramen), so the
	// group gets a chance to talk.
	FollowUpCategories []string `json:"follow_up_categories"`
	// FollowUpWeight is how much a follow-up's demand adds to a plan's score.
	FollowUpWeight float64 `json:"follow_up_weight"`

	// SizeTargets is the group size each SizePref aims for.
	SizeTargets map[SizePref]int `json:"size_targets"`
	// MaxKnown caps how many people in a group may already know each other
	// (have a mutual pick), so groups stay open to new people. 2 = one pair.
	MaxKnown int `json:"max_known"`
	// Weights of the group score's parts.
	Weights Weights `json:"weights"`
	// SearchPasses bounds the local search after greedy placement.
	SearchPasses int `json:"search_passes"`

	// ExtraInvites is how many people are invited beyond the group's target
	// size, since some will decline. The event confirms at the target
	// (Proposal.Quorum).
	ExtraInvites int `json:"extra_invites"`
}

// Weights of the group score's parts, each of which is between 0 and 1.
type Weights struct {
	Interest    float64 `json:"interest"`    // how much members want the activity
	SizeFit     float64 `json:"size_fit"`    // how close the size is to what members prefer
	Mutual      float64 `json:"mutual"`      // a pair who picked each other before
	Newcomers   float64 `json:"newcomers"`   // newcomers mixed in, but not a majority
	Reliability float64 `json:"reliability"` // members who turn up
}

// DefaultConfig returns the settings used when a city doesn't override them.
func DefaultConfig() Config {
	return Config{
		MinBucketSize:      3,
		MinGroupSize:       3,
		BilingualWeight:    0.5,
		PrefValues:         [3]float64{0.5, 1, 2},
		OverBudget:         0.3,
		VenueChoice:        3,
		PlansPerBucket:     5,
		FollowUpCategories: []string{"food"},
		FollowUpWeight:     0.3,
		SizeTargets:        map[SizePref]int{Small: 4, Medium: 6, Large: 8},
		MaxKnown:           2,
		Weights:            Weights{Interest: 3, SizeFit: 1, Mutual: 1, Newcomers: 1, Reliability: 1},
		SearchPasses:       5,
		ExtraInvites:       2,
	}
}

// ParseConfig applies a city's overrides (app.cities.matching_weights) on top
// of DefaultConfig. Unknown keys are rejected so a typo doesn't silently
// leave a weight at its default. Nested objects (weights, size_targets)
// override key by key.
func ParseConfig(raw []byte) (Config, error) {
	cfg := DefaultConfig()
	if len(bytes.TrimSpace(raw)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("matching config: %w", err)
		}
	}
	return cfg, cfg.Validate()
}

// Validate reports settings that would break the engine.
func (c Config) Validate() error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if c.MinBucketSize < 2 || c.MinGroupSize < 3 {
		bad("min_bucket_size %d, min_group_size %d: must be at least 2 and 3", c.MinBucketSize, c.MinGroupSize)
	}
	for _, p := range c.LanguagePairs {
		if p[0] == "" || p[1] == "" || p[0] == p[1] {
			bad("language_pairs %v: need two different languages", p)
		}
	}
	if !slices.IsSorted(c.PrefValues[:]) || c.PrefValues[0] < 0 {
		bad("pref_values %v: must be non-negative and not decreasing", c.PrefValues)
	}
	if c.BilingualWeight < 0 || c.BilingualWeight > 1 {
		bad("bilingual_weight %v: must be within 0..1", c.BilingualWeight)
	}
	if c.OverBudget < 0 || c.OverBudget > 1 {
		bad("over_budget %v: must be within 0..1", c.OverBudget)
	}
	if c.VenueChoice < 1 || c.PlansPerBucket < 1 || c.SearchPasses < 0 || c.ExtraInvites < 0 || c.MaxKnown < 0 {
		bad("venue_choice, plans_per_bucket must be ≥ 1; search_passes, extra_invites, max_known ≥ 0")
	}
	for _, s := range []SizePref{Small, Medium, Large} {
		if c.SizeTargets[s] < 2 {
			bad("size_targets[%s] %d: must be at least 2", s, c.SizeTargets[s])
		}
	}
	w := c.Weights
	if min(w.Interest, w.SizeFit, w.Mutual, w.Newcomers, w.Reliability) < 0 {
		bad("weights must be non-negative")
	}
	return errors.Join(errs...)
}
