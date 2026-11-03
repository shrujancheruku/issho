package adminjobs

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"google.golang.org/protobuf/types/known/timestamppb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// Jobs reads a job by id (*river.Client).
type Jobs interface {
	JobGet(ctx context.Context, id int64) (*rivertype.JobRow, error)
}

// Handler serves AdminService. cmd/api limits the service to admins
// (auth.AdminOnly); each call checks again so a wiring mistake fails closed.
type Handler struct {
	Jobs Jobs
}

func (h *Handler) GetAdminJob(ctx context.Context, req *connect.Request[isshov1.GetAdminJobRequest]) (*connect.Response[isshov1.GetAdminJobResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	row, err := h.Jobs.JobGet(ctx, req.Msg.JobId)
	if errors.Is(err, river.ErrNotFound) {
		return nil, errs.NotFound("no such job")
	}
	if err != nil {
		return nil, err
	}
	// Only the admin operations: other kinds (matching, pushes) are none of
	// the admin SPA's business.
	if row.Kind != KindApplyCityPack && row.Kind != KindDiscoverAreaVenues {
		return nil, errs.NotFound("no such job")
	}
	out, err := Describe(row)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// State is a River job state as the admin sees it. A job waiting to retry is
// still queued; a cancelled or discarded one has failed.
func State(s rivertype.JobState) isshov1.AdminJobState {
	switch s {
	case rivertype.JobStateRunning:
		return isshov1.AdminJobState_ADMIN_JOB_STATE_RUNNING
	case rivertype.JobStateCompleted:
		return isshov1.AdminJobState_ADMIN_JOB_STATE_SUCCEEDED
	case rivertype.JobStateCancelled, rivertype.JobStateDiscarded:
		return isshov1.AdminJobState_ADMIN_JOB_STATE_FAILED
	default: // available, pending, scheduled, retryable
		return isshov1.AdminJobState_ADMIN_JOB_STATE_QUEUED
	}
}

// Describe is a job row as GetAdminJob returns it: River's state and times,
// and the output the worker recorded.
func Describe(row *rivertype.JobRow) (*isshov1.GetAdminJobResponse, error) {
	out := &isshov1.GetAdminJobResponse{JobId: row.ID, State: State(row.State), QueuedAt: timestamppb.New(row.CreatedAt)}
	if row.AttemptedAt != nil {
		out.StartedAt = timestamppb.New(*row.AttemptedAt)
	}
	if row.FinalizedAt != nil {
		out.FinishedAt = timestamppb.New(*row.FinalizedAt)
	}
	// The output is overwritten by each attempt and means nothing until the
	// job is done: a retry's earlier failure isn't this job's result.
	if out.State != isshov1.AdminJobState_ADMIN_JOB_STATE_SUCCEEDED && out.State != isshov1.AdminJobState_ADMIN_JOB_STATE_FAILED {
		return out, nil
	}
	var rec Output
	if raw := row.Output(); len(raw) > 0 {
		if err := json.Unmarshal(raw, &rec); err != nil {
			return nil, err
		}
	}
	switch {
	case out.State == isshov1.AdminJobState_ADMIN_JOB_STATE_FAILED:
		// No output: the worker died before it could record one (River
		// discards a job it rescues too often) or the record was lost.
		out.Failure = &isshov1.AdminJobFailure{Message: msgInternal}
		if rec.Failure != nil {
			out.Failure = &isshov1.AdminJobFailure{Message: rec.Failure.Message, Reason: rec.Failure.Reason}
		}
	case rec.ApplyCityPack != nil:
		out.Result = &isshov1.GetAdminJobResponse_ApplyCityPack{ApplyCityPack: ApplyReport(*rec.ApplyCityPack)}
	case rec.DiscoverAreaVenues != nil:
		out.Result = &isshov1.GetAdminJobResponse_DiscoverAreaVenues{DiscoverAreaVenues: &isshov1.DiscoverAreaVenuesResponse{
			Searches: int32(rec.DiscoverAreaVenues.Searches), Venues: int32(rec.DiscoverAreaVenues.Venues),
		}}
	}
	return out, nil
}

// ApplyReport is an apply's diff as the response the RPC returned before it
// became a job.
func ApplyReport(d cities.Diff) *isshov1.ApplyCityPackResponse {
	replaced := make([]*isshov1.ReplacedArea, 0, len(d.ReplacedAreas))
	for _, r := range d.ReplacedAreas {
		replaced = append(replaced, &isshov1.ReplacedArea{
			OldAreaId: r.Old, NewAreaId: r.New, People: int32(r.People), Crews: int32(r.Crews), Plans: int32(r.Plans),
		})
	}
	return &isshov1.ApplyCityPackResponse{
		NewAreas: d.NewAreas, MovedAreas: d.MovedAreas, TravelTimesRebuilt: d.TravelTimesRebuilt, TravelTimes: int32(d.TravelTimes),
		RemovedAreas: d.RemovedAreas, ReplacedAreas: replaced, RemovedOverrides: d.RemovedOverrides,
	}
}
