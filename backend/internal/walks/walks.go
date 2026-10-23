// Package walks composes walking plans: 2 to 4 hours from a meeting place
// (a station or a café), through several stops on the plan's theme (temples,
// places to photograph, street food), to somewhere to sit and talk.
//
// A model proposes the walk from its own knowledge of the area (Proposer);
// the composer then checks it against Google Maps: every place must exist,
// be open, be in the area and be the kind of place the model said it was,
// and the walk between stops must be short. Routes may reorder the stops in
// between for the shortest walk. Places that fail are dropped; a walk that
// still breaks the rules goes back to the model once, with the problems
// named, and is otherwise rejected.
//
// Nothing from Google goes to the model (Maps terms: no content may be
// created from Google Maps content). The model gets the area's name and
// centre from the city pack, and its feedback names only its own places and
// our rules. Only Google place ids are kept.
package walks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// Request is one walk to compose. Everything in it is our own data.
type Request struct {
	City, Area string      // English names, from the city pack
	Center     maps.LatLng // the area's centre
	Activity   string      // English name, e.g. "Temples and shrines"
	Theme      string      // activities.walk_theme
	Month      time.Month  // when the plan's first round starts
	Blocks     []string    // parts of the day the activity suits; empty = any
}

// Kinds of place the model may name. A meeting place must be a station or a
// café; the finish somewhere to sit; temples and shrines are checked
// against Google's types, the rest aren't (a street or a viewpoint has no
// reliable type).
var Kinds = []string{
	"station", "cafe", "tea_house", "restaurant", "bar", "temple", "shrine", "church", "park", "garden",
	"street", "market", "shop", "viewpoint", "museum", "gallery", "landmark", "other",
}

// Place is one place the model proposed.
type Place struct {
	Name    string // as on Google Maps, e.g. "Kayaba Coffee"
	Search  string // text that finds it, e.g. "Kayaba Coffee, Yanaka"
	Kind    string // one of Kinds
	Minutes int    // time spent there
	Note    string // why it's worth the stop, for the group
}

// Proposal is the model's walk, before checking.
type Proposal struct {
	Title, Summary string
	Meet           Place
	Stops          []Place
	Finish         Place
	Meta           Meta // about the call, for benchmarks and logs
}

// Meta describes the model call that made a proposal.
type Meta struct {
	Model                     string // the model that answered (as reported by the API when it says)
	InputTokens, OutputTokens int    // output includes thinking
	Latency                   time.Duration
	// CostUSD when the client reports it (Claude Code); 0 = work it out
	// from the tokens.
	CostUSD float64
}

// Proposer proposes walks. feedback lists what was wrong with its previous
// proposal for the same request (empty the first time).
type Proposer interface {
	Propose(ctx context.Context, req Request, feedback []string) (Proposal, error)
}

// Role is a stop's part in the walk (app.plan_stops.role).
type Role string

const (
	Meet   Role = "meet"
	Stop   Role = "stop"
	Finish Role = "finish"
)

// WalkStop is one checked stop.
type WalkStop struct {
	Role    Role
	PlaceID string // Google place id: the only thing kept about the place
	Minutes int
	Note    string
	// Walk is the walk here from the previous stop (Routes content: cache
	// for 30 days at most); 0 for the first.
	Walk time.Duration
}

// Walk is a composed walk, stops in walking order.
type Walk struct {
	Title, Summary string
	Stops          []WalkStop
	Route          maps.Route // the whole walk's line and time
}

// Minutes is the whole outing: time at the stops plus walking.
func (w Walk) Minutes() int {
	n := int(w.Route.Duration.Round(time.Minute) / time.Minute)
	for _, s := range w.Stops {
		n += s.Minutes
	}
	return n
}

// Rejected is a walk that couldn't be made to fit the rules.
type Rejected struct {
	Reasons []string
}

func (e *Rejected) Error() string { return "no walk: " + strings.Join(e.Reasons, "; ") }

// Rules bound a walk.
type Rules struct {
	// Stops on the theme, after places that fail are dropped.
	MinStops, MaxStops int
	// The whole outing, at the stops and walking.
	MinMinutes, MaxMinutes int
	// Longest walk between two stops, and walking in all.
	MaxLeg, MaxWalking time.Duration
	// How far from the area's centre a place may be.
	MaxFromCenterM float64
	// Proposals per walk: the first, then retries with feedback.
	Attempts int
}

