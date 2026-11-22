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
//     production, and nothing in the repo sets it. It reads the stored setting
//     (pg_db_role_setting), never the session's, which a URL's `options` could
//     set; a URL that sets any session parameter is refused (checkNoSessionParameters);
//     and the check runs in the transaction that writes, before its first write.
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

	"github.com/jackc/pgx/v5"
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
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return errors.New("DATABASE_URL could not be used")
	}
	if hostedDev {
		if err := checkNoSessionParameters(cfg); err != nil {
			return err
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return errors.New("DATABASE_URL could not be used")
	}
	defer pool.Close()
	// One transaction for the marker check and every write, on one connection:
	// the database that is checked is the database that is written, and
	// nothing is written before the check has passed. (The loaders take a
	// savepoint inside it.)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return errors.New("could not start a transaction on DATABASE_URL")
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	if hostedDev {
		// The local path never reads the marker; a local database has none.
		if err := checkEnvironment(ctx, tx); err != nil {
			return err
		}
	}
	// The city first: the plans refer to it.
	if err := cities.LoadFixture(ctx, tx, fixtures.DaNangHoiAn); err != nil {
		return err
	}
	if err := plans.LoadFixture(ctx, tx, fixtures.DaNangHoiAn); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// checkNoSessionParameters refuses a connection that would send the server any
// runtime parameter at connect: a URL's `options` (`-c name=value`), PGOPTIONS
// or any other `name=value` in the query. Whatever the session says is not the
// database's own answer to "which environment am I", so with --hosted-dev
// nothing is allowed to say it. checkEnvironment doesn't depend on this (it
// reads the stored setting), so this is the second lock. The error never
// echoes the URL.
func checkNoSessionParameters(cfg *pgxpool.Config) error {
	if len(cfg.ConnConfig.RuntimeParams) > 0 {
		return errors.New("--hosted-dev refuses a DATABASE_URL (or PGOPTIONS/PG* environment) that sets session parameters such as options")
	}
	return nil
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

// querier is what checkEnvironment reads through: a pool or a transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// checkEnvironment refuses unless the database says it is dev: the setting
// `issho.environment = dev` must be stored on the database itself
// (`alter database postgres set issho.environment = 'dev'`, run once by the
// founder on issho-dev). It reads pg_db_role_setting, the stored setting for
// this database and no particular role (setrole = 0), and not current_setting:
// a session can set that for itself (`options=-c issho.environment=dev`, `SET`),
// so on production it would read whatever the URL said. The stored setting is
// readable by any role and nothing in a connection can change it. Unlike the
// ref, the operator can't supply it, so production's URL fails here even with
// production's ref passed as the dev ref. Case matters: `Dev` is refused.
func checkEnvironment(ctx context.Context, q querier) error {
	var isDev bool
	err := q.QueryRow(ctx, `
		select exists (
			select 1
			from pg_catalog.pg_db_role_setting s
			join pg_catalog.pg_database d on d.oid = s.setdatabase
			where d.datname = current_database()
			  and s.setrole = 0
			  and 'issho.environment=dev' = any (s.setconfig)
		)`).Scan(&isDev)
	if err != nil {
		return errors.New("could not read the database's issho.environment setting")
	}
	if !isDev {
		return errors.New("--hosted-dev seeds only a database whose stored issho.environment setting is 'dev'; the founder sets it once on issho-dev (see docs/deploy.md)")
	}
	return nil
}
