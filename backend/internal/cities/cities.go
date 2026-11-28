// Package cities owns the cities: which cities are listed and each one's time
// zone, currency and names (docs/spec/cities.md, docs/spec/city-settings.md).
// The tables are app.cities and app.city_names (db/migrations/00004_cities_and_plans.sql);
// no other package reads them.
//
// The city's other settings, its areas and its calendar are later tickets',
// each in a table of its own keyed by the city's slug.
package cities

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/cities/store"
)

// State is a city's state (docs/spec/cities.md, "A city's states").
type State string

const (
	Planned State = "planned"
	Open    State = "open"
	Closed  State = "closed"
)

// ErrNotFound: no city has that slug.
var ErrNotFound = errors.New("cities: no such city")

// City is a city and the settings that are its own so far.
type City struct {
	// Slug is the city's id, e.g. "da-nang-hoi-an".
	Slug  string
	State State
	// TimeZone is an IANA name, e.g. "Asia/Ho_Chi_Minh"; build the city's clock
	// from it with cityclock.New.
	TimeZone string
	// Currency is an ISO 4217 code. Prices are in it and never converted.
	Currency string
	// Names is the city's name by language: "en" and "vi" for Da Nang–Hoi An.
	Names map[string]string
}

// Service is the exported API of cities.
type Service struct {
	pool *pgxpool.Pool
}

// New returns a Service that reads cities from pool.
func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Get returns the city with that slug, or ErrNotFound.
func (s *Service) Get(ctx context.Context, slug string) (City, error) {
	q := store.New(s.pool)
	row, err := q.GetCity(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return City{}, ErrNotFound
	}
	if err != nil {
		return City{}, fmt.Errorf("cities: get: %w", err)
	}
	names, err := q.GetCityNames(ctx, slug)
	if err != nil {
		return City{}, fmt.Errorf("cities: get names: %w", err)
	}
	c := City{Slug: row.Slug, State: State(row.State), TimeZone: row.TimeZone, Currency: row.Currency, Names: map[string]string{}}
	for _, n := range names {
		c.Names[n.Language] = n.Name
	}
	return c, nil
}
