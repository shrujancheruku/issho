package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

const (
	// WalkRetryAfter is how long an area and activity rest after a walk
	// couldn't be composed there, so the worker doesn't pay to fail the same
	// way every hour. Places and the model change slowly.
	WalkRetryAfter = 7 * 24 * time.Hour
	// maxWalksPerPass bounds the walks composed in one pass (minutes each,
	// and about $0.25): the rest wait for the next pass.
	maxWalksPerPass = 4
	// suggestRounds bounds how often a pass asks matching again after
	// walks failed, each time for the next best plans.
	suggestRounds = 3
)

// WalkStop is one stop of a composed walk, as stored.
type WalkStop struct {
	Role    walks.Role
	PlaceID string
	Minutes int
	Note    []byte // {"en": "..."}
	// Walk is the walk here from the previous stop; 0 for the first stop or
	// once its route has been cleared (Maps terms).
	Walk time.Duration
}

// LoadWalk reads a composed walk's stops in order (none for other plans).
func LoadWalk(ctx context.Context, q *dbgen.Queries, planID uuid.UUID) ([]WalkStop, error) {
	rows, err := q.ListPlanStops(ctx, planID)
	if err != nil {
		return nil, err
	}
	out := make([]WalkStop, len(rows))
	for i, r := range rows {
		out[i] = WalkStop{Role: walks.Role(r.Role), PlaceID: r.PlaceID, Minutes: int(r.Minutes), Note: r.Note}
		if r.WalkSeconds != nil {
			out[i].Walk = time.Duration(*r.WalkSeconds) * time.Second
		}
	}
	return out, nil
}

// english wraps English text as stored: {"en": "..."}.
func english(s string) []byte {
	b, _ := json.Marshal(map[string]string{i18n.Authoring: s})
	return b
}

// CreateWalk opens a plan for a suggestion with its composed walk: the
// plan, its title and summary, its stops and its route, in one transaction.
func CreateWalk(ctx context.Context, pool *pgxpool.Pool, cityID string, s matching.Suggestion, w Window, walk walks.Walk, status string, now time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		var err error
		id, err = q.InsertPlan(ctx, dbgen.InsertPlanParams{
			Status:            status,
			CityID:            cityID,
			AreaID:            string(s.Area),
			ActivityID:        string(s.Activity),
			PrimaryLanguage:   s.Primary,
			SecondaryLanguage: ptr(s.Secondary),
			Audience:          int32(s.Audience),
			Score:             float32(s.Score),
			WindowStart:       date(w.Start),
			WindowDays:        int16(w.Days),
			SignupDeadline:    pgtype.Timestamptz{Time: w.Deadline, Valid: true},
			Title:             english(walk.Title),
			Description:       english(walk.Summary),
		})
		if err != nil {
			return err
		}
		p := dbgen.InsertPlanStopsParams{PlanID: id}
		for i, st := range walk.Stops {
			p.Positions = append(p.Positions, int16(i))
			p.Roles = append(p.Roles, string(st.Role))
			p.PlaceIds = append(p.PlaceIds, st.PlaceID)
			p.Minutes = append(p.Minutes, int16(st.Minutes))
			p.Notes = append(p.Notes, english(st.Note))
			secs := int32(-1)
			if i > 0 {
				secs = int32(st.Walk.Seconds())
			}
			p.WalkSeconds = append(p.WalkSeconds, secs)
		}
		if err := q.InsertPlanStops(ctx, p); err != nil {
			return err
		}
		return q.SetPlanRoute(ctx, dbgen.SetPlanRouteParams{
			ID: id, Polyline: &walk.Route.Polyline, Seconds: ptrInt32(int32(walk.Route.Duration.Seconds())),
			At: pgtype.Timestamptz{Time: now, Valid: true},
		})
	})
	return id, err
}

// walkPlaces is what the composer is told about a city: the city pack's
// English names and the areas' centres.
type walkPlaces struct {
	city  string
	areas map[matching.AreaID]struct {
		name   string
		center maps.LatLng
	}
	activities map[matching.ActivityID]string // English names
}

