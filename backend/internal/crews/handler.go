package crews

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// Handler serves CrewService.
type Handler struct {
	Svc *Service
	Now func() time.Time // nil = time.Now
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) ListMyCrews(ctx context.Context, _ *connect.Request[isshov1.ListMyCrewsRequest]) (*connect.Response[isshov1.ListMyCrewsResponse], error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.Svc.List(ctx, c.UserID)
	if err != nil {
		return nil, crewError(err)
	}
	out := &isshov1.ListMyCrewsResponse{Crews: make([]*isshov1.Crew, len(items))}
	for i, it := range items {
		out.Crews[i] = crewToProto(it)
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetCrew(ctx context.Context, req *connect.Request[isshov1.GetCrewRequest]) (*connect.Response[isshov1.GetCrewResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	it, err := h.Svc.Get(ctx, userID, crewID)
	if err != nil {
		return nil, crewError(err)
	}
	return connect.NewResponse(&isshov1.GetCrewResponse{Crew: crewToProto(it)}), nil
}

func (h *Handler) RespondToCrew(ctx context.Context, req *connect.Request[isshov1.RespondToCrewRequest]) (*connect.Response[isshov1.RespondToCrewResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	if err := h.Svc.Respond(ctx, userID, crewID, req.Msg.Accept, h.now()); err != nil {
		return nil, crewError(err)
	}
	out := &isshov1.RespondToCrewResponse{}
	// After declining (or a lapsed offer), the crew is no longer theirs to see.
	if it, err := h.Svc.Get(ctx, userID, crewID); err == nil {
		out.Crew = crewToProto(it)
	} else if !errors.Is(err, ErrNoCrew) {
		return nil, crewError(err)
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ListCrewMessages(ctx context.Context, req *connect.Request[isshov1.ListCrewMessagesRequest]) (*connect.Response[isshov1.ListCrewMessagesResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	msgs, err := h.Svc.Messages(ctx, userID, crewID, req.Msg.AfterId)
	if err != nil {
		return nil, crewError(err)
	}
	out := &isshov1.ListCrewMessagesResponse{Messages: make([]*isshov1.CrewMessage, len(msgs))}
	for i, m := range msgs {
		out.Messages[i] = messageToProto(m)
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) SendCrewMessage(ctx context.Context, req *connect.Request[isshov1.SendCrewMessageRequest]) (*connect.Response[isshov1.SendCrewMessageResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	m, err := h.Svc.Send(ctx, userID, crewID, req.Msg.Body)
	if err != nil {
		return nil, crewError(err)
	}
	return connect.NewResponse(&isshov1.SendCrewMessageResponse{Message: messageToProto(m)}), nil
}

var leaveReasons = map[isshov1.LeaveReason]string{
	isshov1.LeaveReason_LEAVE_REASON_UNCOMFORTABLE:     LeaveUncomfortable,
	isshov1.LeaveReason_LEAVE_REASON_NOT_INTERESTED:    LeaveNotInterested,
	isshov1.LeaveReason_LEAVE_REASON_INACTIVE:          LeaveInactive,
	isshov1.LeaveReason_LEAVE_REASON_TOO_MANY_MESSAGES: LeaveTooManyMessages,
	isshov1.LeaveReason_LEAVE_REASON_OTHER:             LeaveOther,
}

func (h *Handler) LeaveCrew(ctx context.Context, req *connect.Request[isshov1.LeaveCrewRequest]) (*connect.Response[isshov1.LeaveCrewResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	reason, ok := leaveReasons[req.Msg.Reason]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason: required"))
	}
	if err := h.Svc.Leave(ctx, userID, crewID, reason, req.Msg.Details, h.now()); err != nil {
		return nil, crewError(err)
	}
	return connect.NewResponse(&isshov1.LeaveCrewResponse{}), nil
}

func (h *Handler) ReportCrewMember(ctx context.Context, req *connect.Request[isshov1.ReportCrewMemberRequest]) (*connect.Response[isshov1.ReportCrewMemberResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	mateID, err := uuid.Parse(req.Msg.MemberId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("member_id: not a crew member id"))
	}
	reason, ok := plans.ReportReasons[req.Msg.Reason]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason: required"))
	}
	if err := h.Svc.ReportMember(ctx, userID, crewID, mateID, reason, req.Msg.Details, h.now()); err != nil {
		return nil, crewError(err)
	}
	return connect.NewResponse(&isshov1.ReportCrewMemberResponse{}), nil
}

func crewCall(ctx context.Context, rawID string) (uuid.UUID, uuid.UUID, error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	crewID, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errors.New("crew_id: not a crew id"))
	}
	return c.UserID, crewID, nil
}

func crewError(err error) error {
	var inv *plans.InvalidError
	switch {
	case errors.As(err, &inv):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrNoCrew):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrNotMember):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, ErrOver):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return err
}

var (
	statusPB = map[string]isshov1.CrewStatus{
		Offered: isshov1.CrewStatus_CREW_STATUS_OFFERED,
		Active:  isshov1.CrewStatus_CREW_STATUS_ACTIVE,
	}
	responsePB = map[string]isshov1.CrewResponse{
		Undecided: isshov1.CrewResponse_CREW_RESPONSE_OFFERED,
		Accepted:  isshov1.CrewResponse_CREW_RESPONSE_ACCEPTED,
	}
)

func crewToProto(it Item) *isshov1.Crew {
	out := &isshov1.Crew{
		Id: it.ID.String(), Status: statusPB[it.Status], ActivityId: it.Activity, AreaId: it.Area,
		Response: responsePB[it.Response],
	}
	if it.Status == Offered {
		out.OfferUntil = timestamppb.New(it.OfferUntil)
	}
	for _, m := range it.Members {
		out.Members = append(out.Members, &isshov1.CrewMember{Id: m.ID.String(), Name: m.Name, Accepted: m.Accepted, Me: m.Me})
	}
	return out
}

func messageToProto(m Message) *isshov1.CrewMessage {
	out := &isshov1.CrewMessage{Id: m.ID, Name: m.Name, Body: m.Body, At: timestamppb.New(m.At), Mine: m.Mine}
	if m.PlanID != uuid.Nil {
		out.PlanId = m.PlanID.String()
	}
	return out
}
