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
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// Handler serves PlanService.
type Handler struct {
	Svc *Service
}

func (h *Handler) ListFeed(ctx context.Context, _ *connect.Request[isshov1.ListFeedRequest]) (*connect.Response[isshov1.ListFeedResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.Svc.Feed(ctx, userID)
	if err != nil {
		return nil, connectError(err)
	}
	out := &isshov1.ListFeedResponse{Plans: make([]*isshov1.FeedPlan, len(items))}
	for i, it := range items {
		out.Plans[i] = planToProto(it)
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetPlan(ctx context.Context, req *connect.Request[isshov1.GetPlanRequest]) (*connect.Response[isshov1.GetPlanResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	planID, err := parsePlanID(req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	item, steps, signup, err := h.Svc.Plan(ctx, userID, planID)
	if err != nil {
		return nil, connectError(err)
	}
	out := &isshov1.GetPlanResponse{Plan: planToProto(item)}
	for _, st := range steps {
		out.Steps = append(out.Steps, &isshov1.PlanStep{
			ActivityId:         string(st.Activity),
			VenueName:          st.Venue.Name,
			VenueSecondaryName: st.SecondaryName,
			VenueAddress:       st.Venue.Address,
			VenueMapsUrl:       st.Venue.MapsURL,
			WalkMinutes:        int32(st.WalkMinutes),
		})
	}
	if signup != nil {
		out.Signup = signupToProto(*signup)
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) SignUp(ctx context.Context, req *connect.Request[isshov1.SignUpRequest]) (*connect.Response[isshov1.SignUpResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	planID, err := parsePlanID(req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	signup, err := signupFromProto(req.Msg.Signup)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	item, err := h.Svc.SignUp(ctx, userID, planID, signup)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&isshov1.SignUpResponse{Plan: planToProto(item)}), nil
}

func (h *Handler) Withdraw(ctx context.Context, req *connect.Request[isshov1.WithdrawRequest]) (*connect.Response[isshov1.WithdrawResponse], error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	planID, err := parsePlanID(req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	if err := h.Svc.Withdraw(ctx, userID, planID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.WithdrawResponse{}), nil
}

func caller(ctx context.Context) (uuid.UUID, error) {
	c, ok := auth.FromContext(ctx)
	if !ok {
		return uuid.Nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no caller"))
	}
	return c.UserID, nil
}

func parsePlanID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, connect.NewError(connect.CodeInvalidArgument, errors.New("plan_id: not a plan id"))
	}
	return id, nil
}

func connectError(err error) error {
	var inv *InvalidError
	switch {
	case errors.As(err, &inv):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ErrNoProfile), errors.Is(err, ErrNotForYou):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return err
}

var (
	levelsPB = map[matching.Level]isshov1.PlanLevel{
		matching.NotForYou: isshov1.PlanLevel_PLAN_LEVEL_NOT_FOR_YOU,
		matching.Low:       isshov1.PlanLevel_PLAN_LEVEL_LOW,
		matching.Maybe:     isshov1.PlanLevel_PLAN_LEVEL_MAYBE,
		matching.Good:      isshov1.PlanLevel_PLAN_LEVEL_GOOD,
		matching.Top:       isshov1.PlanLevel_PLAN_LEVEL_TOP,
	}
	blocks = map[isshov1.TimeBlock]matching.Block{
		isshov1.TimeBlock_TIME_BLOCK_MORNING:   matching.Morning,
		isshov1.TimeBlock_TIME_BLOCK_AFTERNOON: matching.Afternoon,
		isshov1.TimeBlock_TIME_BLOCK_EVENING:   matching.Evening,
	}
	blocksPB = map[matching.Block]isshov1.TimeBlock{
		matching.Morning:   isshov1.TimeBlock_TIME_BLOCK_MORNING,
		matching.Afternoon: isshov1.TimeBlock_TIME_BLOCK_AFTERNOON,
		matching.Evening:   isshov1.TimeBlock_TIME_BLOCK_EVENING,
		"":                 isshov1.TimeBlock_TIME_BLOCK_UNSPECIFIED, // the whole day
	}
)

func planToProto(it FeedItem) *isshov1.FeedPlan {
	return &isshov1.FeedPlan{
		Id:                 it.ID.String(),
		AreaId:             string(it.Area),
		ActivityId:         string(it.Activity),
		FollowUpActivityId: string(it.FollowUp),
		PrimaryLanguage:    it.Primary,
		SecondaryLanguage:  it.Secondary,
		Signups:            int32(it.Signups),
		Level:              levelsPB[it.Level],
		TravelMinutes:      int32(it.Minutes),
		WindowStart:        it.Window.Start.Format(time.DateOnly),
		WindowDays:         int32(it.Window.Days),
		SignupDeadline:     timestamppb.New(it.Window.Deadline),
		SignedUp:           it.SignedUp,
		Title:              it.Title,
		SecondaryTitle:     it.SecondaryTitle,
	}
}

func signupToProto(s matching.Signup) *isshov1.Signup {
	out := &isshov1.Signup{}
	for _, sl := range s.Pattern {
		out.Pattern = append(out.Pattern, &isshov1.AvailabilitySlot{Weekday: int32(sl.Weekday), Block: blocksPB[sl.Block]})
	}
	for _, o := range s.Unavailable {
		out.Unavailable = append(out.Unavailable, &isshov1.UnavailableTime{Date: o.Date.Format(time.DateOnly), Block: blocksPB[o.Block]})
	}
	return out
}

// signupFromProto checks the times are well formed; whether they fall in
// the plan's window is the service's job. A missing signup means "my usual
// availability, any date".
func signupFromProto(m *isshov1.Signup) (matching.Signup, error) {
	var s matching.Signup
	if m == nil {
		return s, nil
	}
	seen := map[matching.Slot]bool{}
	for _, sl := range m.Pattern {
		b, ok := blocks[sl.Block]
		if !ok || sl.Weekday < 0 || sl.Weekday > 6 {
			return s, fmt.Errorf("pattern: weekday 0–6 and a time block required")
		}
		slot := matching.Slot{Weekday: time.Weekday(sl.Weekday), Block: b}
		if seen[slot] {
			return s, fmt.Errorf("pattern: %s %s listed twice", slot.Weekday, slot.Block)
		}
		seen[slot] = true
		s.Pattern = append(s.Pattern, slot)
	}
	seenTimes := map[matching.Occasion]bool{}
	for _, u := range m.Unavailable {
		d, err := time.Parse(time.DateOnly, u.Date)
		if err != nil {
			return s, fmt.Errorf("unavailable: %q is not a date (YYYY-MM-DD)", u.Date)
		}
		var b matching.Block
		if u.Block != isshov1.TimeBlock_TIME_BLOCK_UNSPECIFIED {
			if b, err = blockOf(u.Block); err != nil {
				return s, err
			}
		}
		o := matching.Occasion{Date: d, Block: b}
		if seenTimes[o] {
			return s, fmt.Errorf("unavailable: %s listed twice", u.Date)
		}
		seenTimes[o] = true
		s.Unavailable = append(s.Unavailable, o)
	}
	return s, nil
}

func blockOf(b isshov1.TimeBlock) (matching.Block, error) {
	if v, ok := blocks[b]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unavailable: unknown time block %d", b)
}
