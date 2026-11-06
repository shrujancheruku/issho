package worker

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/db"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/adminjobs"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/mapstest"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// gateRouter holds the travel-time rebuild open, so a test can look at the
// job while it runs.
type gateRouter struct {
	mapstest.Fake
	started chan struct{}
	release chan struct{}
}

func (g *gateRouter) Matrix(ctx context.Context, r maps.MatrixRequest) ([]maps.MatrixElement, error) {
	select {
	case g.started <- struct{}{}:
	default:
	}
	select {
	case <-g.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return g.Fake.Matrix(ctx, r)
}

// The admin jobs through the real queue: inserted by the API's insert-only
// client, run by a worker client, read back by GetAdminJob. A second apply
// of the same city while one is queued or running is the same job; the state
// goes queued -> running -> succeeded with the apply report as its result; a
// failure carries its reason.
func TestIntegrationAdminJobsLifecycle(t *testing.T) {
	pool := testPool(t)
	cleanJobs(t, pool)
	ctx := context.Background()
	const city = "zz-admin-jobs"
	cleanCity := func() { _, _ = pool.Exec(ctx, "delete from app.cities where id = $1", city) }
	cleanCity()
	t.Cleanup(cleanCity)

	pack := func(areas ...string) cities.Pack {
		p := cities.Pack{
			ID: city, Name: cities.Text{"en": "Admin jobs"}, Timezone: "Asia/Tokyo", Currency: "JPY",
			Languages: []string{"en"}, TravelModes: []string{"walk"},
		}
		for i, a := range areas {
			p.Areas = append(p.Areas, cities.Area{ID: city + "-" + a, Name: cities.Text{"en": a}, Lat: 35.70 + float64(i)/100, Lng: 139.6})
		}
		return p
	}

	// The API's side: an insert-only client and the handler.
	api, err := adminjobs.NewClient(pool)
	if err != nil {
		t.Fatal(err)
	}
	h := &adminjobs.Handler{Jobs: api}
	adminCtx := auth.WithCaller(ctx, auth.Caller{UserID: uuid.New(), Email: "admin@example.com", Admin: true})
	get := func(id int64) *isshov1.GetAdminJobResponse {
		t.Helper()
		res, err := h.GetAdminJob(adminCtx, &isshov1.GetAdminJobRequest{JobId: id})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	waitFor := func(id int64, want isshov1.AdminJobState) *isshov1.GetAdminJobResponse {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for {
			if j := get(id); j.State == want {
				return j
			} else if time.Now().After(deadline) {
				t.Fatalf("job %d is %v, want %v", id, j.State, want)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	ins := func(p cities.Pack) *rivertype.JobInsertResult {
		t.Helper()
		args, err := adminjobs.NewApplyCityPackArgs(p)
		if err != nil {
			t.Fatal(err)
		}
		res, err := api.Insert(ctx, args, nil)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	first := ins(pack("a", "b"))
	if first.UniqueSkippedAsDuplicate {
		t.Fatal("first apply was skipped")
	}
	// Queued: nothing is serving the queue yet.
	if j := get(first.Job.ID); j.State != isshov1.AdminJobState_ADMIN_JOB_STATE_QUEUED || j.QueuedAt == nil || j.StartedAt != nil || j.FinishedAt != nil || j.Result != nil || j.Failure != nil {
		t.Fatalf("queued job = %v", j)
	}
	// The same pack again is skipped and gets the first job's id. A changed
	// pack (main moved on) is a job of its own, queued behind the first: it
	// must not be dropped. They never run at once: the queue has one worker.
	if same := ins(pack("a", "b")); !same.UniqueSkippedAsDuplicate || same.Job.ID != first.Job.ID {
		t.Fatalf("the same pack twice = %+v", same)
	}
	changed := ins(pack("a", "b", "c"))
	if changed.UniqueSkippedAsDuplicate || changed.Job.ID == first.Job.ID {
		t.Fatalf("a changed pack = %+v, want a second job", changed)
	}
	// Another city is another job.
	other, err := api.Insert(ctx, adminjobs.ApplyCityPackArgs{City: "zz-admin-other", Hash: "x", Pack: cities.Pack{ID: "zz-admin-other"}}, nil)
	if err != nil || other.UniqueSkippedAsDuplicate {
		t.Fatalf("another city's apply was skipped: %v", err)
	}
	if _, err := api.JobCancel(ctx, other.Job.ID); err != nil {
		t.Fatal(err)
	}

	// (A pack needs two areas for there to be travel times to rebuild.)
	// The worker's side: the real workers on the maintenance queue, with a
	// router held open and no places key.
	gate := &gateRouter{started: make(chan struct{}, 1), release: make(chan struct{})}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	workers := river.NewWorkers()
	river.AddWorker(workers, &applyCityPackWorker{pool: pool, router: gate, log: log})
	river.AddWorker(workers, &discoverAreaVenuesWorker{pool: pool, places: nil, log: log})
	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: db.RiverSchema, Workers: workers,
		Queues: map[string]river.QueueConfig{QueueMaintenance: {MaxWorkers: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_ = c.Stop(stop)
	})

	select {
	case <-gate.started:
	case <-time.After(20 * time.Second):
		t.Fatal("the apply did not start")
	}
	running := get(first.Job.ID)
	if running.State != isshov1.AdminJobState_ADMIN_JOB_STATE_RUNNING || running.StartedAt == nil || running.FinishedAt != nil || running.Result != nil {
		t.Fatalf("running job = %v", running)
	}
	// Still one job while it runs.
	if dup := ins(pack("a", "b")); !dup.UniqueSkippedAsDuplicate || dup.Job.ID != first.Job.ID {
		t.Errorf("an apply during the apply: %+v", dup)
	}
	close(gate.release)
	done := waitFor(first.Job.ID, isshov1.AdminJobState_ADMIN_JOB_STATE_SUCCEEDED)
	rep := done.GetApplyCityPack()
	if rep == nil || len(rep.NewAreas) != 2 || rep.NewAreas[0] != city+"-a" || !rep.TravelTimesRebuilt || done.FinishedAt == nil || done.Failure != nil {
		t.Fatalf("succeeded job = %v", done)
	}
	// The changed pack runs after the first has finished, and adds area c.
	ch := waitFor(changed.Job.ID, isshov1.AdminJobState_ADMIN_JOB_STATE_SUCCEEDED)
	if got := ch.GetApplyCityPack(); got == nil || len(got.NewAreas) != 1 || got.NewAreas[0] != city+"-c" || ch.StartedAt.AsTime().Before(done.FinishedAt.AsTime()) {
		t.Fatalf("changed pack's job = %v (first finished %v)", ch, done.FinishedAt)
	}
	// Applied once the others have finished, a new job is accepted.
	third := ins(pack("a", "b", "c"))
	if third.UniqueSkippedAsDuplicate || third.Job.ID == first.Job.ID {
		t.Fatalf("apply after the others finished = %+v", third)
	}
	if got := waitFor(third.Job.ID, isshov1.AdminJobState_ADMIN_JOB_STATE_SUCCEEDED).GetApplyCityPack(); got == nil || len(got.NewAreas) != 0 || got.TravelTimesRebuilt {
		t.Errorf("a repeat apply reports %v, want nothing new", got)
	}

	// A failed job carries its reason: this worker has no places key.
	failed, err := api.Insert(ctx, adminjobs.DiscoverAreaVenuesArgs{City: city, Area: city + "-a"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := waitFor(failed.Job.ID, isshov1.AdminJobState_ADMIN_JOB_STATE_FAILED)
	if f.Failure == nil || f.Failure.Reason != isshov1.ErrorReason_ERROR_REASON_FEATURE_UNAVAILABLE || f.Failure.Message == "" || f.FinishedAt == nil || f.Result != nil {
		t.Errorf("failed job = %v", f)
	}

	// A job that isn't an admin operation is not for this screen.
	notAdmin, err := api.Insert(ctx, RetentionArgs{}, &river.InsertOpts{Pending: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.GetAdminJob(adminCtx, &isshov1.GetAdminJobRequest{JobId: notAdmin.Job.ID}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a retention job: %v", err)
	}
}