// DefaultRules: 2 to 4 hours, 3 to 8 stops, no walk between stops longer
// than 20 minutes and 100 minutes of walking at most.
func DefaultRules() Rules {
	return Rules{
		MinStops: 3, MaxStops: 8,
		MinMinutes: 120, MaxMinutes: 240,
		MaxLeg: 20 * time.Minute, MaxWalking: 100 * time.Minute,
		MaxFromCenterM: 3000,
		Attempts:       2,
	}
}

// Composer composes walks.
type Composer struct {
	Proposer Proposer
	Places   maps.PlaceFinder
	Routes   maps.WalkRouter
	Rules    Rules
	Log      *slog.Logger // nil = slog.Default()
	// OnAttempt, if set, hears about every proposal and how it fared
	// (benchmarks, logs).
	OnAttempt func(Attempt)
}

// Why a proposed place wasn't used (Drop.Reason).
const (
	DropNotFound  = "not_found"  // Places found nothing
	DropClosed    = "closed"     // closed for good or for now
	DropFar       = "far"        // outside the area
	DropWrongKind = "wrong_kind" // Google doesn't list it as that kind of place
	DropDuplicate = "duplicate"  // the same place as another stop
	DropOutlier   = "outlier"    // too far from the other stops
)

// Drop is a proposed place that wasn't used.
type Drop struct {
	Role   Role
	Name   string // the model's name for it
	Reason string // a Drop* constant
}

// Attempt is one proposal and what the checks made of it.
type Attempt struct {
	N        int // 1 for the first proposal
	Proposal Proposal
	Drops    []Drop
	Problems []string // why it was rejected; empty when it became the walk
	Walk     *Walk    // the walk, when it passed
	Err      error    // the proposer or Maps failed
}

// Compose proposes a walk and checks it. A walk that can't be made to fit
// the rules is a *Rejected; other errors (an API down) are worth retrying
// later.
func (c *Composer) Compose(ctx context.Context, req Request) (Walk, error) {
	var feedback []string
	for n := range max(c.Rules.Attempts, 1) {
		p, err := c.Proposer.Propose(ctx, req, feedback)
		if err != nil {
			c.observe(Attempt{N: n + 1, Err: err})
			return Walk{}, fmt.Errorf("propose: %w", err)
		}
		w, problems, drops, err := c.check(ctx, req, p)
		a := Attempt{N: n + 1, Proposal: p, Drops: drops, Problems: problems, Err: err}
		if err == nil && len(problems) == 0 {
			a.Walk = &w
		}
		c.observe(a)
		if err != nil {
			return Walk{}, err
		}
		if len(problems) == 0 {
			return w, nil
		}
		c.log().Info("walk proposal rejected", "area", req.Area, "activity", req.Activity, "problems", problems)
		feedback = problems
	}
	return Walk{}, &Rejected{Reasons: feedback}
}

func (c *Composer) observe(a Attempt) {
	if c.OnAttempt != nil {
		c.OnAttempt(a)
	}
}

func (c *Composer) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}

// placeTypes lists the Google types that confirm a kind. Kinds not listed
// aren't checked.
var placeTypes = map[string][]string{
	"station":   {"train_station", "subway_station", "transit_station", "light_rail_station", "bus_station", "transit_depot"},
	"cafe":      {"cafe", "coffee_shop", "tea_house", "cat_cafe", "dog_cafe", "bakery", "dessert_shop", "confectionery", "juice_shop", "book_store"},
	"tea_house": {"tea_house", "cafe", "coffee_shop", "japanese_restaurant", "dessert_shop", "confectionery"},
	"bar":       {"bar", "pub", "wine_bar", "brewpub", "brewery", "cocktail_bar", "bar_and_grill", "beer_hall", "lounge_bar", "sports_bar", "night_club", "japanese_izakaya_restaurant"},
	"temple":    {"buddhist_temple", "hindu_temple", "place_of_worship", "church", "mosque", "synagogue", "historical_landmark", "tourist_attraction"},
	"shrine":    {"shinto_shrine", "place_of_worship", "buddhist_temple", "historical_landmark", "tourist_attraction"},
}

// is reports whether a found place can be the kind of place named.
func is(f maps.Found, kind string) bool {
	if kind == "restaurant" {
		// Restaurants have dozens of types (ramen_restaurant, …).
		return slices.ContainsFunc(append([]string{f.PrimaryType}, f.Types...), func(t string) bool {
			return strings.HasSuffix(t, "_restaurant") || t == "restaurant" || t == "food_court" || t == "meal_takeaway"
		})
	}
	want, ok := placeTypes[kind]
	if !ok {
		return true
	}
	return slices.Contains(want, f.PrimaryType) || slices.ContainsFunc(f.Types, func(t string) bool { return slices.Contains(want, t) })
}

