// Command tool runs operational tasks: migrations, seeding city data, and
// building a city's geo data from Google Maps.
//
//	tool migrate up|down|status|...
//	tool seed [-catalog data/activities.json] [-venue-categories data/venue-categories.json] data/citypacks/tokyo.json ...
//	tool build-matrix [-dry-run] <city>
//	tool discover-venues [-dry-run] [-categories cafe,park] <city>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps/google"
	"github.com/shrujancheruku/issho/backend/internal/platform/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("tool failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tool migrate|seed|build-matrix|discover-venues ...")
	}
	ctx := context.Background()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch args[0] {
	case "migrate":
		if len(args) < 2 {
			return fmt.Errorf("usage: tool migrate up|down|status|...")
		}
		return db.Migrate(ctx, pool, args[1], args[2:]...)
	case "seed":
		fs := flag.NewFlagSet("seed", flag.ExitOnError)
		catalog := fs.String("catalog", "../data/activities.json", "activity catalogue JSON")
		venueCats := fs.String("venue-categories", "../data/venue-categories.json", "venue categories JSON")
		_ = fs.Parse(args[1:])
		n, err := cities.SeedCatalog(ctx, pool, *venueCats, *catalog)
		if err != nil {
			return err
		}
		slog.Info("seeded catalogue", "activities", n)
		for _, path := range fs.Args() {
			p, err := cities.SeedPack(ctx, pool, path)
			if err != nil {
				return err
			}
			slog.Info("seeded city pack", "city", p.ID, "areas", len(p.Areas))
		}
		return nil
	case "build-matrix":
		return buildMatrix(ctx, pool, args[1:])
	case "discover-venues":
		return discoverVenues(ctx, pool, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// mapsKey returns the server-side Google Maps key. It's only needed for real
// runs, so -dry-run works without one.
func mapsKey() (string, error) {
	k := os.Getenv("GOOGLE_MAPS_API_KEY")
	if k == "" {
		return "", errors.New("GOOGLE_MAPS_API_KEY is required (or use -dry-run); see docs/google-maps.md")
	}
	return k, nil
}

func cityArg(fs *flag.FlagSet) (string, error) {
	if fs.NArg() != 1 {
		return "", fmt.Errorf("usage: tool %s [flags] <city>", fs.Name())
	}
	return fs.Arg(0), nil
}

func buildMatrix(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("build-matrix", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "print the planned API usage without calling Google")
	_ = fs.Parse(args)
	city, err := cityArg(fs)
	if err != nil {
		return err
	}

	q := dbgen.New(pool)
	areas, modes, err := geo.LoadCity(ctx, q, city)
	if err != nil {
		return err
	}

	jobs := geo.PlanMatrix(areas, modes)
	slog.Info("travel matrix plan", "city", city, "areas", len(areas), "modes", modes,
		"requests", len(jobs), "billed_elements", geo.Elements(jobs))
	if *dryRun {
		return nil
	}
	key, err := mapsKey()
	if err != nil {
		return err
	}
	rows, err := geo.BuildMatrix(ctx, google.New(key), areas, modes)
	if err != nil {
		return err
	}
	deleted, err := geo.SaveMatrix(ctx, pool, city, rows)
	if err != nil {
		return err
	}
	slog.Info("travel matrix saved", "city", city, "rows", len(rows), "stale_deleted", deleted)
	return nil
}

func discoverVenues(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("discover-venues", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "print the planned API usage without calling Google")
	only := fs.String("categories", "", "comma-separated venue category ids to discover (default: all the city uses)")
	_ = fs.Parse(args)
	city, err := cityArg(fs)
	if err != nil {
		return err
	}

	q := dbgen.New(pool)
	areas, _, err := geo.LoadCity(ctx, q, city)
	if err != nil {
		return err
	}
	var onlyIDs []string
	if *only != "" {
		onlyIDs = strings.Split(*only, ",")
	}
	cats, err := geo.LoadVenueCategories(ctx, q, city, onlyIDs)
	if err != nil {
		return err
	}

	jobs := geo.PlanVenues(areas, cats)
	slog.Info("venue discovery plan", "city", city, "areas", len(areas), "categories", len(cats), "requests", len(jobs))
	if *dryRun {
		return nil
	}
	key, err := mapsKey()
	if err != nil {
		return err
	}
	// The city's main language. It only affects ranking: no names are stored.
	langs, err := q.GetCityLanguages(ctx, city)
	if err != nil {
		return err
	}
	venues, err := geo.DiscoverVenues(ctx, google.New(key), areas, cats, langs[0])
	if err != nil {
		return err
	}
	if err := geo.SaveVenues(ctx, pool, city, venues); err != nil {
		return err
	}
	slog.Info("venues saved", "city", city, "venues", len(venues))
	return nil
}
