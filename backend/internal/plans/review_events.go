package plans

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// Group review in ReviewService (proposals.go has the rules).

var reviewEventStatuses = map[isshov1.ReviewEventStatus]string{
	isshov1.ReviewEventStatus_REVIEW_EVENT_STATUS_PROPOSED:  EventProposed,
	isshov1.ReviewEventStatus_REVIEW_EVENT_STATUS_SCHEDULED: EventScheduled,
}

func (h *ReviewHandler) ListReviewEvents(ctx context.Context, req *connect.Request[isshov1.ListReviewEventsRequest]) (*connect.Response[isshov1.ListReviewEventsResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	status, ok := reviewEventStatuses[req.Msg.Status]
	if !ok {
		return nil, errs.Invalid("status: proposed or scheduled")
	}
	q := dbgen.New(h.Svc.DB)
	rows, err := q.ListReviewEvents(ctx, dbgen.ListReviewEventsParams{
		CityID: req.Msg.CityId, Status: status, FromDate: date(h.now().UTC().Truncate(24 * time.Hour)), MaxRows: reviewRows,
	})
	if err != nil {
		return nil, err
	}
	names, err := h.Svc.activityNames(ctx, q, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListReviewEventsResponse{Events: make([]*isshov1.ReviewEvent, len(rows))}
	for i, r := range rows {
		out.Events[i] = reviewEventToProto(dbgen.GetReviewEventRow{
			ID: r.ID, PlanID: r.PlanID, Status: r.Status, Date: r.Date, Block: r.Block, StartsAt: r.StartsAt,
			ConfirmBy: r.ConfirmBy, Forced: r.Forced, CreatedAt: r.CreatedAt, Title: r.Title, ActivityID: r.ActivityID,
			FollowUpID: r.FollowUpID, AreaName: r.AreaName, Timezone: r.Timezone,
		}, names)
		out.Events[i].Groups, out.Events[i].People = r.Groups, r.People
	}
	return connect.NewResponse(out), nil
}

func (h *ReviewHandler) GetReviewEvent(ctx context.Context, req *connect.Request[isshov1.GetReviewEventRequest]) (*connect.Response[isshov1.GetReviewEventResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	q := dbgen.New(h.Svc.DB)
	ev, err := q.GetReviewEvent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, ErrNoProposal)
	}
	if err != nil {
		return nil, err
	}
	names, err := h.Svc.activityNames(ctx, q, ev.CityID)
	if err != nil {
		return nil, err
	}
	groups, err := q.ListReviewEventGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	members, err := q.ListReviewEventMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &isshov1.GetReviewEventResponse{Event: reviewEventToProto(ev, names)}
	for _, g := range groups {
		rg := &isshov1.ReviewGroup{
			Id: g.ID.String(), Quorum: int32(g.Quorum), MinAttendance: int32(g.MinAttendance), Score: g.Score, Status: g.Status,
		}
		if len(g.MeetingPoint) > 0 {
			rg.MeetingPoint = i18n.Pick(g.MeetingPoint, i18n.Authoring)
		}
		for _, m := range members {
			if m.GroupID == g.ID {
				rg.Members = append(rg.Members, &isshov1.ReviewMember{
					UserId: m.UserID.String(), Name: m.Name, Gender: m.Gender, BirthYear: m.BirthYear,
					Response: m.Response, SignedUp: m.SignedUp,
				})
			}
		}
		out.Event.People += int32(len(rg.Members))
		out.Groups = append(out.Groups, rg)
	}
	out.Event.Groups = int32(len(out.Groups))
	return connect.NewResponse(out), nil
}

func (h *ReviewHandler) MoveProposalMember(ctx context.Context, req *connect.Request[isshov1.MoveProposalMemberRequest]) (*connect.Response[isshov1.MoveProposalMemberResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	eventID, userID, err := eventAndUser(req.Msg.EventId, req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	groupID, err := uuid.Parse(req.Msg.GroupId)
	if err != nil {
		return nil, errs.Invalid("group_id: not a group id")
	}
	if err := h.Svc.MoveProposalMember(ctx, eventID, userID, groupID); err != nil {
		return nil, proposalError(err)
	}
	return connect.NewResponse(&isshov1.MoveProposalMemberResponse{}), nil
}

func (h *ReviewHandler) RemoveProposalMember(ctx context.Context, req *connect.Request[isshov1.RemoveProposalMemberRequest]) (*connect.Response[isshov1.RemoveProposalMemberResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	eventID, userID, err := eventAndUser(req.Msg.EventId, req.Msg.UserId)
	if err != nil {
		return nil, err
	}
	if err := h.Svc.RemoveProposalMember(ctx, eventID, userID); err != nil {
		return nil, proposalError(err)
	}
	return connect.NewResponse(&isshov1.RemoveProposalMemberResponse{}), nil
}

func (h *ReviewHandler) SetProposalStart(ctx context.Context, req *connect.Request[isshov1.SetProposalStartRequest]) (*connect.Response[isshov1.SetProposalStartResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	if err := h.Svc.SetProposalStart(ctx, id, req.Msg.Date, req.Msg.Time, h.now()); err != nil {
		return nil, proposalError(err)
	}
	return connect.NewResponse(&isshov1.SetProposalStartResponse{}), nil
}

func (h *ReviewHandler) SetGroupMeetingPoint(ctx context.Context, req *connect.Request[isshov1.SetGroupMeetingPointRequest]) (*connect.Response[isshov1.SetGroupMeetingPointResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.Msg.GroupId)
	if err != nil {
		return nil, errs.Invalid("group_id: not a group id")
	}
	if err := h.Svc.SetMeetingPoint(ctx, id, req.Msg.Text); err != nil {
		return nil, proposalError(err)
	}
	return connect.NewResponse(&isshov1.SetGroupMeetingPointResponse{}), nil
}

func (h *ReviewHandler) ApproveProposal(ctx context.Context, req *connect.Request[isshov1.ApproveProposalRequest]) (*connect.Response[isshov1.ApproveProposalResponse], error) {
	c, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	if err := h.Svc.ApproveProposal(ctx, id, c.UserID, h.Timing, h.now()); err != nil {
		return nil, proposalError(err)
	}
	return connect.NewResponse(&isshov1.ApproveProposalResponse{}), nil
}

func (h *ReviewHandler) RejectProposal(ctx context.Context, req *connect.Request[isshov1.RejectProposalRequest]) (*connect.Response[isshov1.RejectProposalResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	if err := h.Svc.RejectProposal(ctx, id, h.now()); err != nil {
		return nil, proposalError(err)
	}
	return connect.NewResponse(&isshov1.RejectProposalResponse{}), nil
}

func eventAndUser(event, user string) (uuid.UUID, uuid.UUID, error) {
	eventID, err := parseEventID(event)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	userID, err := uuid.Parse(user)
	if err != nil {
		return uuid.Nil, uuid.Nil, errs.Invalid("user_id: not a user id")
	}
	return eventID, userID, nil
}

// proposalError maps proposal rules to Connect codes.
func proposalError(err error) error {
	var small ErrGroupTooSmall
	switch {
	case errors.Is(err, ErrNoProposal), errors.Is(err, ErrNoSuchGroup), errors.Is(err, ErrNotInProposal):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrNotProposal), errors.Is(err, ErrProposalStale), errors.Is(err, ErrProposalTooLate), errors.As(err, &small):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, ErrBadStart), errors.Is(err, ErrMeetingPointTooLong):
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return err
}

func reviewEventToProto(e dbgen.GetReviewEventRow, names map[string]string) *isshov1.ReviewEvent {
	out := &isshov1.ReviewEvent{
		Id: e.ID.String(), PlanId: e.PlanID.String(), AreaName: e.AreaName, Block: e.Block, Forced: e.Forced,
		CreatedAt: timestamppb.New(e.CreatedAt.Time), TimeZone: e.Timezone,
	}
	for k, v := range reviewEventStatuses {
		if v == e.Status {
			out.Status = k
		}
	}
	if e.Title != nil {
		out.Title = i18n.Pick(e.Title, i18n.Authoring)
	} else {
		out.Title = Title(names[e.ActivityID], names[deref(e.FollowUpID)])
	}
	if e.Date.Valid {
		out.Date = e.Date.Time.Format(time.DateOnly)
	}
	if e.StartsAt.Valid {
		out.StartsAt = timestamppb.New(e.StartsAt.Time)
	}
	if e.ConfirmBy.Valid {
		out.ConfirmBy = timestamppb.New(e.ConfirmBy.Time)
	}
	return out
}
