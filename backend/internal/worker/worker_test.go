package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// jobKinds lists every job: its args, its worker (for Timeout) and what it
// must report.
var jobKinds = []struct {
	args     river.JobArgs
	timeout  time.Duration
	queue    string
	attempts int
}{
	{MatchAllArgs{}, MatchAllTimeout, QueuePasses, MatchAllMaxAttempts},
	{MatchCityArgs{City: "x"}, MatchCityTimeout, QueuePasses, MatchCityMaxAttempts},
	{CrewsCityArgs{City: "x"}, CrewsCityTimeout, QueuePasses, CrewsCityMaxAttempts},
	{SendNotificationsArgs{}, SendNotificationsTimeout, QueueNotify, SendNotificationsMaxAttempts},
	{RefreshMapsArgs{}, RefreshMapsTimeout, QueueMaintenance, RefreshMapsMaxAttempts},
	{RefreshMapsCityArgs{City: "x"}, RefreshMapsCityTimeout, QueueMaintenance, RefreshMapsCityMaxAttempts},
	{RetentionArgs{}, RetentionTimeout, QueueMaintenance, RetentionMaxAttempts},
}

// timeoutOf asks the registered worker for the job's time limit.
func timeoutOf(t *testing.T, a river.JobArgs) time.Duration {
	t.Helper()
	switch a.(type) {
	case MatchAllArgs:
		return (&matchAllWorker{}).Timeout(nil)
	case MatchCityArgs:
		return (&matchCityWorker{}).Timeout(nil)
	case CrewsCityArgs:
		return (&crewsCityWorker{}).Timeout(nil)
	case SendNotificationsArgs:
		return (&sendNotificationsWorker{}).Timeout(nil)
	case RefreshMapsArgs:
		return (&refreshMapsWorker{}).Timeout(nil)
	case RefreshMapsCityArgs:
		return (&refreshMapsCityWorker{}).Timeout(nil)
	case RetentionArgs:
		return (&retentionWorker{}).Timeout(nil)
	}
	t.Fatalf("no worker for %T", a)
	return 0
}

func TestEveryKindReportsQueueTimeoutAndAttempts(t *testing.T) {
	for _, k := range jobKinds {
		o := k.args.(river.JobArgsWithInsertOpts).InsertOpts()
		if o.Queue != k.queue || o.MaxAttempts != k.attempts {
			t.Errorf("%s: queue %q attempts %d, want %q %d", k.args.Kind(), o.Queue, o.MaxAttempts, k.queue, k.attempts)
		}
		if got := timeoutOf(t, k.args); got != k.timeout || got <= 0 {
			t.Errorf("%s: timeout %v, want %v", k.args.Kind(), got, k.timeout)
		}
		if _, ok := queues()[o.Queue]; !ok {
			t.Errorf("%s: queue %q is not served", k.args.Kind(), o.Queue)
		}
	}
	// Old rows from before the queues stay served.
	if q, ok := queues()[river.QueueDefault]; !ok || q.MaxWorkers < 1 {
		t.Error("the default queue is no longer served: jobs enqueued by the previous release would strand")
	}
	if MatchCityTimeout < 2*walks.MinBudget {
		t.Error("MatchCityTimeout leaves little room for walks")
	}
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testfixture.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func cleanJobs(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	del := func() {
		if _, err := pool.Exec(context.Background(), `delete from river.river_job where kind like 'test\_%' or kind in ('match_all','match_city','crews_city','send_notifications','refresh_maps','refresh_maps_city','retention')`); err != nil {
			t.Errorf("clean jobs: %v", err)
		}
	}
	del()
	t.Cleanup(del)
}

// Every kind is registered (an insert of an unregistered kind is refused),
// and inserting any twice in its window leaves one job.
func TestIntegrationEveryKindIsRegisteredAndUnique(t *testing.T) {
	pool := testPool(t)
	cleanJobs(t, pool)
	ctx := context.Background()
	c, err := NewClient(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), plans.DefaultTiming(), nil, Maps{})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range jobKinds {
		first, err := c.Insert(ctx, k.args, nil)
		if err != nil {
			t.Fatalf("%s: insert: %v", k.args.Kind(), err)
		}
		second, err := c.Insert(ctx, k.args, nil)
		if err != nil {
			t.Fatalf("%s: second insert: %v", k.args.Kind(), err)
		}
		if first.UniqueSkippedAsDuplicate || !second.UniqueSkippedAsDuplicate || first.Job.ID != second.Job.ID {
			t.Errorf("%s: inserted twice gave jobs %d and %d (skipped: %v, %v)", k.args.Kind(),
				first.Job.ID, second.Job.ID, first.UniqueSkippedAsDuplicate, second.UniqueSkippedAsDuplicate)
		}
		if first.Job.Queue != k.queue || first.Job.MaxAttempts != k.attempts {
			t.Errorf("%s: stored in %q with %d attempts", k.args.Kind(), first.Job.Queue, first.Job.MaxAttempts)
		}
	}
	// Different cities are different jobs.
	if r, err := c.Insert(ctx, MatchCityArgs{City: "y"}, nil); err != nil || r.UniqueSkippedAsDuplicate {
		t.Errorf("another city's pass was skipped: %v", err)
	}
}

