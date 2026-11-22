package outings

import (
	"context"
	"errors"

	"github.com/riverqueue/river"

	"github.com/shrujancheruku/issho/backend/internal/plans"
)

// FormArgs is the forming job's arguments: the plan to form groups for. It
// holds a plan's id and nothing about a person, so River's tables and the job
// log keep no personal data from it (internal/retention/README.md).
type FormArgs struct {
	PlanID string `json:"plan_id"`
}

func (FormArgs) Kind() string { return "outings_form" }

// formWorker is the job that forms groups for a plan: it calls Service.Form,
// which does the whole of it in one transaction.
type formWorker struct {
	river.WorkerDefaults[FormArgs]
	svc *Service
}

// NewFormWorker returns the worker for FormArgs.
func NewFormWorker(svc *Service) river.Worker[FormArgs] { return &formWorker{svc: svc} }

// AddWorker registers the forming and the expiring workers on workers (cmd/worker calls it, as
// jobs.NewWorkers' comment says each domain package does).
func AddWorker(workers *river.Workers, svc *Service) {
	river.AddWorker(workers, NewFormWorker(svc))
	river.AddWorker(workers, NewExpireWorker(svc))
}

func (w *formWorker) Work(ctx context.Context, job *river.Job[FormArgs]) error {
	_, err := w.svc.Form(ctx, job.Args.PlanID)
	// A plan that isn't there or isn't shown has nothing to form, and retrying
	// can't change that: finish rather than fail.
	if errors.Is(err, plans.ErrNotFound) {
		return nil
	}
	return err
}

// ExpireArgs is the job that records an outing's unanswered invitations: the
// outing, scheduled for the end of its 48 hours when the invitations are made.
// It holds an outing's id and nothing about a person, as FormArgs holds a plan's.
type ExpireArgs struct {
	OutingID string `json:"outing_id"`
}

func (ExpireArgs) Kind() string { return "outings_expire_invitations" }

// expireWorker calls Service.Expire, which closes only what is due and so is
// safe to run twice, as River may.
type expireWorker struct {
	river.WorkerDefaults[ExpireArgs]
	svc *Service
}

// NewExpireWorker returns the worker for ExpireArgs.
func NewExpireWorker(svc *Service) river.Worker[ExpireArgs] { return &expireWorker{svc: svc} }

func (w *expireWorker) Work(ctx context.Context, job *river.Job[ExpireArgs]) error {
	return w.svc.Expire(ctx, job.Args.OutingID)
}
