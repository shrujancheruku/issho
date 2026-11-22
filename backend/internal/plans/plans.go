// Package plans owns plans: the fields a plan page shows and the prose in each
// language (docs/spec/plans.md, docs/spec/languages.md). The tables are
// app.plans, app.plan_texts, app.plan_stops and app.plan_stop_texts
// (db/migrations/00004_cities_and_plans.sql); no other package reads them.
//
// So far plans are read only: a fixture seeds them in development and tests
// (LoadFixture). Making, reviewing and opening plans are later tickets'.
package plans

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/plans/store"
)

// FallbackLanguage is the language a plan reads in when it has no text in the
// one asked for: English is the source the owner writes and the fallback for
// every language (docs/spec/languages.md).
const FallbackLanguage = "en"

// ErrNotFound: no plan has that id. An id that could not be a plan's is the
// same error, so a caller can't tell a malformed id from a missing plan.
var ErrNotFound = errors.New("plans: no such plan")

// Plan is what a plan page shows. The prose is in Language; every other field
// is the same in every language, so a translation can't change what must be
// true on the day.
type Plan struct {
	ID   string
	City string
	// Language is the language of the prose: the one asked for, or
	// FallbackLanguage when the plan has no text in it.
	Language    string
	Title       string
	Description string
	// Price is per person, in whole units of the city's currency.
	Price int
	// MeetingAddress is the meeting point's street address, never translated.
	// MeetingPoint is what to look for there.
	MeetingAddress string
	MeetingPoint   string
	WhatToBring    []string
	// PartsOfDay are the parts of the day (morning, afternoon, evening) the
	// plan can run in.
	PartsOfDay []string
	// StartsAt and EndsAt are "HH:MM" on the city's clock.
	StartsAt, EndsAt string
	// Stops are in order.
	Stops []Stop
}

// Stop is one stop of a plan.
type Stop struct {
	// Position counts from 1.
	Position    int
	StayMinutes int
	// Name is the place's name in the plan's language; Note is the maker's note.
	Name string
	Note string
	// Leg is the way to the next stop, nil for the last stop.
	Leg *Leg
}

// Leg is the way between two stops: its mode ("walk", "ride" or "transit") and
// minutes are fields, not prose.
type Leg struct {
	Mode    string
	Minutes int
}

// Service is the exported API of plans.
type Service struct {
	pool *pgxpool.Pool
}

// New returns a Service that reads plans from pool.
func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

var idRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// List returns the city's plans with their text in language, by start time. A
// city with none, or no such city, lists none: whether the city is listed is
// the cities package's to say.
func (s *Service) List(ctx context.Context, city, language string) ([]Plan, error) {
	rows, err := store.New(s.pool).ListPlansInCity(ctx, city)
	if err != nil {
		return nil, fmt.Errorf("plans: list: %w", err)
	}
	return s.assemble(ctx, rows, language)
}

// Get returns one plan with its text in language, or ErrNotFound.
func (s *Service) Get(ctx context.Context, id, language string) (Plan, error) {
	id = strings.ToLower(id)
	if !idRe.MatchString(id) {
		return Plan{}, ErrNotFound
	}
	row, err := store.New(s.pool).GetPlan(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	if err != nil {
		return Plan{}, fmt.Errorf("plans: get: %w", err)
	}
	got, err := s.assemble(ctx, []store.AppPlan{row}, language)
	if err != nil {
		return Plan{}, err
	}
	return got[0], nil
}

// assemble reads the texts and stops of rows' plans, in three queries however
// many plans there are, and builds them in language.
func (s *Service) assemble(ctx context.Context, rows []store.AppPlan, language string) ([]Plan, error) {
	if len(rows) == 0 {
		return []Plan{}, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	q := store.New(s.pool)
	texts, err := q.ListPlanTexts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("plans: texts: %w", err)
	}
	stops, err := q.ListPlanStops(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("plans: stops: %w", err)
	}
	stopTexts, err := q.ListPlanStopTexts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("plans: stop texts: %w", err)
	}

	type stopKey struct {
		plan     string
		position int32
	}
	planText := map[string]map[string]store.AppPlanText{}
	for _, t := range texts {
		if planText[t.PlanID] == nil {
			planText[t.PlanID] = map[string]store.AppPlanText{}
		}
		planText[t.PlanID][t.Language] = t
	}
	stopText := map[stopKey]map[string]store.AppPlanStopText{}
	for _, t := range stopTexts {
		k := stopKey{t.PlanID, t.Position}
		if stopText[k] == nil {
			stopText[k] = map[string]store.AppPlanStopText{}
		}
		stopText[k][t.Language] = t
	}
	stopsOf := map[string][]store.AppPlanStop{}
	for _, st := range stops {
		stopsOf[st.PlanID] = append(stopsOf[st.PlanID], st)
	}

	out := make([]Plan, 0, len(rows))
	for _, r := range rows {
		lang := language
		text, ok := planText[r.ID][lang]
		if !ok {
			lang = FallbackLanguage
			if text, ok = planText[r.ID][lang]; !ok {
				return nil, fmt.Errorf("plans: plan %s has no %s text", r.ID, FallbackLanguage)
			}
		}
		p := Plan{
			ID: r.ID, City: r.City, Language: lang,
			Title: text.Title, Description: text.Description,
			Price:          int(r.Price),
			MeetingAddress: r.MeetingAddress, MeetingPoint: text.MeetingPoint,
			WhatToBring: text.WhatToBring,
			PartsOfDay:  r.PartsOfDay,
			StartsAt:    r.StartsAt, EndsAt: r.EndsAt,
		}
		for _, st := range stopsOf[r.ID] {
			k := stopKey{r.ID, st.Position}
			t, ok := stopText[k][lang]
			if !ok {
				return nil, fmt.Errorf("plans: plan %s stop %d has no %s text", r.ID, st.Position, lang)
			}
			stop := Stop{Position: int(st.Position), StayMinutes: int(st.StayMinutes), Name: t.Name, Note: t.Note}
			if st.LegMode != nil {
				stop.Leg = &Leg{Mode: *st.LegMode, Minutes: int(*st.LegMinutes)}
			}
			p.Stops = append(p.Stops, stop)
		}
		out = append(out, p)
	}
	return out, nil
}
