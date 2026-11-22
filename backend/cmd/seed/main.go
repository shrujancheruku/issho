// Command seed loads the development fixture (backend/fixtures): the city Da
// Nang–Hoi An and its two plans, into a migrated database. It is for
// development only, behind `make seed`:
//
//   - It refuses any database that isn't on this machine, before it connects,
//     so a DATABASE_URL left in the environment can't seed a hosted project.
//   - The production image doesn't build it (backend/Dockerfile builds api and
//     worker), and neither of those imports the fixtures
//     (TestProductionProcessesDoNotImportTheFixtures).
//
// Running it again replaces the fixture's rows with the fixture's.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/fixtures"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

func main() {
	if err := run(context.Background(), os.Getenv("DATABASE_URL")); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, databaseURL string) error {
	if databaseURL == "" {
		return errors.New("DATABASE_URL is not set")
	}
	if !isLocal(databaseURL) {
		// The URL holds a password, so it is not echoed.
		return errors.New("DATABASE_URL is not a database on this machine: the fixture is for development only")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return errors.New("DATABASE_URL could not be used")
	}
	defer pool.Close()
	// The city first: the plans refer to it.
	if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		return err
	}
	return plans.LoadFixture(ctx, pool, fixtures.DaNangHoiAn)
}

// isLocal is whether the URL's host is this machine: localhost or a loopback
// address. The local Supabase stack is at 127.0.0.1:54322.
func isLocal(databaseURL string) bool {
	u, err := url.Parse(databaseURL)
	if err != nil || u.Host == "" {
		return false
	}
	// A query can override the host (`?host=...`) or name a service file, so the
	// host above would not be the one connected to.
	for _, k := range []string{"host", "hostaddr", "service"} {
		if u.Query().Has(k) {
			return false
		}
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