func loadWalkPlaces(ctx context.Context, q *dbgen.Queries, cityID string) (*walkPlaces, error) {
	wp := &walkPlaces{
		areas: map[matching.AreaID]struct {
			name   string
			center maps.LatLng
		}{},
		activities: map[matching.ActivityID]string{},
	}
	cities, err := q.ListCities(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range cities {
		if c.ID == cityID {
			wp.city = i18n.Pick(c.Name, i18n.Authoring)
		}
	}
	areas, err := q.ListAreas(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range areas {
		if a.CityID == cityID {
			wp.areas[matching.AreaID(a.ID)] = struct {
				name   string
				center maps.LatLng
			}{i18n.Pick(a.Name, i18n.Authoring), maps.LatLng{Lat: a.Lat, Lng: a.Lng}}
		}
	}
	acts, err := q.ListCityActivities(ctx, cityID)
	if err != nil {
		return nil, err
	}
	for _, a := range acts {
		wp.activities[matching.ActivityID(a.ID)] = WithoutExamples(i18n.Pick(a.Name, i18n.Authoring))
	}
	return wp, nil
}

// suggest opens plans for a city's suggestions. Walks are composed first
// (a few at a time, at once); a walk that can't be composed isn't opened,
// and matching is asked again for the next best plan in its place.
func (r *Runner) suggest(ctx context.Context, q *dbgen.Queries, c *City, in matching.SuggestInput, w Window, now time.Time) (int, error) {
	acts := c.ActivitiesByID()
	failures, err := q.ListRecentWalkFailures(ctx, dbgen.ListRecentWalkFailuresParams{
		CityID: c.ID, Since: pgtype.Timestamptz{Time: now.Add(-WalkRetryAfter), Valid: true},
	})
	if err != nil {
		return 0, err
	}
	for _, f := range failures {
		in.Skip = append(in.Skip, matching.Plan{Area: matching.AreaID(f.AreaID), Activity: matching.ActivityID(f.ActivityID)})
	}
	var places *walkPlaces
	opened, composed := 0, 0
	for range suggestRounds {
		var walkIdx []int
		suggestions := matching.Suggest(in, c.Config)
		for i, s := range suggestions {
			if a := acts[s.Activity]; a != nil && a.Walk {
				walkIdx = append(walkIdx, i)
			}
		}
		results := make([]walkResult, len(suggestions))
		if len(walkIdx) > 0 && r.Walks != nil && places == nil {
			if places, err = loadWalkPlaces(ctx, q, c.ID); err != nil {
				return opened, err
			}
		}
		var wg sync.WaitGroup
		for _, i := range walkIdx {
			if r.Walks == nil || composed == maxWalksPerPass {
				results[i].skip = true
				continue
			}
			composed++
			s := suggestions[i]
			area := places.areas[s.Area]
			req := walks.Request{
				City: places.city, Area: area.name, Center: area.center,
				Activity: places.activities[s.Activity], Theme: c.WalkThemes[s.Activity], Month: w.Start.Month(),
			}
			for _, b := range acts[s.Activity].Blocks {
				req.Blocks = append(req.Blocks, string(b))
			}
			wg.Go(func() { results[i].walk, results[i].err = r.Walks.Compose(ctx, req) })
		}
		wg.Wait()

		skipped := false
		for i, s := range suggestions {
			res := results[i]
			var rejected *walks.Rejected
			switch {
			case res.skip:
				// No composer, or this pass's walks are used up: next pass.
				skipped = true
				in.Skip = append(in.Skip, s.Plan)
				continue
			case errors.As(res.err, &rejected):
				r.Log.Info("walk not composed", "city", c.ID, "area", s.Area, "activity", s.Activity, "reasons", rejected.Reasons)
				if err := q.UpsertWalkFailure(ctx, dbgen.UpsertWalkFailureParams{
					AreaID: string(s.Area), ActivityID: string(s.Activity),
					FailedAt: pgtype.Timestamptz{Time: now, Valid: true}, Reason: rejected.Error(),
				}); err != nil {
					return opened, err
				}
				skipped = true
				in.Skip = append(in.Skip, s.Plan)
				continue
			case res.err != nil:
				// The model or Maps failing isn't the area's fault: no rest.
				r.Log.Warn("walk compose", "city", c.ID, "area", s.Area, "activity", s.Activity, "err", res.err)
				skipped = true
				in.Skip = append(in.Skip, s.Plan)
				continue
			}
			var id uuid.UUID
			// In a city under review the plan waits as a draft (00021).
			status := PlanOpen
			if c.Review {
				status = PlanDraft
			}
			if acts[s.Activity] != nil && acts[s.Activity].Walk {
				id, err = CreateWalk(ctx, r.DB, c.ID, s, w, res.walk, status, now)
			} else {
				id, err = create(ctx, q, c.ID, s, w, acts, status)
			}
			if err != nil {
				return opened, fmt.Errorf("open plan %s in %s: %w", s.Activity, s.Area, err)
			}
			if acts[s.Activity] != nil && acts[s.Activity].Walk {
				r.Log.Info("walk opened", "city", c.ID, "plan", id, "status", status, "area", s.Area, "activity", s.Activity,
					"title", res.walk.Title, "stops", len(res.walk.Stops), "minutes", res.walk.Minutes())
			}
			opened++
			in.Open = append(in.Open, s.Plan)
		}
		if !skipped {
			break
		}
	}
	return opened, nil
}

// Contexts tell the translator what a walk's text is.
const (
	summaryContext = "one or two sentences describing a walking outing for a small group"
	noteContext    = "a note to a small group about one stop on their walk: why it's worth the stop and what to look at"
)

// text returns stored text ({"en": "..."}) in the caller's two display
// languages (secondary "" when the same or not chosen), translated and
// cached like titles; English without a translator.
func (s *Service) text(ctx context.Context, raw []byte, hint string, caller Caller) (string, string) {
	if raw == nil {
		return "", ""
	}
	out := s.texts(ctx, [][]byte{raw}, hint, caller)
	return out[0][0], out[0][1]
}

// texts is text for several texts at once.
func (s *Service) texts(ctx context.Context, raws [][]byte, hint string, caller Caller) [][2]string {
	texts := make([]translate.Text, len(raws))
	for i, r := range raws {
		texts[i] = translate.Text{Raw: r, Context: hint}
	}
	primary, secondary := make([]string, len(raws)), make([]string, len(raws))
	if s.Translations != nil {
		primary = s.Translations.Resolve(ctx, caller.Primary, texts)
		if caller.Secondary != "" {
			secondary = s.Translations.Resolve(ctx, caller.Secondary, texts)
		}
	}
	out := make([][2]string, len(raws))
	for i, r := range raws {
		out[i][0], out[i][1] = i18n.Pair(primary[i], secondary[i], i18n.Pick(r, i18n.Authoring))
	}
	return out
}

// walkSteps lists a composed walk's stops: each place fetched live
// (fillVenues), the time there, the walk from the previous stop (Google's,
// or estimated once the stored route has been cleared) and the note in the
// caller's languages.
func (s *Service) walkSteps(ctx context.Context, q *dbgen.Queries, planID uuid.UUID, caller Caller) ([]Step, error) {
	stops, err := LoadWalk(ctx, q, planID)
	if err != nil {
		return nil, err
	}
	steps := make([]Step, len(stops))
	venues := make([]string, len(stops))
	notes := make([][]byte, len(stops))
	for i, st := range stops {
		steps[i] = Step{Meeting: st.Role == walks.Meet, Finish: st.Role == walks.Finish, Minutes: st.Minutes}
		if st.Walk > 0 {
			steps[i].WalkMinutes = (&Route{Seconds: int(st.Walk.Seconds())}).Minutes()
		}
		venues[i], notes[i] = st.PlaceID, st.Note
	}
	for i, n := range s.texts(ctx, notes, noteContext, caller) {
		steps[i].Note, steps[i].SecondaryNote = n[0], n[1]
	}
	if s.Places == nil {
		return steps, nil
	}
	s.fillVenues(ctx, steps, venues, caller)
	for i := 1; i < len(steps); i++ {
		if steps[i].WalkMinutes == 0 {
			steps[i].WalkMinutes = walkMinutes(steps[i-1].Venue.Location, steps[i].Venue.Location)
		}
	}
	return steps, nil
}

type walkResult struct {
	walk walks.Walk
	err  error
	skip bool
}

// checkWalk is checkVenues for a composed walk: every stop must be open
// from when the group gets there until it leaves, estimating 10 minutes
// for a walk whose route has been cleared.
func (r *Runner) checkWalk(ctx context.Context, p Open, start time.Time, cache map[string]*maps.Hours) venueProblem {
	stops, err := LoadWalk(ctx, dbgen.New(r.DB), p.ID)
	if err != nil {
		r.Log.Warn("walk stops", "plan", p.ID, "err", err)
		return venuesOK
	}
	at := start
	for i, s := range stops {
		if i > 0 {
			walk := s.Walk
			if walk == 0 {
				walk = 10 * time.Minute
			}
			at = at.Add(walk)
		}
		stay := time.Duration(s.Minutes) * time.Minute
		if v := r.checkPlace(ctx, p, s.PlaceID, []time.Time{at, at.Add(max(stay-time.Minute, 0))}, cache); v != venuesOK {
			return v
		}
		at = at.Add(stay)
	}
	return venuesOK
}

// refreshWalkRoutes is RefreshRoutes for composed walks: their route goes
// through every stop, in the stored order (Google isn't asked to reorder
// them again: the stops and notes are written for that order).
func (r *Runner) refreshWalkRoutes(ctx context.Context, q *dbgen.Queries, cityID string, now time.Time) (int, error) {
	plans, err := q.ListWalksNeedingRoute(ctx, dbgen.ListWalksNeedingRouteParams{
		CityID: cityID, OlderThan: pgtype.Timestamptz{Time: now.Add(-RouteRefreshAge), Valid: true}, Max: routeBatch,
	})
	if err != nil {
		return 0, err
	}
	computed := 0
	var errs []error
	for _, p := range plans {
		walk, err := r.WalkRoutes.WalkThrough(ctx, p.PlaceIds, false)
		if err != nil {
			errs = append(errs, fmt.Errorf("walk %s: %w", p.ID, err))
			continue
		}
		legs := dbgen.SetWalkLegsParams{PlanID: p.ID}
		for i, l := range walk.Legs {
			legs.Positions = append(legs.Positions, int16(i+1))
			legs.Seconds = append(legs.Seconds, int32(l.Duration.Seconds()))
		}
		err = pgx.BeginFunc(ctx, r.DB, func(tx pgx.Tx) error {
			tq := dbgen.New(tx)
			if err := tq.SetWalkLegs(ctx, legs); err != nil {
				return err
			}
			return tq.SetPlanRoute(ctx, dbgen.SetPlanRouteParams{
				ID: p.ID, Polyline: &walk.Polyline, Seconds: ptrInt32(int32(walk.Duration.Seconds())),
				At: pgtype.Timestamptz{Time: now, Valid: true},
			})
		})
		if err != nil {
			return computed, err
		}
		computed++
	}
	return computed, errors.Join(errs...)
}
