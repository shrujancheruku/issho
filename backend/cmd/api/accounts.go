package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
)

// defaultCity is the Pilot's only listed city, the city of an account whose
// first call names none (docs/spec/cities.md, "Which city a visitor with no
// account sees"). It becomes a lookup in cities when a second city is listed.
const defaultCity = "da-nang-hoi-an"

// accounts is what the API needs to know who is calling and what to show them:
// the database that keeps accounts, cities and plans, and Supabase Auth's
// tokens to check. Both values are public
// (a connection URL for the app's own role, and the project's Auth URL); the
// API never holds the project's JWT secret.
type accounts struct {
	people  *people.Service
	cities  *cities.Service
	plans   *plans.Service
	signups *plans.Signups
	auth    rpc.Authenticator
	close   func()
}

// newAccounts reads DATABASE_URL (the Supabase pooler's URL for the app) and
// SUPABASE_AUTH_URL (https://<ref>.supabase.co/auth/v1, which is also the
// tokens' issuer, and the keys are at <it>/.well-known/jwks.json). With neither
// set it returns nil: the API then serves only its public RPCs, as a local
// process with no stack does. With one of them set, it is an error: half a
// configuration would serve accounts it can't keep, or keep ones it can't check.
func newAccounts(ctx context.Context, log *slog.Logger, getenv func(string) string, clk clock.Clock) (*accounts, error) {
	dbURL, authURL := getenv("DATABASE_URL"), strings.TrimRight(getenv("SUPABASE_AUTH_URL"), "/")
	switch {
	case dbURL == "" && authURL == "":
		return nil, nil
	case dbURL == "":
		return nil, errors.New("SUPABASE_AUTH_URL is set but DATABASE_URL is not")
	case authURL == "":
		return nil, errors.New("DATABASE_URL is set but SUPABASE_AUTH_URL is not")
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	verifier := people.NewVerifier(clk, people.VerifierConfig{JWKSURL: authURL + "/.well-known/jwks.json", Issuer: authURL, Log: log})
	cs := cities.New(pool)
	return &accounts{
		people:  people.New(clk, pool, cs, people.Config{DefaultCity: defaultCity}),
		cities:  cs,
		plans:   plans.New(clk, pool, cs),
		signups: plans.NewSignups(clk, pool, cs),
		auth:    rpc.NewAuthenticator(verifier),
		close:   pool.Close,
	}, nil
}
