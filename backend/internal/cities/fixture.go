package cities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/internal/cities/store"
)

// fixtureCity is this package's part of a fixture file (backend/fixtures): the
// "city" key. The file's other keys are other packages'.
type fixtureCity struct {
	Slug     string            `json:"slug"`
	State    State             `json:"state"`
	TimeZone string            `json:"time_zone"`
	Currency string            `json:"currency"`
	Names    map[string]string `json:"names"`
}

// Beginner starts the transaction LoadFixture writes in: a pool, or a
// transaction (which then gives it a savepoint, so the caller can do other
// work in the same transaction, as cmd/seed does).
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// LoadFixture writes the fixture's city, replacing a city of the same slug.
// It is for development and tests only: cmd/seed and the tests call it, and no
// process in the production image does (see package fixtures).
//
// A field the city doesn't know is an error, so a typo in the file can't
// silently drop one. Other packages' top-level keys ("plans") are ignored,
// since each package reads only its own.
func LoadFixture(ctx context.Context, pool Beginner, data []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return fmt.Errorf("cities: fixture: %w", err)
	}
	raw, ok := top["city"]
	if !ok {
		return errors.New("cities: fixture has no city")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var c fixtureCity
	if err := dec.Decode(&c); err != nil {
		return fmt.Errorf("cities: fixture city: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cities: fixture: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)
	if err := q.UpsertCity(ctx, store.UpsertCityParams{Slug: c.Slug, State: string(c.State), TimeZone: c.TimeZone, Currency: c.Currency}); err != nil {
		return fmt.Errorf("cities: fixture city %q: %w", c.Slug, err)
	}
	for lang, name := range c.Names {
		if err := q.UpsertCityName(ctx, store.UpsertCityNameParams{City: c.Slug, Language: lang, Name: name}); err != nil {
			return fmt.Errorf("cities: fixture name %q: %w", lang, err)
		}
	}
	return tx.Commit(ctx)
}