// Kinds allowed for the meeting place and the finish.
var (
	meetKinds   = []string{"station", "cafe", "tea_house"}
	finishKinds = []string{"cafe", "tea_house", "restaurant", "bar"}
)

// candidate is a proposed place and what Places found for it.
type candidate struct {
	role    Role
	place   Place
	found   maps.Found
	problem string // why it can't be used; "" = fine
	reason  string // the problem as a Drop* constant
}

// check resolves the proposal's places, drops stops that fail, routes the
// walk and checks it against the rules. problems (for the model, naming
// only its own places) is empty when the walk is good.
func (c *Composer) check(ctx context.Context, req Request, p Proposal) (Walk, []string, []Drop, error) {
	var problems []string
	if strings.TrimSpace(p.Title) == "" || len([]rune(p.Title)) > 70 {
		problems = append(problems, "the title must be at most 60 characters")
	}
	if !slices.Contains(meetKinds, p.Meet.Kind) {
		problems = append(problems, fmt.Sprintf("%s: the meeting place must be a station or a café", p.Meet.Name))
	}
	if !slices.Contains(finishKinds, p.Finish.Kind) {
		problems = append(problems, fmt.Sprintf("%s: the walk must end at a café, tea house, restaurant or bar", p.Finish.Name))
	}
	cands := []*candidate{{role: Meet, place: p.Meet}}
	for _, s := range p.Stops {
		cands = append(cands, &candidate{role: Stop, place: s})
	}
	cands = append(cands, &candidate{role: Finish, place: p.Finish})
	if err := c.resolve(ctx, req, cands); err != nil {
		return Walk{}, nil, nil, err
	}
	var drops []Drop
	for _, cd := range cands {
		if cd.problem != "" {
			drops = append(drops, Drop{Role: cd.role, Name: cd.place.Name, Reason: cd.reason})
		}
	}
	var stops []*candidate
	for _, cd := range cands {
		switch {
		case cd.problem != "" && cd.role != Stop:
			problems = append(problems, cd.problem)
		case cd.problem != "":
			// A stop that fails is dropped; it only matters if too few are left.
			c.log().Info("walk stop dropped", "area", req.Area, "problem", cd.problem)
		case cd.role == Stop:
			stops = append(stops, cd)
		}
	}
	if len(stops) < c.Rules.MinStops {
		for _, cd := range cands {
			if cd.role == Stop && cd.problem != "" {
				problems = append(problems, cd.problem)
			}
		}
		problems = append(problems, fmt.Sprintf("too few places on the theme could be used: plan at least %d", c.Rules.MinStops+1))
	}
	if len(problems) > 0 {
		return Walk{}, problems, drops, nil
	}
	if len(stops) > c.Rules.MaxStops {
		stops = stops[:c.Rules.MaxStops]
	}

	meet, finish := cands[0], cands[len(cands)-1]
	route, stops, outliers, err := c.route(ctx, req, meet, stops, finish)
	if err != nil {
		return Walk{}, nil, drops, err
	}
	for _, o := range outliers {
		drops = append(drops, Drop{Role: Stop, Name: o.place.Name, Reason: DropOutlier})
	}
	w := Walk{Title: strings.TrimSpace(p.Title), Summary: strings.TrimSpace(p.Summary), Route: route.Route}
	walked := slices.Concat([]*candidate{meet}, stops, []*candidate{finish})
	for i, cd := range walked {
		s := WalkStop{Role: cd.role, PlaceID: cd.found.ID, Minutes: minutesAt(cd), Note: strings.TrimSpace(cd.place.Note)}
		if i > 0 {
			s.Walk = route.Legs[i-1].Duration
		}
		w.Stops = append(w.Stops, s)
	}

	var long []string
	for i, l := range route.Legs {
		if l.Duration > c.Rules.MaxLeg {
			long = append(long, walked[i].place.Name+" to "+walked[i+1].place.Name)
		}
	}
	if len(long) > 0 {
		problems = append(problems, fmt.Sprintf("the walk from %s is too long: keep every walk between places under 15 minutes", strings.Join(long, " and from ")))
	}
	if route.Duration > c.Rules.MaxWalking {
		problems = append(problems, "too much walking in all: keep the places closer together")
	}
	switch m := w.Minutes(); {
	case m > c.Rules.MaxMinutes:
		problems = append(problems, "the outing is too long: aim for about 3 hours in all, walking included")
	case m < c.Rules.MinMinutes:
		problems = append(problems, "the outing is too short: aim for about 3 hours in all, walking included")
	}
	return w, problems, drops, nil
}

