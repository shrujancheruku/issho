// Command seed loads the development fixture (backend/fixtures): the city Da
// Nang–Hoi An and its two plans, into a migrated database. It is for
// development only, behind `make seed`:
//
//   - It refuses any database that isn't on this machine, before it connects,
//     so a DATABASE_URL left in the environment can't seed a hosted project.
//
//   - The production image doesn't build it (backend/Dockerfile builds api and
//     worker), and neither of those imports the fixtures
//     (TestProductionProcessesDoNotImportTheFixtures).
//
//   - One explicit opt-in reaches the hosted dev database (and only it): the
//     --hosted-dev flag together with SEED_DEV_PROJECT_REF, the dev project's
//     ref. The target must be that project's own host or pooler user
//     (checkTarget), so a production URL is refused even with the opt-in.
//
//   - The ref comes from the operator, so it alone can't tell dev from
//     production. With the opt-in the seed also reads the database's own marker,
//     `issho.environment`, and writes nothing unless it is exactly `dev`
//     (checkEnvironment). The founder sets it once on issho-dev, never on
//     production, and nothing in the repo sets it.
//
// Running it again replaces the fixture's rows with the fixture's.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/fixtures"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

func main() {
	hostedDev := flag.Bool("hosted-dev", false, "seed the hosted dev database; needs SEED_DEV_PROJECT_REF, the dev project's ref")
	flag.Parse()
	if err := run(context.Background(), os.Getenv("DATABASE_URL"), *hostedDev, os.Getenv("SEED_DEV_PROJECT_REF")); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, databaseURL string, hostedDev bool, devRef string) error {
	if databaseURL == "" {
		return errors.New("DATABASE_URL is not set")
	}
	if err := checkTarget(databaseURL, hostedDev, devRef); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return errors.New("DATABASE_URL could not be used")
	}
	defer pool.Close()
	if hostedDev {
		// The local path never reads the marker; a local database has none.
		if err := checkEnvironment(ctx, pool); err != nil {
			return err
		}
	}
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

// refPattern is the shape of a Supabase project ref. It also keeps the ref
// from carrying anything that could change what the comparison below means.
var refPattern = regexp.MustCompile(`^[a-z0-9]{8,32}$`)

// checkTarget is whether the database may be seeded. A database on this machine
// always may. Any other only with the hosted-dev opt-in, and then only if it is
// the dev project's: devRef (given by the operator, never taken from the URL)
// must be the ref in the URL's host (`db.<ref>.supabase.co`) or in the pooler's
// user (`postgres.<ref>`). A production URL carries production's ref, so it
// fails here even with the opt-in. The errors never echo the URL (it holds a
// password).
func checkTarget(databaseURL string, hostedDev bool, devRef string) error {
	if isLocal(databaseURL) {
		return nil
	}
	if !hostedDev {
		return errors.New("DATABASE_URL is not a database on this machine: the fixture is for development only")
	}
	if !refPattern.MatchString(devRef) {
		return errors.New("--hosted-dev needs SEED_DEV_PROJECT_REF set to the dev project's ref")
	}
	u, err := url.Parse(databaseURL)
	if err != nil || u.Host == "" {
		return errors.New("DATABASE_URL could not be used")
	}
	for _, k := range []string{"host", "hostaddr", "service"} {
		if u.Query().Has(k) {
			return errors.New("DATABASE_URL has a query that could change its host")
		}
	}
	if u.Hostname() == "db."+devRef+".supabase.co" {
		return nil
	}
	if strings.HasSuffix(u.Hostname(), ".pooler.supabase.com") && u.User != nil && u.User.Username() == "postgres."+devRef {
		return nil
	}
	return errors.New("DATABASE_URL is not the dev project's database (SEED_DEV_PROJECT_REF): --hosted-dev reaches dev only")
}

// checkEnvironment refuses unless the database says it is dev: the setting
// `issho.environment` (`alter database postgres set issho.environment = 'dev'`,
// run once by the founder on issho-dev) must be exactly `dev`. Unlike the ref,
// the operator can't supply it: it is stored in the database reached, so
// production's URL fails here even with production's ref passed as the dev
// ref. The missing-ok form returns NULL when it was never set.
func checkEnvironment(ctx context.Context, pool *pgxpool.Pool) error {
	var env *string
	if err := pool.QueryRow(ctx, `select current_setting('issho.environment', true)`).Scan(&env); err != nil {
		return errors.New("could not read the database's issho.environment setting")
	}
	if env == nil || *env != "dev" {
		return errors.New("--hosted-dev seeds only a database whose issho.environment setting is 'dev'; the founder sets it once on issho-dev (see docs/deploy.md)")
	}
	return nil
}
