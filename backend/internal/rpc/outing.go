package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/outings"
)

type outingService struct {
	outings *outings.Service
}

// ListMyOutings lists the caller's own seats' outings. The account is the
// interceptor's verified identity and the request names no other and no
// outing, so nobody is shown an outing they have no seat in.
func (s outingService) ListMyOutings(ctx context.Context, _ *connect.Request[isshov1.ListMyOutingsRequest]) (*connect.Response[isshov1.ListMyOutingsResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	seats, err := s.outings.Mine(ctx, id.Subject)
	if err != nil {
		// The error may quote the database; the caller gets none of it.
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the outings"))
	}
	out := make([]*isshov1.Outing, len(seats))
	for i, seat := range seats {
		out[i] = outingOf(seat)
	}
	return connect.NewResponse(&isshov1.ListMyOutingsResponse{Outings: out}), nil
}

// AnswerInvitation records the caller's yes or no to their own invitation. The
// seat is the caller's, from the interceptor's verified identity; the request
// names an outing and nobody else.
func (s outingService) AnswerInvitation(ctx context.Context, req *connect.Request[isshov1.AnswerInvitationRequest]) (*connect.Response[isshov1.AnswerInvitationResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	var answer outings.Answer
	switch req.Msg.GetAnswer() {
	case isshov1.InvitationAnswer_INVITATION_ANSWER_YES:
		answer = outings.Yes
	case isshov1.InvitationAnswer_INVITATION_ANSWER_NO:
		answer = outings.No
	}
	seat, err := s.outings.Answer(ctx, id.Subject, req.Msg.GetOutingId(), answer)
	if err != nil {
		return nil, outingError(err, "could not record the answer")
	}
	return connect.NewResponse(&isshov1.AnswerInvitationResponse{Outing: outingOf(seat)}), nil
}

// CancelSeat cancels the caller's own seat in an outing they are going to.
func (s outingService) CancelSeat(ctx context.Context, req *connect.Request[isshov1.CancelSeatRequest]) (*connect.Response[isshov1.CancelSeatResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	seat, err := s.outings.Cancel(ctx, id.Subject, req.Msg.GetOutingId())
	if err != nil {
		return nil, outingError(err, "could not cancel the seat")
	}
	return connect.NewResponse(&isshov1.CancelSeatResponse{Outing: outingOf(seat)}), nil
}

// CheckIn records the caller's "I'm here" for their own seat. The seat is the
// caller's, from the interceptor's verified identity.
func (s outingService) CheckIn(ctx context.Context, req *connect.Request[isshov1.CheckInRequest]) (*connect.Response[isshov1.CheckInResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	seat, err := s.outings.CheckIn(ctx, id.Subject, req.Msg.GetOutingId())
	if err != nil {
		return nil, outingError(err, "could not check in")
	}
	return connect.NewResponse(&isshov1.CheckInResponse{Outing: outingOf(seat)}), nil
}

// GetFeedback is the caller's own feedback form for an outing they were going to
// at the start: the others by first name and handle, and the caller's answers.
func (s outingService) GetFeedback(ctx context.Context, req *connect.Request[isshov1.GetFeedbackRequest]) (*connect.Response[isshov1.GetFeedbackResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	f, err := s.outings.Feedback(ctx, id.Subject, req.Msg.GetOutingId())
	if err != nil {
		return nil, outingError(err, "could not read the feedback")
	}
	return connect.NewResponse(&isshov1.GetFeedbackResponse{Feedback: feedbackOf(f)}), nil
}

// SaveFeedback records the caller's answers to feedback's first three questions.
// Nothing it returns says who picked whom: the response holds the caller's own
// answers only.
func (s outingService) SaveFeedback(ctx context.Context, req *connect.Request[isshov1.SaveFeedbackRequest]) (*connect.Response[isshov1.SaveFeedbackResponse], error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("no identity"))
	}
	f, err := s.outings.SaveFeedback(ctx, id.Subject, req.Msg.GetOutingId(), outings.FeedbackInput{
		Went: req.Msg.GetWent(), NotThere: req.Msg.GetNotThere(), MeetAgain: req.Msg.GetMeetAgain(),
	})
	if err != nil {
		return nil, outingError(err, "could not save the feedback")
	}
	return connect.NewResponse(&isshov1.SaveFeedbackResponse{Feedback: feedbackOf(f)}), nil
}

// feedbackOf is a form as the API shows it: first names and handles of the
// others, and the caller's own answers.
func feedbackOf(f outings.Feedback) *isshov1.Feedback {
	others := make([]*isshov1.Companion, len(f.Others))
	for i, c := range f.Others {
		others[i] = &isshov1.Companion{Handle: c.Handle, FirstName: c.FirstName}
	}
	return &isshov1.Feedback{
		OutingId: f.OutingID, Answered: f.Answered, Went: f.Went, Others: others,
		NotThere: f.NotThere, MeetAgain: f.MeetAgain, ClosesAt: timestamppb.New(f.ClosesAt),
	}
}

// outingOf is a seat as the API shows it: no other person but the first names of
// those going, which the seat carries only for a person who is going.
func outingOf(seat outings.Seat) *isshov1.Outing {
	o := &isshov1.Outing{
		Id: seat.OutingID, PlanId: seat.PlanID, Date: seat.Date, Part: seat.Part, StartsAt: seat.StartsAt,
		Languages: seat.Languages, SeatState: seatState(seat.State),
		InvitedAt: timestamppb.New(seat.InvitedAt), InvitationEndsAt: timestamppb.New(seat.InvitationEndsAt),
		OutingState: outingState(seat.OutingState), YesCount: int32(seat.YesCount), OthersGoing: seat.GoingFirstNames,
		CheckedIn: seat.CheckedIn, CheckedInNames: seat.CheckedInFirstNames,
		CheckInOpensAt: timestamppb.New(seat.CheckInOpensAt), CheckInClosesAt: timestamppb.New(seat.CheckInClosesAt),
		FeedbackOpensAt: timestamppb.New(seat.FeedbackOpensAt), FeedbackClosesAt: timestamppb.New(seat.FeedbackClosesAt),
	}
	if seat.CheckedIn {
		o.CheckedInAt = timestamppb.New(seat.CheckedInAt)
	}
	return o
}

// outingError is the status for a refused call. An error that isn't one of the
// outings' own may quote the database, so the caller gets only msg.
func outingError(err error, msg string) error {
	switch {
	case errors.Is(err, outings.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("no such outing"))
	case errors.Is(err, outings.ErrInvalid):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("the request is not one the outing takes"))
	case errors.Is(err, outings.ErrInvitationClosed):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the invitation is closed"))
	case errors.Is(err, outings.ErrTimeTaken):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("you are going to another outing at that time"))
	case errors.Is(err, outings.ErrNotGoing):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the seat is not going"))
	case errors.Is(err, outings.ErrCheckedIn):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("you have checked in, so there is no cancel"))
	case errors.Is(err, outings.ErrCheckInClosed):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("check-in is not open"))
	case errors.Is(err, outings.ErrFeedbackNotOpen):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("feedback is not open yet"))
	case errors.Is(err, outings.ErrFeedbackClosed):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("feedback is closed"))
	case errors.Is(err, outings.ErrNotThere):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("you were not going at the start"))
	}
	return connect.NewError(connect.CodeInternal, errors.New(msg))
}

