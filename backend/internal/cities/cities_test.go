package cities_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/fixtures"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// migrated is a fresh database with every migration applied and no fixture.
func migrated(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testdb.NewDatabase(t)
	if err := db.Up(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// The fixture seeds Da Nang–Hoi An with its time zone, currency and names.
func TestFixtureSeedsTheCity(t *testing.T) {
	ctx := context.Background()
	pool := migrated(t)
	if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}

	got, err := cities.New(pool).Get(ctx, "da-nang-hoi-an")
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "da-nang-hoi-an" || got.TimeZone != "Asia/Ho_Chi_Minh" || got.Currency != "VND" || got.State != cities.Open {
		t.Fatalf("city = %+v", got)
	}
	if got.Names["en"] != "Da Nang–Hoi An" || got.Names["vi"] != "Đà Nẵng – Hội An" {
		t.Fatalf("names = %v", got.Names)
	}
}

// A city that isn't there is ErrNotFound, and so is one nobody seeded.
func TestACityThatIsNotThereIsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := migrated(t)
	if _, err := cities.New(pool).Get(ctx, "da-nang-hoi-an"); !errors.Is(err, cities.ErrNotFound) {
		t.Fatalf("empty database: err = %v, want ErrNotFound", err)
	}
	if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	if _, err := cities.New(pool).Get(ctx, "atlantis"); !errors.Is(err, cities.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// Loading twice leaves the one city, so `make seed` can be run again.
func TestLoadingTheFixtureTwiceChangesNothing(t *testing.T) {
	ctx := context.Background()
	pool := migrated(t)
	for range 2 {
		if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := pool.QueryRow(ctx, `select count(*) from app.cities`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("cities = %d, want 1", n)
	}
}

// A fixture that says something this package doesn't know is refused, so a
// typo in the file can't silently drop a field.
func TestAFixtureWithAnUnknownFieldIsRefused(t *testing.T) {
	pool := migrated(t)
	bad := []byte(`{"city": {"slug": "x", "state": "open", "time_zone": "UTC", "currency": "VND", "names": {"en": "X"}, "areas": []}}`)
	if err := cities.LoadFixture(context.Background(), pool, bad); err == nil {
		t.Fatal("LoadFixture accepted an unknown field")
	}
}
