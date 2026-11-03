package matching

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Config holds the engine's tunable settings. A city overrides any subset of
// them with JSON in app.cities.matching; see ParseConfig.
type Config struct {
	// MinGroupSize is the only hard size limit: below it there's no group,
	// so smaller pools are dropped too.
	// Everything above it is a preference (PreferredMinSize, the activity's
	// own minimum, SizeTargets).
	MinGroupSize int `json:"min_group_size"`
	// PreferredMinSize is the smallest size groups aim for; smaller groups
	// (a pair is close to a 1:1 meeting) are allowed but score lower. An
	// activity's own min_group_size raises it.
	PreferredMinSize int `json:"preferred_min_size"`
	// LanguagePairs lists the two-language events the city runs, such as
	// [["ja", "en"]] in Tokyo. Anyone fluent in either language can join;
	// language learning isn't used yet. Empty = single-language events only.
	LanguagePairs [][2]string `json:"language_pairs"`
	// BilingualWeight discounts two-language suggestions so most plans are
	// in one language everyone shares (0..1).
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
	// PlansPerPool is how many plans to suggest per pool (area × language).
	PlansPerPool int `json:"plans_per_pool"`
	// PlanSpacingMin keeps plans for the same activity in a shared language
	// at least this many minutes apart, so they don't split one crowd.
	PlanSpacingMin int `json:"plan_spacing_min"`
	// UsersPerPlan caps open plans at one per this many people in the city,
	// so signups concentrate on plans that can fill.
	UsersPerPlan int `json:"users_per_plan"`
	// MinAudience is how many people in a pool must want an activity for it
	// to be suggested: only some of them will sign up.
	MinAudience int `json:"min_audience"`
	// MomentumAt is the signup count at which a plan's momentum bonus in the
	// feed is full (see Weights.Momentum).
	MomentumAt int `json:"momentum_at"`
	// OutingHours is how long an outing feels full: a plan shorter than this
	// scores in proportion (one 1.5-hour session alone scores 0.6 of its
	// demand). One long activity and two shorter ones both fill it.
	OutingHours float64 `json:"outing_hours"`
	// MaxOutingHours caps two activities' total length.
	MaxOutingHours float64 `json:"max_outing_hours"`
	// NeutralInterest is a person's interest in a plan's activity they have
	// no preference for, when they like its other one (appeal): a meal after
	// bouldering is fine even if they didn't pick meals.
	NeutralInterest float64 `json:"neutral_interest"`
	// PairBonus scores a two-activity plan this much higher than its parts
	// alone would: a preference for variety, not a rule.
	PairBonus float64 `json:"pair_bonus"`
	// PairCandidates is how many of a pool's most wanted activities are tried
	// as each activity's second.
	PairCandidates int `json:"pair_candidates"`
	// PairShortOnly pairs only activities too short to fill an outing alone
	// (under OutingHours): a long walk or a festival always stands alone.
	PairShortOnly bool `json:"pair_short_only"`

	// SizeTargets is the group size each SizePref aims for.
	SizeTargets map[SizePref]int `json:"size_targets"`
	// Weights of the group score's parts.
	Weights Weights `json:"weights"`
	// SearchPasses bounds the local search after greedy placement.
	SearchPasses int `json:"search_passes"`

	// ExtraInvites is how many people are invited beyond the group's target
	// size, since some will drop out. The event confirms early at the
	// target (Group.Quorum), or at the deadline with at least MinGroupSize.
	ExtraInvites int `json:"extra_invites"`

	// Timing is when rounds happen in the city: how long signups stay open
	// and at what time of day each block starts.
	Timing Timing `json:"timing"`
}

// Timing sets when a city's rounds can happen. Whole days and clock times,
// so a city's pack can write them down; internal/plans turns them into dates.
type Timing struct {
	// LeadDays: the earliest possible occasion is this many days after
	// today, so invitees have at least a day to confirm.
	LeadDays int `json:"lead_days"`
	// WindowDays: how many days a round's occasions span.
	WindowDays int `json:"window_days"`
	// SignupDays: the signup deadline is this many days into the window.
	// At the deadline the round is scheduled with whoever can go, on a date
	// at least LeadDays later, so it must leave room for that.
	SignupDays int `json:"signup_days"`
	// CooldownDays: a plan that closed without a group isn't suggested again
	// for this long.
	CooldownDays int `json:"cooldown_days"`
	// BlockStarts is when an event starts in each part of the day, as local
	// "HH:MM" (an 18:30 dinner in Tokyo, 21:00 in Spain). The venue's hours
	// aren't known in advance, so this is a sensible default people can plan
	// around; the meeting time can be refined later.
	BlockStarts map[Block]string `json:"block_starts"`
}

// Upper bounds of Timing's day counts.
const (
	MaxWindowDays   = 60
	MaxLeadDays     = 14
	MaxSignupDays   = 30
	MaxCooldownDays = 365
)

// BlockStart returns when the block starts, as time since midnight.
func (t Timing) BlockStart(b Block) (time.Duration, error) {
	h, m, ok := strings.Cut(t.BlockStarts[b], ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || len(h) != 2 || len(m) != 2 || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, fmt.Errorf("block_starts[%s] %q: need a time like \"18:30\"", b, t.BlockStarts[b])
	}
	return time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute, nil
}

