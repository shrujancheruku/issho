package outings_test

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"

	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/outings"
)

// work runs one forming job the way the worker process would, with River's test
// harness: the job's own transaction is the harness's, and forming opens its
// own, so a job redelivered after it committed starts from what is stored.
func (r *rig) work(t *testing.T, planID string) error {
	t.Helper()
	ctx := context.Background()
	w := rivertest.NewWorker(t, riverpgxv5.New(r.pool), &river.Config{Schema: jobs.Schema}, outings.NewFormWorker(r.svc))
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = w.Work(ctx, t, tx, outings.FormArgs{PlanID: planID}, nil)
	return err
}

// A job forms groups for a plan: it makes the outings and their invitations,
// and River delivering it twice (it runs jobs at least once) makes no second.
func TestTheFormingJobFormsTheGroupsOfAPlanOnce(t *testing.T) {
	r := newRig(t)
	ids := r.crowd(t, 1, 4, person{})

	if err := r.work(t, lanterns); err != nil {
		t.Fatal(err)
	}
	if err := r.work(t, lanterns); err != nil {
		t.Fatal(err)
	}

	if seats := r.seatsOf(t, ids...); len(seats) != 4 || distinctOutings(seats) != 1 {
		t.Fatalf("seats = %+v, want 4 seats in one outing", seats)
	}
}

// A plan that isn't there (or isn't shown) has nothing to form, and retrying
// the job would change nothing, so the job finishes without an error.
func TestTheFormingJobForAPlanThatIsNotThereFinishes(t *testing.T) {
	r := newRig(t)
	if err := r.work(t, "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff"); err != nil {
		t.Fatalf("job = %v, want it to finish without an error", err)
	}
}
