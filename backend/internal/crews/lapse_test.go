package crews

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationUnansweredLapse: while an offer is open, people who haven't
// answered are listed as deciding; when it's decided at its deadline, they
// lapse and the crew no longer lists them.
func TestIntegrationUnansweredLapse(t *testing.T) {
	f := testfixture.New(t, "zz-crews-lapse", "UTC", 4)
	ctx := context.Background()
	for _, u := range f.Users {
		f.AddProfile(t, u, "ja")
	}
	a, b, c, d := f.Users[0], f.Users[1], f.Users[2], f.Users[3]
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
		return id
	}
	crewID := offer(a, b, c, d)
	tooFew := offer(a, b, c)
	for _, u := range []uuid.UUID{a, b, c} {
		if err := svc.Respond(ctx, u, crewID, true, day(2)); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Respond(ctx, a, tooFew, true, day(2)); err != nil {
		t.Fatal(err)
	}
	it, err := svc.Get(ctx, a, crewID)
	if err != nil || it.Status != Offered || len(it.Members) != 4 {
		t.Fatalf("open offer = %+v, %v; want four listed, d among those deciding", it, err)
	}

	rep, err := r.Pass(ctx, f.City, day(21))
	if err != nil || rep.Formed != 1 || rep.Lapsed != 1 {
		t.Fatalf("pass at the deadline = %+v, %v; want one formed, one lapsed", rep, err)
	}
	it, err = svc.Get(ctx, a, crewID)
	if err != nil || it.Status != Active || len(it.Members) != 3 {
		t.Fatalf("active crew = %+v, %v; want three members", it, err)
	}
	for _, m := range it.Members {
		if m.ID == d || !m.Accepted {
			t.Errorf("listed %+v: d never answered and isn't a member", m)
		}
	}
	var response string
	var respondedAt *time.Time
	if err := f.Pool.QueryRow(ctx, `select response, responded_at from app.crew_members where crew_id = $1 and user_id = $2`, crewID, d).Scan(&response, &respondedAt); err != nil {
		t.Fatal(err)
	}
	if response != Unanswered || respondedAt == nil || !respondedAt.Equal(day(21)) {
		t.Errorf("d = %s at %v; want lapsed when decided", response, respondedAt)
	}
	if _, err := svc.Get(ctx, d, crewID); !errors.Is(err, ErrNoCrew) {
		t.Errorf("d sees the crew: %v", err)
	}
	// An offer that too few accepted lapses its silent members too.
	var waiting int
	if err := f.Pool.QueryRow(ctx, `select count(*) from app.crew_members where crew_id = $1 and response = 'offered'`, tooFew).Scan(&waiting); err != nil || waiting != 0 {
		t.Errorf("members still deciding in the lapsed crew = %d, %v", waiting, err)
	}
}