// minutesAt keeps the model's time at a place within reason for its role.
func minutesAt(cd *candidate) int {
	switch cd.role {
	case Meet:
		return min(max(cd.place.Minutes, 5), 15)
	case Finish:
		return min(max(cd.place.Minutes, 30), 90)
	}
	return min(max(cd.place.Minutes, 5), 60)
}

// resolve finds every place at once and records what's wrong with each.
func (c *Composer) resolve(ctx context.Context, req Request, cands []*candidate) error {
	var wg sync.WaitGroup
	errs := make([]error, len(cands))
	for i, cd := range cands {
		wg.Go(func() {
			query := strings.TrimSpace(cd.place.Search)
			if query == "" {
				query = cd.place.Name
			}
			// The city disambiguates names common across a country.
			query += ", " + req.City
			f, ok, err := c.Places.Find(ctx, maps.FindRequest{Query: query, Near: req.Center, RadiusM: c.Rules.MaxFromCenterM})
			switch {
			case err != nil:
				errs[i] = fmt.Errorf("find %q: %w", query, err)
			case !ok, !sameName(cd.place.Name, f.Name, req):
				// Text Search returns its nearest match even for a name that
				// doesn't exist: a different place is "not found" too.
				cd.problem, cd.reason = fmt.Sprintf("%s: not found on Google Maps", cd.place.Name), DropNotFound
			case f.Closed:
				cd.problem, cd.reason = fmt.Sprintf("%s: closed", cd.place.Name), DropClosed
			case maps.DistanceM(req.Center, f.Location) > c.Rules.MaxFromCenterM:
				cd.problem, cd.reason = fmt.Sprintf("%s: not in or near %s", cd.place.Name, req.Area), DropFar
			case !is(f, cd.place.Kind):
				cd.problem = fmt.Sprintf("%s: Google Maps doesn't list it as a %s", cd.place.Name, strings.ReplaceAll(cd.place.Kind, "_", " "))
				cd.reason = DropWrongKind
			}
			cd.found = f
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return err
	}
	// The same place twice (two names for one temple) counts once.
	seen := map[string]string{}
	for _, cd := range cands {
		if cd.problem != "" {
			continue
		}
		if first, ok := seen[cd.found.ID]; ok {
			cd.problem, cd.reason = fmt.Sprintf("%s: the same place as %s", cd.place.Name, first), DropDuplicate
			continue
		}
		seen[cd.found.ID] = cd.place.Name
	}
	return nil
}

// route walks meet → stops → finish, letting Google order the stops. While
// a walk between two stops is too long and there are stops to spare, the
// stop at the end of the longest such walk is dropped and the walk routed
// again: an outlier is better left out than walked to.
func (c *Composer) route(ctx context.Context, req Request, meet *candidate, stops []*candidate, finish *candidate) (maps.WalkRoute, []*candidate, []*candidate, error) {
	var dropped []*candidate
	for {
		ids := []string{meet.found.ID}
		for _, s := range stops {
			ids = append(ids, s.found.ID)
		}
		ids = append(ids, finish.found.ID)
		r, err := c.Routes.WalkThrough(ctx, ids, true)
		if err != nil {
			return maps.WalkRoute{}, nil, dropped, fmt.Errorf("route: %w", err)
		}
		ordered := make([]*candidate, 0, len(stops))
		for _, i := range r.Order[1 : len(r.Order)-1] {
			ordered = append(ordered, stops[i-1])
		}
		stops = ordered
		worst, worstLeg := -1, c.Rules.MaxLeg
		for i, l := range r.Legs {
			if l.Duration > worstLeg {
				worst, worstLeg = i, l.Duration
			}
		}
		if worst < 0 || len(stops) <= c.Rules.MinStops {
			return r, stops, dropped, nil
		}
		// Leg i ends at walked place i+1: a stop unless it's the finish, in
		// which case drop the stop it starts from.
		drop := worst
		if worst == len(stops) {
			drop = worst - 1
		}
		c.log().Info("walk stop dropped: too far", "area", req.Area, "place", stops[drop].place.Name)
		dropped = append(dropped, stops[drop])
		stops = slices.Delete(stops, drop, drop+1)
	}
}