// outingState is an outing's stored state as the API names it.
func outingState(s outings.OutingState) isshov1.OutingState {
	switch s {
	case outings.OutingProposed:
		return isshov1.OutingState_OUTING_STATE_PROPOSED
	case outings.OutingOffered:
		return isshov1.OutingState_OUTING_STATE_OFFERED
	case outings.OutingSet:
		return isshov1.OutingState_OUTING_STATE_SET
	case outings.OutingOver:
		return isshov1.OutingState_OUTING_STATE_OVER
	case outings.OutingNotSet:
		return isshov1.OutingState_OUTING_STATE_NOT_SET
	case outings.OutingCancelled:
		return isshov1.OutingState_OUTING_STATE_CANCELLED
	default:
		return isshov1.OutingState_OUTING_STATE_UNSPECIFIED
	}
}

// seatState is a seat's stored state as the API names it.
func seatState(s outings.SeatState) isshov1.SeatState {
	switch s {
	case outings.SeatInvited:
		return isshov1.SeatState_SEAT_STATE_INVITED
	case outings.SeatAccepted:
		return isshov1.SeatState_SEAT_STATE_ACCEPTED
	case outings.SeatGoing:
		return isshov1.SeatState_SEAT_STATE_GOING
	case outings.SeatDeclined:
		return isshov1.SeatState_SEAT_STATE_DECLINED
	case outings.SeatUnanswered:
		return isshov1.SeatState_SEAT_STATE_UNANSWERED
	case outings.SeatFull:
		return isshov1.SeatState_SEAT_STATE_FULL
	case outings.SeatCancelled:
		return isshov1.SeatState_SEAT_STATE_CANCELLED
	case outings.SeatLateCancel:
		return isshov1.SeatState_SEAT_STATE_LATE_CANCEL
	case outings.SeatRemoved:
		return isshov1.SeatState_SEAT_STATE_REMOVED
	default:
		return isshov1.SeatState_SEAT_STATE_UNSPECIFIED
	}
}
