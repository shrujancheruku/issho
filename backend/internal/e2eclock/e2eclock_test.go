//go:build e2e

package e2eclock_test

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/fixtures"
	e2ev1 "github.com/shrujancheruku/issho/backend/gen/issho/e2e/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/e2e/v1/e2ev1connect"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/e2eclock"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
	"github.com/shrujancheruku/issho/backend/internal/staff"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

const lanterns = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0001" // the fixture plan that runs in the evening

// 09:00 UTC on Monday 5 January 2026 is 16:00 in Da Nang–Hoi An, the fixture
// plans' opening day.
var start = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// serve mounts the test-only services on the real handler, behind the real
// access interceptor, and returns clients for them.
func serve(t *testing.T, clk *clock.Fake, former e2eclock.Former) (e2ev1connect.ClockServiceClient, e2ev1connect.FormingServiceClient) {
	t.Helper()
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{Version: "e2e", Mounts: e2eclock.Mounts(clk, former)}))
	t.Cleanup(srv.Close)
	return e2ev1connect.NewClockServiceClient(srv.Client(), srv.URL), e2ev1connect.NewFormingServiceClient(srv.Client(), srv.URL)
}

// The API's clock and the jobs' are one clock, which a test moves through this
// service with no token (every RPC declares an access rule, and these are
// public in a build that holds no data).
func TestTheClockRPCsMoveTheClockTheAPIReads(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewFake(start)
	c, _ := serve(t, clk, nil)

	got, err := c.GetTime(ctx, connect.NewRequest(&e2ev1.GetTimeRequest{}))
	if err != nil || !got.Msg.GetNow().AsTime().Equal(start) {
		t.Fatalf("GetTime = %v, %v; want %v", got, err, start)
	}

	later := start.Add(72 * time.Hour)
	set, err := c.SetTime(ctx, connect.NewRequest(&e2ev1.SetTimeRequest{Now: timestamppb.New(later)}))
	if err != nil || !set.Msg.GetNow().AsTime().Equal(later) || !clk.Now().Equal(later) {
		t.Fatalf("SetTime = %v, %v; clock = %v, want %v", set, err, clk.Now(), later)
	}

	adv, err := c.AdvanceTime(ctx, connect.NewRequest(&e2ev1.AdvanceTimeRequest{By: durationpb.New(90 * time.Minute)}))
	want := later.Add(90 * time.Minute)
	if err != nil || !adv.Msg.GetNow().AsTime().Equal(want) || !clk.Now().Equal(want) {
		t.Fatalf("AdvanceTime = %v, %v; clock = %v, want %v", adv, err, clk.Now(), want)
	}

	// Back in time too: a test starts a new scene from its own instant.
	if _, err := c.SetTime(ctx, connect.NewRequest(&e2ev1.SetTimeRequest{Now: timestamppb.New(start)})); err != nil || !clk.Now().Equal(start) {
		t.Fatalf("SetTime back = %v; clock = %v, want %v", err, clk.Now(), start)
	}
}

func TestTheClockRefusesWhatItCannotMean(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewFake(start)
	c, _ := serve(t, clk, nil)

	_, err := c.SetTime(ctx, connect.NewRequest(&e2ev1.SetTimeRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("SetTime with no instant = %v, want invalid_argument", err)
	}
	_, err = c.AdvanceTime(ctx, connect.NewRequest(&e2ev1.AdvanceTimeRequest{By: durationpb.New(-time.Hour)}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("AdvanceTime backwards = %v, want invalid_argument", err)
	}
	if !clk.Now().Equal(start) {
		t.Fatalf("a refused call moved the clock to %v", clk.Now())
	}
}

// FormPlan inserts the job a schedule will insert and returns when the worker
// has run it: three people who ticked one evening each have an invited seat
// afterwards, which is what the end-to-end journey then reads in the app.
func TestFormPlanRunsTheFormingJobAndReturnsWhenItHasRun(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	if err := plans.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(start)
	cs := cities.New(pool)
	ppl := people.New(clk, pool, cs, people.Config{DefaultCity: "da-nang-hoi-an"})
	signups := plans.NewSignups(clk, pool, cs)
	svc := outings.New(clk, pool, signups, ppl)

	var ids []string
	for n := 1; n <= 3; n++ {
		id := member(t, ppl, n)
		if _, err := signups.Set(ctx, id, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}

	former := riverFormer(t, clk, pool, svc)
	_, f := serve(t, clk, former)
	if _, err := f.FormPlan(ctx, connect.NewRequest(&e2ev1.FormPlanRequest{PlanId: lanterns})); err != nil {
		t.Fatalf("FormPlan: %v", err)
	}

	for _, id := range ids {
		seats, err := svc.Mine(ctx, id)
		if err != nil || len(seats) != 1 {
			t.Fatalf("seats of %s = %v, %v; want one invited seat", id, seats, err)
		}
	}
}

// A request that names no plan is refused before any job is inserted.
func TestFormPlanRefusesARequestWithNoPlan(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(start)
	cs := cities.New(pool)
	ppl := people.New(clk, pool, cs, people.Config{DefaultCity: "da-nang-hoi-an"})
	former := riverFormer(t, clk, pool, outings.New(clk, pool, plans.NewSignups(clk, pool, cs), ppl))
	_, f := serve(t, clk, former)

	_, err := f.FormPlan(ctx, connect.NewRequest(&e2ev1.FormPlanRequest{PlanId: ""}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("FormPlan with no plan = %v, want invalid_argument", err)
	}
}

type failingFormer struct{}

func (failingFormer) Form(context.Context, string) error {
	return fmt.Errorf("river: insert: ERROR: relation \"river_job\" does not exist")
}

// A forming that fails answers with a fixed message, never the error's text,
// which may quote River or the database.
func TestFormPlanThatFailsSendsNoErrorText(t *testing.T) {
	_, f := serve(t, clock.NewFake(start), failingFormer{})

	_, err := f.FormPlan(context.Background(), connect.NewRequest(&e2ev1.FormPlanRequest{PlanId: lanterns}))

	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeInternal || ce.Message() != "could not form the plan" {
		t.Fatalf("FormPlan = %v, want internal with a fixed message", err)
	}
}

func member(t *testing.T, ppl *people.Service, n int) string {
	t.Helper()
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
	if _, err := ppl.Join(context.Background(), people.Claims{Subject: id, Email: fmt.Sprintf("p%d@example.com", n)}, people.Preferences{}); err != nil {
		t.Fatal(err)
	}
	if _, err := ppl.SaveProfile(context.Background(), id, people.ProfileInput{
		Name: fmt.Sprintf("Person %d", n), DateOfBirth: "1990-06-15", Gender: people.GenderMan, Languages: []string{"en"},
		LivesInCity: true, AcceptedCodeOfConduct: true,
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

// riverFormer is what cmd/api builds under the tag: a River client that works
// the forming job on the same pool, and the Former over it.
func riverFormer(t *testing.T, clk clock.Clock, pool *pgxpool.Pool, svc *outings.Service) e2eclock.Former {
	t.Helper()
	workers := jobs.NewWorkers(clk, pool, staff.NewKinds())
	outings.AddWorker(workers, svc)
	client, err := jobs.New(pool, workers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })
	return e2eclock.NewRiverFormer(client)
}
