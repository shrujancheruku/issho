package moderation

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/adminjobs"
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// failingInserter is a job queue that always refuses.
type failingInserter struct{}

func (failingInserter) InsertTx(context.Context, pgx.Tx, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	return nil, errors.New("queue down")
}

func day(d, h int) time.Time { return time.Date(2030, 1, d, h, 0, 0, 0, time.UTC) }

// modFixture is a city with a person to suspend (users[0]), another and an
// admin, and the River client the API uses.
type modFixture struct {
	*testfixture.Fixture
	svc         *Service
	suspensions *Suspensions
	remover     *Remover
}

func newModFixture(t *testing.T, id string) *modFixture {
	t.Helper()
	f := &modFixture{Fixture: testfixture.New(t, id, "UTC", 3)}
	for _, u := range f.Users {
		f.AddProfile(t, u, "en")
	}
	jobs, err := adminjobs.NewClient(f.Pool)
	if err != nil {
		t.Fatal(err)
	}
	f.suspensions = &Suspensions{Q: f.Q}
	f.svc = &Service{DB: f.Pool, Jobs: jobs, Suspensions: f.suspensions}
	f.remover = &Remover{Q: f.Q, Plans: &plans.Service{DB: f.Pool}, Crews: &crews.Service{DB: f.Pool}}
	del := func() {
		if _, err := f.Pool.Exec(context.Background(), `delete from river.river_job where kind = $1 and args->>'user_id' = any($2)`, KindRemoveSuspended, uuidStrings(f.Users)); err != nil {
			t.Errorf("clean jobs: %v", err)
		}
	}
	del()
	t.Cleanup(del)
	return f
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func (f *modFixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *modFixture) queued(t *testing.T, user uuid.UUID) int {
	return f.count(t, `select count(*) from river.river_job where kind = $1 and args->>'user_id' = $2`, KindRemoveSuspended, user.String())
}

// join puts the person in a signup, a confirmed outing and a crew offer, and
// returns a function counting what's left of each.
func (f *modFixture) join(t *testing.T, user, other uuid.UUID) (left func() (signups, outings, crewPlaces int)) {
	t.Helper()
	ctx := context.Background()
	newPlan := func() uuid.UUID {
		plan := matching.Plan{Area: matching.AreaID(f.Area), Activity: matching.ActivityID(f.Act), Primary: "en"}
		id, err := plans.Create(ctx, f.Q, f.City, matching.Suggestion{Plan: plan, Audience: 5, Score: 1}, plans.Window{Start: day(7, 0), Days: 14, Deadline: day(14, 0)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	ids := []matching.UserID{matching.UserID(user.String()), matching.UserID(other.String())}
	eventID, _, err := plans.SaveEvent(ctx, f.Pool, plans.Event{
		PlanID: newPlan(), Round: 1, StartsAt: day(12, 14), ConfirmBy: day(11, 0), Now: day(5, 0),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12, 0), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: ids, Quorum: 2, MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (&plans.Service{DB: f.Pool}).Respond(ctx, user, eventID, plans.Confirm, day(8, 0)); err != nil {
		t.Fatal(err)
	}
	if err := plans.SaveSignup(ctx, f.Pool, newPlan(), matching.Signup{User: matching.UserID(user.String())}); err != nil {
		t.Fatal(err)
	}
	var crew uuid.UUID
	if err := f.Pool.QueryRow(ctx, `insert into app.crews (city_id, activity_id, offer_until) values ($1, $2, $3) returning id`, f.City, f.Act, day(20, 0)).Scan(&crew); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Pool.Exec(ctx, `insert into app.crew_members (crew_id, user_id) select $1, unnest($2::uuid[])`, crew, []uuid.UUID{user, other}); err != nil {
		t.Fatal(err)
	}
	return func() (int, int, int) {
		return f.count(t, `select count(*) from app.plan_signups where user_id = $1`, user),
			f.count(t, `select count(*) from app.group_members where user_id = $1 and event_id = $2 and response = 'confirmed'`, user, eventID),
			f.count(t, `select count(*) from app.crew_members where user_id = $1 and crew_id = $2 and response = 'offered'`, user, crew)
	}
}

// If the cleanup can't be queued, the suspension doesn't commit: nobody is
// left suspended without the job that removes them.
func TestIntegrationSuspendRollsBackWhenTheJobCannotBeQueued(t *testing.T) {
	f := newModFixture(t, "zz-mod-rollback")
	ctx := context.Background()
	svc := &Service{DB: f.Pool, Jobs: failingInserter{}, Suspensions: f.suspensions}
	user := f.Users[0]
	if err := svc.Suspend(ctx, f.Users[2], user, "reason", day(8, 0)); err == nil {
		t.Fatal("Suspend succeeded with the queue down")
	}
	if n := f.count(t, `select count(*) from app.suspensions where user_id = $1`, user); n != 0 {
		t.Errorf("%d suspensions committed", n)
	}
	if got, err := f.Q.IsSuspended(ctx, user); err != nil || got {
		t.Errorf("IsSuspended = %v, %v", got, err)
	}
}

// Suspend commits the suspension and queues one job; suspending again queues
// no second; the job removes the person from everything, and again changes
// nothing; the API's check is told.
func TestIntegrationSuspendQueuesCleanupWhichIsRepeatable(t *testing.T) {
	f := newModFixture(t, "zz-mod-cleanup")
	ctx := context.Background()
	user, other, admin := f.Users[0], f.Users[1], f.Users[2]
	left := f.join(t, user, other)
	if s, o, c := left(); s != 1 || o != 1 || c != 1 {
		t.Fatalf("before: signups %d, outings %d, crew places %d", s, o, c)
	}
	if got, _ := f.suspensions.Suspended(ctx, user); got { // also loads the cache
		t.Fatal("suspended before the call")
	}
	for range 2 {
		if err := f.svc.Suspend(ctx, admin, user, "reason", day(8, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := f.suspensions.Suspended(ctx, user); err != nil || !got {
		t.Errorf("the API's check = %v, %v", got, err)
	}
	if n := f.queued(t, user); n != 1 {
		t.Fatalf("%d jobs queued, want 1", n)
	}

	args := RemoveSuspendedArgs{UserID: user, At: day(8, 0)}
	for run := 1; run <= 2; run++ {
		if err := f.remover.Run(ctx, args); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if s, o, c := left(); s != 0 || o != 0 || c != 0 {
			t.Errorf("after run %d: signups %d, outings %d, crew places %d", run, s, o, c)
		}
	}
	if got := f.count(t, `select count(*) from app.group_members where user_id = $1 and response = 'dropped'`, user); got != 1 {
		t.Errorf("dropped places = %d, want 1", got)
	}
}

// A job for a suspension lifted before it runs does nothing.
func TestIntegrationRemovalSkippedWhenLifted(t *testing.T) {
	f := newModFixture(t, "zz-mod-lifted")
	ctx := context.Background()
	user, other, admin := f.Users[0], f.Users[1], f.Users[2]
	left := f.join(t, user, other)
	if err := f.svc.Suspend(ctx, admin, user, "reason", day(8, 0)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Lift(ctx, admin, user, "appeal", day(8, 1)); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.suspensions.Suspended(ctx, user); got {
		t.Error("the API's check still has them suspended after lifting")
	}
	if err := f.remover.Run(ctx, RemoveSuspendedArgs{UserID: user, At: day(8, 0)}); err != nil {
		t.Fatal(err)
	}
	if s, o, c := left(); s != 1 || o != 1 || c != 1 {
		t.Errorf("after a stale job: signups %d, outings %d, crew places %d; want 1 1 1", s, o, c)
	}
	if err := f.svc.Lift(ctx, admin, user, "again", day(8, 2)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("lifting twice: %v", err)
	}
}

// The handler's checks come before the service: yourself, no reason, and
// someone who doesn't exist.
func TestIntegrationSuspendPersonValidation(t *testing.T) {
	f := newModFixture(t, "zz-mod-validate")
	ctx := auth.WithCaller(context.Background(), auth.Caller{UserID: f.Users[2], Email: "admin@example.com", Admin: true})
	h := &Handler{Q: dbgen.New(f.Pool), Svc: f.svc}
	for _, c := range []struct {
		name string
		req  *isshov1.SuspendPersonRequest
		want connect.Code
	}{
		{"yourself", &isshov1.SuspendPersonRequest{UserId: f.Users[2].String(), Reason: "x"}, connect.CodeInvalidArgument},
		{"empty reason", &isshov1.SuspendPersonRequest{UserId: f.Users[0].String(), Reason: "  "}, connect.CodeInvalidArgument},
		{"not an id", &isshov1.SuspendPersonRequest{UserId: "nope", Reason: "x"}, connect.CodeInvalidArgument},
		{"unknown person", &isshov1.SuspendPersonRequest{UserId: uuid.NewString(), Reason: "x"}, connect.CodeNotFound},
	} {
		if _, err := h.SuspendPerson(ctx, connect.NewRequest(c.req)); connect.CodeOf(err) != c.want {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	if n := f.count(t, `select count(*) from app.suspensions where user_id = any($1)`, f.Users); n != 0 {
		t.Errorf("%d suspensions made by refused calls", n)
	}
}
