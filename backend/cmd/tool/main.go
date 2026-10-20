// Command tool runs operational tasks: migrations and seeding city data.
//
//	tool migrate up|down|status|...
//	tool seed [-catalog data/activities.json] data/citypacks/tokyo.json ...
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/shrujancheruku/issho/backend/internal/cities"
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
		return fmt.Errorf("usage: tool migrate|seed ...")
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
		_ = fs.Parse(args[1:])
		n, err := cities.SeedCatalog(ctx, pool, *catalog)
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
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
