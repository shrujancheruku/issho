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
		out[i] = &isshov1.Outing{
			Id: seat.OutingID, PlanId: seat.PlanID, Date: seat.Date, Part: seat.Part, StartsAt: seat.StartsAt,
			Languages: seat.Languages, SeatState: seatState(seat.State),
			InvitedAt: timestamppb.New(seat.InvitedAt), InvitationEndsAt: timestamppb.New(seat.InvitationEndsAt),
		}
	}
	return connect.NewResponse(&isshov1.ListMyOutingsResponse{Outings: out}), nil
}

// seatState is a seat's stored state as the API names it.
func seatState(s outings.SeatState) isshov1.SeatState {
	switch s {
	case outings.SeatInvited:
		return isshov1.SeatState_SEAT_STATE_INVITED
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