// Validate reports a timing that would leave no occasion at the deadline.
func (t Timing) Validate() error {
	var errs []error
	if t.LeadDays < 1 || t.SignupDays < 1 || t.SignupDays+t.LeadDays >= t.WindowDays {
		errs = append(errs, fmt.Errorf("timing: need lead_days ≥ 1, signup_days ≥ 1 and signup_days + lead_days < window_days (got %d, %d, %d)",
			t.LeadDays, t.SignupDays, t.WindowDays))
	}
	// Upper bounds catch a typo (140 for 14) before it sends events a year out.
	if t.WindowDays > MaxWindowDays || t.LeadDays > MaxLeadDays || t.SignupDays > MaxSignupDays {
		errs = append(errs, fmt.Errorf("timing: at most window_days %d, lead_days %d, signup_days %d (got %d, %d, %d)",
			MaxWindowDays, MaxLeadDays, MaxSignupDays, t.WindowDays, t.LeadDays, t.SignupDays))
	}
	if t.CooldownDays < 0 || t.CooldownDays > MaxCooldownDays {
		errs = append(errs, fmt.Errorf("timing: cooldown_days %d: need 0..%d", t.CooldownDays, MaxCooldownDays))
	}
	for _, b := range Blocks {
		if _, err := t.BlockStart(b); err != nil {
			errs = append(errs, fmt.Errorf("timing: %w", err))
		}
	}
	// Parts of the day come in order: an evening before the afternoon would
	// schedule backwards.
	var prev time.Duration = -1
	for _, b := range Blocks {
		d, err := t.BlockStart(b)
		if err != nil {
			break // reported above
		}
		if d <= prev {
			errs = append(errs, fmt.Errorf("timing: block_starts must be in order, morning < afternoon < evening (%s %q is not after the one before)", b, t.BlockStarts[b]))
		}
		prev = d
	}
	for b := range t.BlockStarts {
		if !slices.Contains(Blocks, b) {
			errs = append(errs, fmt.Errorf("timing: block_starts: unknown block %q", b))
		}
	}
	return errors.Join(errs...)
}

// Weights of the group score's parts, each of which is between 0 and 1,
// and of the feed's momentum.
type Weights struct {
	Interest    float64 `json:"interest"`    // how much members want the activity
	SizeFit     float64 `json:"size_fit"`    // how close the size is to what members prefer
	MinSize     float64 `json:"min_size"`    // expected attendance reaches the preferred minimum
	Balance     float64 `json:"balance"`     // nobody is the only woman or the only man
	Mutual      float64 `json:"mutual"`      // how much members are already friends (clusters count most)
	Newcomers   float64 `json:"newcomers"`   // newcomers aren't a majority
	Reliability float64 `json:"reliability"` // members who turn up

	// Friends is how much people they get on with having signed up lifts a
	// plan in someone's feed (on the same scale), besides moving it up a
	// level.
	Friends float64 `json:"friends"`

	// Momentum is how much a plan's signups so far lift it in someone's
	// feed (on the same scale as a Like = 1), so plans close to happening
	// fill up.
	Momentum float64 `json:"momentum"`
}

// DefaultConfig returns the settings used when a city doesn't override them.
func DefaultConfig() Config {
	return Config{
		MinGroupSize:     2,
		PreferredMinSize: 3,
		BilingualWeight:  0.5,
		PrefValues:       [3]float64{0.5, 1, 2},
		OverBudget:       0.3,
		VenueChoice:      3,
		PlansPerPool:     5,
		PlanSpacingMin:   20,
		UsersPerPlan:     10,
		MinAudience:      6,
		MomentumAt:       8,
		OutingHours:      2,
		MaxOutingHours:   4,
		NeutralInterest:  0.25,
		PairBonus:        0.1,
		PairCandidates:   10,
		PairShortOnly:    true,
		SizeTargets:      map[SizePref]int{Small: 4, Medium: 6, Large: 8},
		Weights:          Weights{Interest: 3, SizeFit: 1, MinSize: 1, Balance: 2, Mutual: 2, Newcomers: 1, Reliability: 1, Momentum: 0.5, Friends: 1.5},
		SearchPasses:     5,
		ExtraInvites:     2,
		// Rounds open for signups for a week, then pick a date in the week
		// after.
		Timing: Timing{
			LeadDays: 2, WindowDays: 14, SignupDays: 7, CooldownDays: 14,
			BlockStarts: map[Block]string{Morning: "10:00", Afternoon: "14:00", Evening: "18:30"},
		},
	}
}

// ParseConfig applies a city's overrides (app.cities.matching) on top
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
	if c.MinGroupSize < 2 || c.PreferredMinSize < c.MinGroupSize {
		bad("min_group_size %d: must be at least 2; preferred_min_size %d: at least min_group_size",
			c.MinGroupSize, c.PreferredMinSize)
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
	if min(c.VenueChoice, c.PlansPerPool, c.UsersPerPlan, c.MinAudience, c.MomentumAt) < 1 ||
		min(c.SearchPasses, c.ExtraInvites, c.PlanSpacingMin) < 0 {
		bad("venue_choice, plans_per_pool, users_per_plan, min_audience, momentum_at must be ≥ 1; search_passes, extra_invites, plan_spacing_min ≥ 0")
	}
	for _, s := range []SizePref{Small, Medium, Large} {
		if c.SizeTargets[s] < 2 {
			bad("size_targets[%s] %d: must be at least 2", s, c.SizeTargets[s])
		}
	}
	w := c.Weights
	if min(w.Interest, w.SizeFit, w.MinSize, w.Balance, w.Mutual, w.Newcomers, w.Reliability, w.Momentum, w.Friends) < 0 {
		bad("weights must be non-negative")
	}
	if err := c.Timing.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