type blockArgs struct{ Queue string }

func (blockArgs) Kind() string { return "test_block" }

type blockWorker struct {
	river.WorkerDefaults[blockArgs]
	started chan struct{}
	release chan struct{}
}

func (w *blockWorker) Work(ctx context.Context, _ *river.Job[blockArgs]) error {
	w.started <- struct{}{}
	select {
	case <-w.release:
	case <-ctx.Done():
	}
	return nil
}

type pingArgs struct{}

func (pingArgs) Kind() string { return "test_ping" }

type pingWorker struct {
	river.WorkerDefaults[pingArgs]
	done chan struct{}
}

func (w *pingWorker) Work(context.Context, *river.Job[pingArgs]) error { close(w.done); return nil }

// With both `passes` workers busy (and a third pass waiting), a push job
// still runs: the queues are the client's real ones.
func TestIntegrationNotifyRunsWhilePassesAreBusy(t *testing.T) {
	pool := testPool(t)
	cleanJobs(t, pool)
	ctx := context.Background()
	bw := &blockWorker{started: make(chan struct{}, 3), release: make(chan struct{})}
	pw := &pingWorker{done: make(chan struct{})}
	workers := river.NewWorkers()
	river.AddWorker(workers, bw)
	river.AddWorker(workers, pw)
	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: db.RiverSchema, Queues: queues(), Workers: workers})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(bw.release)
		stop, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_ = c.Stop(stop)
	})
	for range 3 {
		if _, err := c.Insert(ctx, blockArgs{}, &river.InsertOpts{Queue: QueuePasses}); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		select {
		case <-bw.started:
		case <-time.After(15 * time.Second):
			t.Fatal("passes jobs did not start")
		}
	}
	if _, err := c.Insert(ctx, pingArgs{}, &river.InsertOpts{Queue: QueueNotify}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pw.done:
	case <-time.After(15 * time.Second):
		t.Fatal("the notify job waited behind the passes")
	}
	select {
	case <-bw.started:
		t.Error("a third pass started: passes has more than 2 workers")
	default:
	}
}

// refresh_maps queues one refresh_maps_city per city.
func TestIntegrationRefreshMapsQueuesOneJobPerCity(t *testing.T) {
	pool := testPool(t)
	cleanJobs(t, pool)
	ctx := context.Background()
	q := dbgen.New(pool)
	for _, id := range []string{"zz-worker-a", "zz-worker-b"} {
		cleanup := func() { _, _ = pool.Exec(ctx, "delete from app.cities where id = $1", id) }
		cleanup()
		t.Cleanup(cleanup)
		if err := q.UpsertCity(ctx, dbgen.UpsertCityParams{
			ID: id, Name: []byte(`{"en":"T"}`), Timezone: "UTC", Currency: "USD",
			Languages: []string{"en"}, TravelModes: []string{"walk"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	cities, err := q.ListCities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, c := range cities {
		want = append(want, c.ID)
	}
	w := &refreshMapsWorker{pool: pool, router: fakeRouter{}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rw := rivertest.NewWorker(t, riverpgxv5.New(pool), &river.Config{Schema: db.RiverSchema}, w)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Work(ctx, t, tx, RefreshMapsArgs{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `select args->>'city' from river.river_job where kind = 'refresh_maps_city'`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("queued for %v, want one per city %v", got, want)
	}
}

// A city failing is an error for its own job only.
func TestRefreshMapsCityFailureIsItsOwn(t *testing.T) {
	boom := errors.New("maps down")
	w := &refreshMapsCityWorker{log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		refresh: func(_ context.Context, city string) (bool, int64, error) {
			if city == "bad" {
				return false, 0, boom
			}
			return true, 3, nil
		}}
	ctx := context.Background()
	if err := w.Work(ctx, &river.Job[RefreshMapsCityArgs]{Args: RefreshMapsCityArgs{City: "bad"}}); !errors.Is(err, boom) {
		t.Errorf("bad city: %v", err)
	}
	if err := w.Work(ctx, &river.Job[RefreshMapsCityArgs]{Args: RefreshMapsCityArgs{City: "good"}}); err != nil {
		t.Errorf("good city: %v", err)
	}
}

type fakeRouter struct{ maps.Router }
