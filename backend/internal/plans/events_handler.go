package plans

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// EventHandler serves EventService.
type EventHandler struct {
	Svc *Service
	Now func() time.Time // nil = time.Now
}

func (h *EventHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *EventHandler) ListMyEvents(ctx context.Context, _ *connect.Request[isshov1.ListMyEventsRequest]) (*connect.Response[isshov1.ListMyEventsResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.Svc.Events(ctx, userID, h.now())
	if err != nil {
		return nil, eventError(err)
	}
	out := &isshov1.ListMyEventsResponse{Events: make([]*isshov1.MyEvent, len(items))}
	for i, it := range items {
		out.Events[i] = eventToProto(it)
	}
	return connect.NewResponse(out), nil
}

func (h *EventHandler) GetEvent(ctx context.Context, req *connect.Request[isshov1.GetEventRequest]) (*connect.Response[isshov1.GetEventResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	eventID, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	it, steps, names, err := h.Svc.Event(ctx, userID, eventID, h.now())
	if err != nil {
		return nil, eventError(err)
	}
	out := &isshov1.GetEventResponse{
		Event: eventToProto(it), Steps: stepsToProto(steps), Attendees: names,
		Summary: it.Summary, SecondarySummary: it.SecondarySummary,
	}
	if it.Route != nil {
		out.RoutePolyline = it.Route.Polyline
	}
	return connect.NewResponse(out), nil
}

var actions = map[isshov1.EventAction]Action{
	isshov1.EventAction_EVENT_ACTION_CONFIRM: Confirm,
	isshov1.EventAction_EVENT_ACTION_DECLINE: Decline,
	isshov1.EventAction_EVENT_ACTION_CANCEL:  Cancel,
}

func (h *EventHandler) RespondToEvent(ctx context.Context, req *connect.Request[isshov1.RespondToEventRequest]) (*connect.Response[isshov1.RespondToEventResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	eventID, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	action, ok := actions[req.Msg.Action]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("action: confirm, decline or cancel"))
	}
	now := h.now()
	if err := h.Svc.Respond(ctx, userID, eventID, action, now); err != nil {
		return nil, eventError(err)
	}
	it, _, _, err := h.Svc.Event(ctx, userID, eventID, now)
	if err != nil {
		return nil, eventError(err)
	}
	return connect.NewResponse(&isshov1.RespondToEventResponse{Event: eventToProto(it)}), nil
}

func (h *EventHandler) ListChatMessages(ctx context.Context, req *connect.Request[isshov1.ListChatMessagesRequest]) (*connect.Response[isshov1.ListChatMessagesResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	eventID, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	msgs, st, err := h.Svc.ChatMessages(ctx, userID, eventID, req.Msg.AfterId, h.now())
	if err != nil {
		return nil, eventError(err)
	}
	out := &isshov1.ListChatMessagesResponse{OpensAt: timestamp(st.OpensAt), ClosesAt: timestamp(st.ClosesAt), Open: st.Open}
	for _, m := range msgs {
		out.Messages = append(out.Messages, chatToProto(m))
	}
	return connect.NewResponse(out), nil
}

func (h *EventHandler) SendChatMessage(ctx context.Context, req *connect.Request[isshov1.SendChatMessageRequest]) (*connect.Response[isshov1.SendChatMessageResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	eventID, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	m, err := h.Svc.SendChat(ctx, userID, eventID, req.Msg.Body, h.now())
	if err != nil {
		return nil, eventError(err)
	}
	return connect.NewResponse(&isshov1.SendChatMessageResponse{Message: chatToProto(m)}), nil
}

func (h *EventHandler) CheckIn(ctx context.Context, req *connect.Request[isshov1.CheckInRequest]) (*connect.Response[isshov1.CheckInResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	eventID, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	now := h.now()
	if err := h.Svc.CheckIn(ctx, userID, eventID, now); err != nil {
		return nil, eventError(err)
	}
	it, _, _, err := h.Svc.Event(ctx, userID, eventID, now)
	if err != nil {
		return nil, eventError(err)
	}
	return connect.NewResponse(&isshov1.CheckInResponse{Event: eventToProto(it)}), nil
}

func (h *EventHandler) GetFeedback(ctx context.Context, req *connect.Request[isshov1.GetFeedbackRequest]) (*connect.Response[isshov1.GetFeedbackResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	eventID, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	f, err := h.Svc.FeedbackForm(ctx, userID, eventID, h.now())
	if err != nil {
		return nil, eventError(err)
	}
	return connect.NewResponse(feedbackToProto(f)), nil
}

func (h *EventHandler) SubmitFeedback(ctx context.Context, req *connect.Request[isshov1.SubmitFeedbackRequest]) (*connect.Response[isshov1.SubmitFeedbackResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	eventID, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	fb := Feedback{Attended: req.Msg.Attended, PlanRating: int(req.Msg.PlanRating), GroupRating: int(req.Msg.GroupRating)}
	if fb.Picks, err = parseIDs("picks", req.Msg.Picks); err != nil {
		return nil, err
	}
	if fb.Avoid, err = parseIDs("avoid", req.Msg.Avoid); err != nil {
		return nil, err
	}
	now := h.now()
	if err := h.Svc.SubmitFeedback(ctx, userID, eventID, fb, now); err != nil {
		return nil, eventError(err)
	}
	f, err := h.Svc.FeedbackForm(ctx, userID, eventID, now)
	if err != nil {
		return nil, eventError(err)
	}
	return connect.NewResponse(&isshov1.SubmitFeedbackResponse{Feedback: feedbackToProto(f)}), nil
}

var reportReasons = map[isshov1.ReportReason]string{
	isshov1.ReportReason_REPORT_REASON_DATING:     ReportDating,
	isshov1.ReportReason_REPORT_REASON_HARASSMENT: ReportHarassment,
	isshov1.ReportReason_REPORT_REASON_UNSAFE:     ReportUnsafe,
	isshov1.ReportReason_REPORT_REASON_NO_SHOW:    ReportNoShow,
	isshov1.ReportReason_REPORT_REASON_OTHER:      ReportOther,
}

func (h *EventHandler) ReportPerson(ctx context.Context, req *connect.Request[isshov1.ReportPersonRequest]) (*connect.Response[isshov1.ReportPersonResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	eventID, err := parseEventID(req.Msg.EventId)
	if err != nil {
		return nil, err
	}
	mateID, err := uuid.Parse(req.Msg.MateId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("mate_id: not a group mate id"))
	}
	reason, ok := reportReasons[req.Msg.Reason]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason: required"))
	}
	if err := h.Svc.Report(ctx, userID, eventID, mateID, reason, req.Msg.Details, h.now()); err != nil {
		return nil, eventError(err)
	}
	return connect.NewResponse(&isshov1.ReportPersonResponse{}), nil
}

func feedbackToProto(f FeedbackForm) *isshov1.GetFeedbackResponse {
	out := &isshov1.GetFeedbackResponse{
		Given: f.Given, Attended: f.Attended,
		PlanRating: int32(f.PlanRating), GroupRating: int32(f.GroupRating),
		OpensAt: timestamp(f.OpensAt), ClosesAt: timestamp(f.ClosesAt), Open: f.Open,
	}
	for _, m := range f.Mates {
		out.Mates = append(out.Mates, &isshov1.GroupMate{Id: m.ID.String(), Name: m.Name, Cancelled: m.Cancelled})
	}
	for _, p := range f.Picks {
		out.Picks = append(out.Picks, p.String())
	}
	for _, p := range f.Avoid {
		out.Avoid = append(out.Avoid, p.String())
	}
	return out
}

func parseIDs(field string, ss []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(ss))
	for i, s := range ss {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s: not a group mate id", field))
		}
		ids[i] = id
	}
	return ids, nil
}

func chatToProto(m ChatMessage) *isshov1.ChatMessage {
	return &isshov1.ChatMessage{Id: m.ID, Name: m.Name, Body: m.Body, At: timestamp(m.At), Mine: m.Mine}
}

func parseEventID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event_id: not an event id"))
	}
	return id, nil
}

func eventError(err error) error {
	var inv *InvalidError
	switch {
	case errors.As(err, &inv):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrNoEvent):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrNotGoing):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, ErrChatClosed), errors.Is(err, ErrCheckInClosed), errors.Is(err, ErrFeedbackClosed):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, ErrNoProfile), errors.Is(err, ErrEventOver), errors.Is(err, ErrTooLate), errors.Is(err, ErrNotNow):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return err
}

