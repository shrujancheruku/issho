package plans

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/plans/store"
)

// Fixture shapes: this package's part of a fixture file (backend/fixtures),
// the "plans" key. Text is keyed by language.
type fixturePlan struct {
	ID             string                 `json:"id"`
	City           string                 `json:"city"`
	Price          int32                  `json:"price"`
	MeetingAddress string                 `json:"meeting_address"`
	PartsOfDay     []string               `json:"parts_of_day"`
	StartsAt       string                 `json:"starts_at"`
	EndsAt         string                 `json:"ends_at"`
	Texts          map[string]fixtureText `json:"texts"`
	Stops          []fixtureStop          `json:"stops"`
}

type fixtureText struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	MeetingPoint string   `json:"meeting_point"`
	WhatToBring  []string `json:"what_to_bring"`
}

type fixtureStop struct {
	StayMinutes int32 `json:"stay_minutes"`
	Leg         *struct {
		Mode    string `json:"mode"`
		Minutes int32  `json:"minutes"`
	} `json:"leg"`
	Texts map[string]struct {
		Name string `json:"name"`
		Note string `json:"note"`
	} `json:"texts"`
}

// LoadFixture writes the fixture's plans, replacing any plan of the same id
// whole. Their city must already be there (cities.LoadFixture first). It is
// for development and tests only: cmd/seed and the tests call it, and no
// process in the production image does (see package fixtures).
//
// A field a plan doesn't know is an error, so a typo in the file can't
// silently drop one. Other packages' top-level keys ("city") are ignored.
// Stops are numbered from 1 in the order the file lists them.
func LoadFixture(ctx context.Context, pool *pgxpool.Pool, data []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return fmt.Errorf("plans: fixture: %w", err)
	}
	raw, ok := top["plans"]
	if !ok {
		return errors.New("plans: fixture has no plans")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var fixture []fixturePlan
	if err := dec.Decode(&fixture); err != nil {
		return fmt.Errorf("plans: fixture plans: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("plans: fixture: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)
	for _, p := range fixture {
		if err := writePlan(ctx, q, p); err != nil {
			return fmt.Errorf("plans: fixture plan %q: %w", p.ID, err)
		}
	}
	return tx.Commit(ctx)
}

func writePlan(ctx context.Context, q *store.Queries, p fixturePlan) error {
	// Deleting cascades to the plan's texts, stops and stop texts.
	if err := q.DeletePlan(ctx, p.ID); err != nil {
		return err
	}
	if err := q.InsertPlan(ctx, store.InsertPlanParams{
		ID: p.ID, City: p.City, Price: p.Price, MeetingAddress: p.MeetingAddress,
		PartsOfDay: p.PartsOfDay, StartsAt: p.StartsAt, EndsAt: p.EndsAt,
	}); err != nil {
		return err
	}
	for lang, t := range p.Texts {
		if err := q.InsertPlanText(ctx, store.InsertPlanTextParams{
			PlanID: p.ID, Language: lang, Title: t.Title, Description: t.Description,
			MeetingPoint: t.MeetingPoint, WhatToBring: t.WhatToBring,
		}); err != nil {
			return err
		}
	}
	for i, st := range p.Stops {
		position := int32(i + 1)
		params := store.InsertPlanStopParams{PlanID: p.ID, Position: position, StayMinutes: st.StayMinutes}
		if st.Leg != nil {
			params.LegMode, params.LegMinutes = &st.Leg.Mode, &st.Leg.Minutes
		}
		if err := q.InsertPlanStop(ctx, params); err != nil {
			return err
		}
		for lang, t := range st.Texts {
			if err := q.InsertPlanStopText(ctx, store.InsertPlanStopTextParams{
				PlanID: p.ID, Position: position, Language: lang, Name: t.Name, Note: t.Note,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
