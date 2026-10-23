package plans

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// CrewHandler serves CrewService.
type CrewHandler struct {
	Svc *Service
	Now func() time.Time // nil = time.Now
}

func (h *CrewHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *CrewHandler) ListMyCrews(ctx context.Context, _ *connect.Request[isshov1.ListMyCrewsRequest]) (*connect.Response[isshov1.ListMyCrewsResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.Svc.Crews(ctx, userID)
	if err != nil {
		return nil, crewError(err)
	}
	out := &isshov1.ListMyCrewsResponse{Crews: make([]*isshov1.Crew, len(items))}
	for i, it := range items {
		out.Crews[i] = crewToProto(it)
	}
	return connect.NewResponse(out), nil
}

func (h *CrewHandler) GetCrew(ctx context.Context, req *connect.Request[isshov1.GetCrewRequest]) (*connect.Response[isshov1.GetCrewResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	it, err := h.Svc.Crew(ctx, userID, crewID)
	if err != nil {
		return nil, crewError(err)
	}
	return connect.NewResponse(&isshov1.GetCrewResponse{Crew: crewToProto(it)}), nil
}

func (h *CrewHandler) RespondToCrew(ctx context.Context, req *connect.Request[isshov1.RespondToCrewRequest]) (*connect.Response[isshov1.RespondToCrewResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	if err := h.Svc.RespondToCrew(ctx, userID, crewID, req.Msg.Accept, h.now()); err != nil {
		return nil, crewError(err)
	}
	out := &isshov1.RespondToCrewResponse{}
	// After declining (or a lapsed offer), the crew is no longer theirs to see.
	if it, err := h.Svc.Crew(ctx, userID, crewID); err == nil {
		out.Crew = crewToProto(it)
	} else if !errors.Is(err, ErrNoCrew) {
		return nil, crewError(err)
	}
	return connect.NewResponse(out), nil
}

func (h *CrewHandler) ListCrewMessages(ctx context.Context, req *connect.Request[isshov1.ListCrewMessagesRequest]) (*connect.Response[isshov1.ListCrewMessagesResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	msgs, err := h.Svc.CrewMessages(ctx, userID, crewID, req.Msg.AfterId)
	if err != nil {
		return nil, crewError(err)
	}
	out := &isshov1.ListCrewMessagesResponse{Messages: make([]*isshov1.CrewMessage, len(msgs))}
	for i, m := range msgs {
		out.Messages[i] = crewMessageToProto(m)
	}
	return connect.NewResponse(out), nil
}

func (h *CrewHandler) SendCrewMessage(ctx context.Context, req *connect.Request[isshov1.SendCrewMessageRequest]) (*connect.Response[isshov1.SendCrewMessageResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	m, err := h.Svc.SendCrewMessage(ctx, userID, crewID, req.Msg.Body)
	if err != nil {
		return nil, crewError(err)
	}
	return connect.NewResponse(&isshov1.SendCrewMessageResponse{Message: crewMessageToProto(m)}), nil
}

var leaveReasons = map[isshov1.LeaveReason]string{
	isshov1.LeaveReason_LEAVE_REASON_UNCOMFORTABLE:     LeaveUncomfortable,
	isshov1.LeaveReason_LEAVE_REASON_NOT_INTERESTED:    LeaveNotInterested,
	isshov1.LeaveReason_LEAVE_REASON_INACTIVE:          LeaveInactive,
	isshov1.LeaveReason_LEAVE_REASON_TOO_MANY_MESSAGES: LeaveTooManyMessages,
	isshov1.LeaveReason_LEAVE_REASON_OTHER:             LeaveOther,
}

func (h *CrewHandler) LeaveCrew(ctx context.Context, req *connect.Request[isshov1.LeaveCrewRequest]) (*connect.Response[isshov1.LeaveCrewResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	reason, ok := leaveReasons[req.Msg.Reason]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason: required"))
	}
	if err := h.Svc.LeaveCrew(ctx, userID, crewID, reason, req.Msg.Details, h.now()); err != nil {
		return nil, crewError(err)
	}
	return connect.NewResponse(&isshov1.LeaveCrewResponse{}), nil
}

func (h *CrewHandler) ReportCrewMember(ctx context.Context, req *connect.Request[isshov1.ReportCrewMemberRequest]) (*connect.Response[isshov1.ReportCrewMemberResponse], error) {
	userID, crewID, err := crewCall(ctx, req.Msg.CrewId)
	if err != nil {
		return nil, err
	}
	mateID, err := uuid.Parse(req.Msg.MemberId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("member_id: not a crew member id"))
	}
	reason, ok := reportReasons[req.Msg.Reason]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason: required"))
	}
	if err := h.Svc.ReportCrewMember(ctx, userID, crewID, mateID, reason, req.Msg.Details, h.now()); err != nil {
		return nil, crewError(err)
	}
	return connect.NewResponse(&isshov1.ReportCrewMemberResponse{}), nil
}

func crewCall(ctx context.Context, rawID string) (uuid.UUID, uuid.UUID, error) {
	userID, err := caller(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	crewID, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errors.New("crew_id: not a crew id"))
	}
	return userID, crewID, nil
}

func crewError(err error) error {
	var inv *InvalidError
	switch {
	case errors.As(err, &inv):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrNoCrew):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrNotInCrew):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, ErrCrewOver):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return err
}

var (
	crewStatusPB = map[string]isshov1.CrewStatus{
		CrewOffered: isshov1.CrewStatus_CREW_STATUS_OFFERED,
		CrewActive:  isshov1.CrewStatus_CREW_STATUS_ACTIVE,
	}
	crewResponsePB = map[string]isshov1.CrewResponse{
		"offered":    isshov1.CrewResponse_CREW_RESPONSE_OFFERED,
		CrewAccepted: isshov1.CrewResponse_CREW_RESPONSE_ACCEPTED,
	}
)

func crewToProto(it CrewItem) *isshov1.Crew {
	out := &isshov1.Crew{
		Id: it.ID.String(), Status: crewStatusPB[it.Status], ActivityId: it.Activity, AreaId: it.Area,
		Response: crewResponsePB[it.Response],
	}
	if it.Status == CrewOffered {
		out.OfferUntil = timestamp(it.OfferUntil)
	}
	for _, m := range it.Members {
		out.Members = append(out.Members, &isshov1.CrewMember{Id: m.ID.String(), Name: m.Name, Accepted: m.Accepted, Me: m.Me})
	}
	return out
}

func crewMessageToProto(m CrewMessage) *isshov1.CrewMessage {
	out := &isshov1.CrewMessage{Id: m.ID, Name: m.Name, Body: m.Body, At: timestamp(m.At), Mine: m.Mine}
	if m.PlanID != uuid.Nil {
		out.PlanId = m.PlanID.String()
	}
	return out
}