var (
	eventStatusPB = map[string]isshov1.EventStatus{
		"scheduled": isshov1.EventStatus_EVENT_STATUS_SCHEDULED,
		"done":      isshov1.EventStatus_EVENT_STATUS_DONE,
		"cancelled": isshov1.EventStatus_EVENT_STATUS_CANCELLED,
	}
	groupStatusPB = map[string]isshov1.GroupStatus{
		GroupInviting:  isshov1.GroupStatus_GROUP_STATUS_INVITING,
		GroupConfirmed: isshov1.GroupStatus_GROUP_STATUS_CONFIRMED,
		GroupCancelled: isshov1.GroupStatus_GROUP_STATUS_CANCELLED,
	}
	responsePB = map[string]isshov1.InviteResponse{
		Invited:   isshov1.InviteResponse_INVITE_RESPONSE_INVITED,
		Confirmed: isshov1.InviteResponse_INVITE_RESPONSE_CONFIRMED,
		Declined:  isshov1.InviteResponse_INVITE_RESPONSE_DECLINED,
		Dropped:   isshov1.InviteResponse_INVITE_RESPONSE_DROPPED,
	}
)

func timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func eventToProto(it EventItem) *isshov1.MyEvent {
	return &isshov1.MyEvent{
		Id:                 it.ID.String(),
		PlanId:             it.PlanID.String(),
		Title:              it.Title,
		SecondaryTitle:     it.SecondaryTitle,
		ActivityId:         string(it.Activity),
		FollowUpActivityId: string(it.FollowUp),
		AreaId:             string(it.Area),
		PrimaryLanguage:    it.Primary,
		SecondaryLanguage:  it.Secondary,
		Date:               it.Date.Format(time.DateOnly),
		Block:              blocksPB[it.Block],
		StartsAt:           timestamp(it.StartsAt),
		ConfirmBy:          timestamp(it.ConfirmBy),
		Status:             eventStatusPB[it.Status],
		GroupStatus:        groupStatusPB[it.GroupStatus],
		Response:           responsePB[it.Response],
		Confirmed:          int32(it.Confirmed),
		Quorum:             int32(it.Quorum),
		MinAttendance:      int32(it.MinAttendance),
		MeetingPoint:       it.MeetingPoint,
		RevealAt:           timestamp(it.RevealAt),
		CheckedIn:          it.CheckedIn,
		CheckInFrom:        timestamp(it.CheckInFrom),
		CheckInUntil:       timestamp(it.CheckInUntil),
		FeedbackFrom:       timestamp(it.FeedbackFrom),
		FeedbackUntil:      timestamp(it.FeedbackUntil),
		FeedbackGiven:      it.FeedbackGiven,
	}
}
