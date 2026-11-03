package crews

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// failWhen makes the database reject an update on a table when cond (SQL
// over NEW) holds, until the test ends: the way to make one crew fail
// inside its transaction.
func failWhen(t *testing.T, f *testfixture.Fixture, name, table, cond string) {
	t.Helper()
	ctx := context.Background()
	fn := "app.zz_crews_" + name
	for _, s := range []string{
		fmt.Sprintf(`create or replace function %s() returns trigger language plpgsql as $$ begin raise exception 'injected failure'; end $$`, fn),
		fmt.Sprintf(`create trigger zz_crews_%s before update on %s for each row when (%s) execute function %s()`, name, table, cond, fn),
	} {
		if _, err := f.Pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, s := range []string{
			fmt.Sprintf(`drop trigger if exists zz_crews_%s on %s`, name, table),
			fmt.Sprintf(`drop function if exists %s()`, fn),
		} {
			if _, err := f.Pool.Exec(ctx, s); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})
}

// TestIntegrationPassSkipsFailedCrew: with two offers ready to be decided,
// one failing is rolled back and not counted, the other forms, and the pass
// succeeds. Every crew of the stage failing is an error.
func TestIntegrationPassSkipsFailedCrew(t *testing.T) {
	f := testfixture.New(t, "zz-crews-skipfail", "UTC", 8)
	ctx := context.Background()
	for _, u := range f.Users {
		f.AddProfile(t, u, "ja")
	}
	svc := &Service{DB: f.Pool}
	r := &Runner{DB: f.Pool, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	offer := func(members ...uuid.UUID) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := f.Pool.QueryRow(ctx, `insert into app.crews (city_id, activity_id, offer_until) values ($1, $2, $3) returning id`,
			f.City, f.Act, day(20)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Pool.Exec(ctx, `insert into app.crew_members (crew_id, user_id) select $1, unnest($2::uuid[])`, id, members); err != nil {
			t.Fatal(err)
		}
		// The last person stays silent, so the offer is decided at its deadline
		// rather than when the third accepts.
		for _, u := range members[:len(members)-1] {
			if err := svc.Respond(ctx, u, id, true, day(2)); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	bad, good := offer(f.Users[0], f.Users[1], f.Users[2], f.Users[3]), offer(f.Users[4], f.Users[5], f.Users[6], f.Users[7])
	status := func(id uuid.UUID) string {
		var s string
		if err := f.Pool.QueryRow(ctx, "select status from app.crews where id = $1", id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// Forming the first crew fails after the decision would have been counted.
	failWhen(t, f, "crew", "app.crews", fmt.Sprintf("new.id = '%s' and new.status = 'active'", bad))
	rep, err := r.Pass(ctx, f.City, day(21))
	if err != nil {
		t.Fatalf("Pass = %+v, %v; one crew failing is still success", rep, err)
	}
	if rep.Formed != 1 || rep.Failed != 1 || rep.Lapsed != 0 {
		t.Errorf("report = %+v, want 1 formed (the rolled-back crew isn't counted) and 1 failed", rep)
	}
	if got := status(bad); got != Offered {
		t.Errorf("failed crew is %s, want it left offered by the rollback", got)
	}
	if got := status(good); got != Active {
		t.Errorf("other crew is %s, want active", got)
	}

	// Only the failing crew is left to decide, and it fails again.
	rep, err = r.Pass(ctx, f.City, day(21))
	if err == nil || !strings.Contains(err.Error(), "every crew failed") || rep.Failed != 1 || rep.Formed != 0 {
		t.Errorf("Pass with only a failing crew = %+v, %v; want an error and no counts", rep, err)
	}
}

// TestIntegrationPassSetupFailure: with the database down the pass fails,
// so River retries the job.
func TestIntegrationPassSetupFailure(t *testing.T) {
	f := testfixture.New(t, "zz-crews-setupfail", "UTC", 1)
	down, err := pgxpool.New(context.Background(), os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	down.Close()
	r := &Runner{DB: down, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if _, err := r.Pass(context.Background(), f.City, day(21)); err == nil {
		t.Error("Pass with the database down: no error")
	}
}
