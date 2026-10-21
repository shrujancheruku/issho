package profile

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestIntegrationSave runs Save and Get against a real, migrated and seeded
// database (the Tokyo pack). It's skipped unless TEST_DATABASE_URL is set (CI
// sets it). It creates its own throwaway user and deletes it afterwards.
func TestIntegrationSave(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	svc := &Service{DB: pool, Now: func() time.Time { return time.Date(year, 6, 1, 0, 0, 0, 0, time.UTC) }}
	id := uuid.New()

	if _, err := svc.Save(ctx, id, valid()); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("save without an account: %v", err)
	}

	// On Supabase, app.users references auth.users; plain Postgres (CI) has
	// no auth schema.
	var supabase bool
	if err := pool.QueryRow(ctx, "select exists (select 1 from pg_namespace where nspname = 'auth')").Scan(&supabase); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		stmts := []string{"delete from app.users where id = $1"}
		if supabase {
			stmts = append(stmts, "delete from auth.users where id = $1")
		}
		for _, q := range stmts {
			if _, err := pool.Exec(ctx, q, id); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}
	t.Cleanup(cleanup)
	if supabase {
		if _, err := pool.Exec(ctx, "insert into auth.users (id, email) values ($1, 'profile-test@example.com')", id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, "insert into app.users (id) values ($1)", id); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("before onboarding: %v", err)
	}

	got, err := svc.Save(ctx, id, valid())
	if err != nil {
		t.Fatal(err)
	}
	if got.CityID != "tokyo" || got.DisplayName != "Aiko" || len(got.Languages) != 2 || len(got.Availability) != 2 || len(got.Prefs) != 3 {
		t.Fatalf("saved %+v", got)
	}
	if got.Languages[1] != (Language{Code: "es", Level: 1}) || got.Prefs["photography"] != 2 {
		t.Errorf("children: %+v %+v", got.Languages, got.Prefs)
	}

	// Saving again replaces every list rather than adding to it.
	p := valid()
	p.Languages = []Language{{Code: "en", Fluent: true}}
	p.Availability = []Slot{{3, "evening"}}
	p.Prefs = map[string]int{"bowling": 1, "board-games": 1, "food-tour": 2}
	p.SecondaryLanguage = "en"
	got, err = svc.Save(ctx, id, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Languages) != 1 || got.Availability[0] != (Slot{3, "evening"}) || len(got.Prefs) != 3 || got.Prefs["photography"] != 0 || got.SecondaryLanguage != "en" {
		t.Errorf("replace: %+v", got)
	}
	if again, err := svc.Get(ctx, id); err != nil || again.SecondaryLanguage != "en" {
		t.Errorf("get: %+v, %v", again, err)
	}

	unknownArea := valid()
	unknownArea.HomeAreaID = "tokyo-atlantis"
	if _, err := svc.Save(ctx, id, unknownArea); !IsInvalid(err) {
		t.Errorf("unknown area: %v", err)
	}
	unknownActivity := valid()
	unknownActivity.Prefs["time-travel"] = 2
	if _, err := svc.Save(ctx, id, unknownActivity); !IsInvalid(err) {
		t.Errorf("unknown activity: %v", err)
	}
	// A failed save leaves the previous profile untouched.
	if kept, _ := svc.Get(ctx, id); len(kept.Languages) != 1 {
		t.Errorf("failed save changed the profile: %+v", kept)
	}
}
