//go:build e2e

// Package e2eclock is the test-only surface of the end-to-end build: the RPCs
// that move the API's clock and trigger forming, which nothing schedules yet
// (docs/spec/quality.md, "End to end"). Every file is under the `e2e` build tag
// and only cmd/api/e2e_on.go imports it, so the production api and worker hold
// none of it, and the generated code it serves (gen/issho/e2e) is in a package
// of its own that production never imports. `cmd/cmd_test.go` and
// `scripts/image.sh` prove that.
//
// Each RPC declares an access rule in proto/issho/e2e/v1, like every other: the
// interceptor refuses one that has none. They are public, which is safe in a
// build that has a throwaway database and is never deployed.
package e2eclock

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"google.golang.org/protobuf/types/known/timestamppb"

	e2ev1 "github.com/shrujancheruku/issho/backend/gen/issho/e2e/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/e2e/v1/e2ev1connect"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
)

// Former forms a plan's groups the way the schedule will, and returns once it
// has been done.
type Former interface {
	Form(ctx context.Context, planID string) error
}

// Mounts are the services for rpc.Config.Mounts. The API's handlers and the
// jobs the process works read clk, so moving it moves both. log gets a forming
// error the caller is not told.
func Mounts(log *slog.Logger, clk *clock.Fake, former Former) []rpc.Mount {
	return []rpc.Mount{
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return e2ev1connect.NewClockServiceHandler(clockService{clk: clk}, opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return e2ev1connect.NewFormingServiceHandler(formingService{log: log, former: former}, opts...)
		},
	}
}

type clockService struct{ clk *clock.Fake }

func (s clockService) GetTime(context.Context, *connect.Request[e2ev1.GetTimeRequest]) (*connect.Response[e2ev1.GetTimeResponse], error) {
	return connect.NewResponse(&e2ev1.GetTimeResponse{Now: timestamppb.New(s.clk.Now())}), nil
}

func (s clockService) SetTime(_ context.Context, req *connect.Request[e2ev1.SetTimeRequest]) (*connect.Response[e2ev1.SetTimeResponse], error) {
	if req.Msg.GetNow() == nil || req.Msg.GetNow().CheckValid() != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("SetTime needs an instant"))
	}
	s.clk.Set(req.Msg.GetNow().AsTime())
	return connect.NewResponse(&e2ev1.SetTimeResponse{Now: timestamppb.New(s.clk.Now())}), nil
}

func (s clockService) AdvanceTime(_ context.Context, req *connect.Request[e2ev1.AdvanceTimeRequest]) (*connect.Response[e2ev1.AdvanceTimeResponse], error) {
	by := req.Msg.GetBy().AsDuration()
	if req.Msg.GetBy().CheckValid() != nil || by < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("AdvanceTime needs a duration that is not negative"))
	}
	s.clk.Advance(by)
	return connect.NewResponse(&e2ev1.AdvanceTimeResponse{Now: timestamppb.New(s.clk.Now())}), nil
}

type formingService struct {
	log    *slog.Logger
	former Former
}

func (s formingService) FormPlan(ctx context.Context, req *connect.Request[e2ev1.FormPlanRequest]) (*connect.Response[e2ev1.FormPlanResponse], error) {
	if req.Msg.GetPlanId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("FormPlan needs a plan"))
	}
	if err := s.former.Form(ctx, req.Msg.GetPlanId()); err != nil {
		// The error may quote River or the database, so it goes to the log and
		// the caller gets a fixed message, as from every other handler.
		s.log.ErrorContext(ctx, "e2e: forming failed", "plan", req.Msg.GetPlanId(), "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not form the plan"))
	}
	return connect.NewResponse(&e2ev1.FormPlanResponse{}), nil
}

// riverFormer inserts outings.FormArgs, the job a schedule will insert, and
// waits for the client's worker to finish it.
type riverFormer struct{ client *river.Client[pgx.Tx] }

// NewRiverFormer returns a Former over a River client that is started and has
// the forming worker registered (outings.AddWorker).
func NewRiverFormer(client *river.Client[pgx.Tx]) Former { return riverFormer{client: client} }

// waitLimit is how long Form waits for the job. Forming a handful of people
// takes milliseconds; the limit only keeps a lost job from hanging a test run.
const waitLimit = 30 * time.Second

func (f riverFormer) Form(ctx context.Context, planID string) error {
	ctx, cancel := context.WithTimeout(ctx, waitLimit)
	defer cancel()
	// Subscribe before inserting, so the event of a quick job isn't missed.
	events, unsubscribe := f.client.Subscribe(river.EventKindJobCompleted, river.EventKindJobFailed, river.EventKindJobCancelled)
	defer unsubscribe()
	res, err := f.client.Insert(ctx, outings.FormArgs{PlanID: planID}, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case ev := <-events:
			if ev == nil || ev.Job.ID != res.Job.ID {
				continue
			}
			if ev.Kind != river.EventKindJobCompleted {
				return fmt.Errorf("the forming job %s", ev.Job.State)
			}
			return nil
		case <-ctx.Done():
			return fmt.Errorf("the forming job did not finish: %w", ctx.Err())
		}
	}
}
