package plans

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// Limits on what admins write and read in plan review.
const (
	MaxTitle        = 80
	MaxSummary      = 500
	reviewRows      = 200
	venueOptionRows = 20
)

// ReviewHandler serves ReviewService: plan review in the admin SPA (M8).
// cmd/api limits the service to admins (auth.AdminOnly); admin() checks
// again so a wiring mistake fails closed.
type ReviewHandler struct {
	Svc *Service
	// Timing sets a round's window when a plan opens (approved or made by
	// hand); the worker's, so both agree.
	Timing Timing
	Now    func() time.Time // nil = time.Now
	// For editing stops (stops.go): finding and checking places, and the
	// walk through them. nil without GOOGLE_MAPS_API_KEY.
	Places maps.PlaceSearch
	Routes maps.WalkRouter
}

func (h *ReviewHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func admin(ctx context.Context) (auth.Caller, error) {
	c, ok := auth.FromContext(ctx)
	if !ok {
		return c, connect.NewError(connect.CodeUnauthenticated, errors.New("no caller"))
	}
	if !c.Admin {
		return c, connect.NewError(connect.CodePermissionDenied, errors.New("admins only"))
	}
	return c, nil
}

func reviewInvalid(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

var reviewStatuses = map[isshov1.ReviewPlanStatus]string{
	isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_DRAFT:  PlanDraft,
	isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_OPEN:   PlanOpen,
	isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_CLOSED: PlanClosed,
}

func (h *ReviewHandler) ListReviewCities(ctx context.Context, _ *connect.Request[isshov1.ListReviewCitiesRequest]) (*connect.Response[isshov1.ListReviewCitiesResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	rows, err := dbgen.New(h.Svc.DB).ListReviewCities(ctx)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListReviewCitiesResponse{Cities: make([]*isshov1.ReviewCity, len(rows))}
	for i, r := range rows {
		out.Cities[i] = &isshov1.ReviewCity{Id: r.ID, Name: r.Name, Review: r.Review, Drafts: r.Drafts, Open: r.Open}
	}
	return connect.NewResponse(out), nil
}

func (h *ReviewHandler) SetCityReview(ctx context.Context, req *connect.Request[isshov1.SetCityReviewRequest]) (*connect.Response[isshov1.SetCityReviewResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	n, err := dbgen.New(h.Svc.DB).SetCityReview(ctx, dbgen.SetCityReviewParams{ID: req.Msg.CityId, Review: req.Msg.Review})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such city"))
	}
	return connect.NewResponse(&isshov1.SetCityReviewResponse{}), nil
}

func (h *ReviewHandler) ListReviewPlans(ctx context.Context, req *connect.Request[isshov1.ListReviewPlansRequest]) (*connect.Response[isshov1.ListReviewPlansResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	status, ok := reviewStatuses[req.Msg.Status]
	if !ok {
		return nil, reviewInvalid("status: draft, open or closed")
	}
	q := dbgen.New(h.Svc.DB)
	rows, err := q.ListReviewPlans(ctx, dbgen.ListReviewPlansParams{CityID: req.Msg.CityId, Status: status, MaxRows: reviewRows})
	if err != nil {
		return nil, err
	}
	names, err := englishActivityNames(ctx, q, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListReviewPlansResponse{Plans: make([]*isshov1.ReviewPlan, len(rows))}
	for i, r := range rows {
		out.Plans[i] = reviewPlanToProto(dbgen.GetReviewPlanRow{
			ID: r.ID, CityID: r.CityID, Status: r.Status, ClosedReason: r.ClosedReason, Title: r.Title, Description: r.Description,
			AreaID: r.AreaID, AreaName: r.AreaName, ActivityID: r.ActivityID, FollowUpID: r.FollowUpID,
			PrimaryLanguage: r.PrimaryLanguage, SecondaryLanguage: r.SecondaryLanguage, Walk: r.Walk,
			Audience: r.Audience, Score: r.Score, Signups: r.Signups, WindowStart: r.WindowStart, WindowDays: r.WindowDays,
			SignupDeadline: r.SignupDeadline, CreatedAt: r.CreatedAt, CreatedBy: r.CreatedBy, ReviewedBy: r.ReviewedBy, ReviewedAt: r.ReviewedAt,
		}, names)
	}
	return connect.NewResponse(out), nil
}

func (h *ReviewHandler) GetReviewPlan(ctx context.Context, req *connect.Request[isshov1.GetReviewPlanRequest]) (*connect.Response[isshov1.GetReviewPlanResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := parsePlanID(req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	q := dbgen.New(h.Svc.DB)
	r, err := q.GetReviewPlan(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such plan"))
	}
	if err != nil {
		return nil, err
	}
	names, err := englishActivityNames(ctx, q, r.CityID)
	if err != nil {
		return nil, err
	}
	// The stops as people would see them, in English.
	caller := Caller{City: r.CityID, Primary: i18n.Authoring}
	var steps []Step
	out := &isshov1.GetReviewPlanResponse{
		VenuePlaceId: deref(r.VenuePlaceID), FollowUpVenuePlaceId: deref(r.FollowUpVenuePlaceID), UpcomingEvents: r.UpcomingEvents,
	}
	if r.Walk {
		steps, err = h.Svc.walkSteps(ctx, q, id, caller)
		if err != nil {
			return nil, err
		}
		stops, err := LoadWalk(ctx, q, id)
		if err != nil {
			return nil, err
		}
		out.Stops = reviewStops(stops)
	} else {
		plan := matching.Plan{
			Area: matching.AreaID(r.AreaID), Activity: matching.ActivityID(r.ActivityID), FollowUp: matching.ActivityID(deref(r.FollowUpID)),
			Primary: r.PrimaryLanguage, Secondary: deref(r.SecondaryLanguage),
		}
		steps = h.Svc.steps(ctx, plan, deref(r.VenuePlaceID), deref(r.FollowUpVenuePlaceID), deref(r.MeetingPlaceID),
			routeFrom(r.RoutePolyline, r.RouteSeconds), caller)
	}
	out.Plan, out.Steps = reviewPlanToProto(r, names), stepsToProto(steps)
	return connect.NewResponse(out), nil
}

func (h *ReviewHandler) ApprovePlan(ctx context.Context, req *connect.Request[isshov1.ApprovePlanRequest]) (*connect.Response[isshov1.ApprovePlanResponse], error) {
	c, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePlanID(req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	q := dbgen.New(h.Svc.DB)
	p, err := q.GetReviewPlan(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such plan"))
	}
	if err != nil {
		return nil, err
	}
	loc, err := location(ctx, q, p.CityID)
	if err != nil {
		return nil, err
	}
	// A draft may have waited days: its round starts now, as if just
	// suggested.
	now := h.now()
	w := h.Timing.Round(now, loc)
	n, err := q.ApproveDraft(ctx, dbgen.ApproveDraftParams{
		ID: id, WindowStart: date(w.Start), WindowDays: int16(w.Days),
		SignupDeadline: pgtype.Timestamptz{Time: w.Deadline, Valid: true},
		ReviewedBy:     pgtype.UUID{Bytes: c.UserID, Valid: true}, At: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("not a draft"))
	}
	return connect.NewResponse(&isshov1.ApprovePlanResponse{}), nil
}

func (h *ReviewHandler) RejectPlan(ctx context.Context, req *connect.Request[isshov1.RejectPlanRequest]) (*connect.Response[isshov1.RejectPlanResponse], error) {
	c, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePlanID(req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	n, err := dbgen.New(h.Svc.DB).RejectDraft(ctx, dbgen.RejectDraftParams{
		ID: id, ReviewedBy: pgtype.UUID{Bytes: c.UserID, Valid: true}, At: pgtype.Timestamptz{Time: h.now(), Valid: true},
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("not a draft"))
	}
	return connect.NewResponse(&isshov1.RejectPlanResponse{}), nil
}

func (h *ReviewHandler) EditPlan(ctx context.Context, req *connect.Request[isshov1.EditPlanRequest]) (*connect.Response[isshov1.EditPlanResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := parsePlanID(req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	title, summary, err := planText(req.Msg.Title, req.Msg.Summary)
	if err != nil {
		return nil, err
	}
	n, err := dbgen.New(h.Svc.DB).EditPlanText(ctx, dbgen.EditPlanTextParams{ID: id, Title: title, Description: summary})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no draft or open plan with that id"))
	}
	return connect.NewResponse(&isshov1.EditPlanResponse{}), nil
}

func (h *ReviewHandler) WithdrawPlan(ctx context.Context, req *connect.Request[isshov1.WithdrawPlanRequest]) (*connect.Response[isshov1.WithdrawPlanResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := parsePlanID(req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	n, err := dbgen.New(h.Svc.DB).WithdrawPlan(ctx, dbgen.WithdrawPlanParams{ID: id, At: pgtype.Timestamptz{Time: h.now(), Valid: true}})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("not an open plan"))
	}
	return connect.NewResponse(&isshov1.WithdrawPlanResponse{}), nil
}

func (h *ReviewHandler) ListVenueOptions(ctx context.Context, req *connect.Request[isshov1.ListVenueOptionsRequest]) (*connect.Response[isshov1.ListVenueOptionsResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := dbgen.New(h.Svc.DB)
	_, act, err := cityActivity(ctx, q, req.Msg.AreaId, req.Msg.ActivityId)
	if err != nil {
		return nil, err
	}
	rows, err := venueOptions(ctx, q, req.Msg.AreaId, act, venueOptionRows)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListVenueOptionsResponse{Venues: make([]*isshov1.VenueOption, len(rows))}
	var wg sync.WaitGroup
	for i, r := range rows {
		out.Venues[i] = &isshov1.VenueOption{PlaceId: r.GooglePlaceID, CategoryId: r.CategoryID, Status: r.Status}
		if h.Svc.Places == nil {
			continue
		}
		wg.Go(func() {
			// Names are fetched live (Maps terms); a failure leaves it blank.
			if d, err := h.Svc.Places.Details(ctx, r.GooglePlaceID, i18n.Authoring); err == nil {
				out.Venues[i].Name, out.Venues[i].Address, out.Venues[i].MapsUrl = d.Name, d.Address, d.MapsURL
			}
		})
	}
	wg.Wait()
	return connect.NewResponse(out), nil
}

func (h *ReviewHandler) CreatePlan(ctx context.Context, req *connect.Request[isshov1.CreatePlanRequest]) (*connect.Response[isshov1.CreatePlanResponse], error) {
	c, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	q := dbgen.New(h.Svc.DB)
	city, act, err := cityActivity(ctx, q, m.AreaId, m.ActivityId)
	if err != nil {
		return nil, err
	}
	if city.ID != m.CityId {
		return nil, reviewInvalid("area_id: not in that city")
	}
	var followUp *matching.Activity
	if m.FollowUpId != "" {
		if followUp = city.activity(matching.ActivityID(m.FollowUpId)); followUp == nil || followUp.ID == act.ID {
			return nil, reviewInvalid("follow_up_id: not another activity in the city")
		}
	}
	if !i18n.Valid(m.PrimaryLanguage) || (m.SecondaryLanguage != "" && (!i18n.Valid(m.SecondaryLanguage) || m.SecondaryLanguage == m.PrimaryLanguage)) {
		return nil, reviewInvalid("languages: ISO 639-1 codes, the second different from the first")
	}
	title, summary, err := planText(m.Title, m.Summary)
	if err != nil {
		return nil, err
	}
	venue, err := chooseVenue(ctx, q, m.AreaId, act, m.VenuePlaceId)
	if err != nil {
		return nil, err
	}
	var followUpVenue *string
	if followUp != nil {
		if followUpVenue, err = chooseVenue(ctx, q, m.AreaId, followUp, m.FollowUpVenuePlaceId); err != nil {
			return nil, err
		}
	} else if m.FollowUpVenuePlaceId != "" {
		return nil, reviewInvalid("follow_up_venue_place_id: no follow-up activity")
	}
	now := h.now()
	w := h.Timing.Round(now, city.Location)
	id, err := q.InsertPlan(ctx, dbgen.InsertPlanParams{
		Status:               PlanOpen,
		CreatedBy:            pgtype.UUID{Bytes: c.UserID, Valid: true},
		CityID:               city.ID,
		AreaID:               m.AreaId,
		ActivityID:           m.ActivityId,
		FollowUpID:           ptr(m.FollowUpId),
		PrimaryLanguage:      m.PrimaryLanguage,
		SecondaryLanguage:    ptr(m.SecondaryLanguage),
		VenuePlaceID:         venue,
		FollowUpVenuePlaceID: followUpVenue,
		WindowStart:          date(w.Start),
		WindowDays:           int16(w.Days),
		SignupDeadline:       pgtype.Timestamptz{Time: w.Deadline, Valid: true},
		Title:                title,
		Description:          summary,
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.CreatePlanResponse{PlanId: id.String()}), nil
}

// cityActivity finds an area's city and an activity enabled there.
func cityActivity(ctx context.Context, q *dbgen.Queries, areaID, activityID string) (*City, *matching.Activity, error) {
	cityID, err := q.GetAreaCity(ctx, areaID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, reviewInvalid("area_id: no such area")
	}
	if err != nil {
		return nil, nil, err
	}
	// Any month: an admin may plan a seasonal activity ahead.
	city, err := loadPlaces(ctx, q, cityID, 0)
	if err != nil {
		return nil, nil, err
	}
	act := city.activity(matching.ActivityID(activityID))
	if act == nil {
		return nil, nil, reviewInvalid("activity_id: not an activity enabled in the city")
	}
	return city, act, nil
}

func (c *City) activity(id matching.ActivityID) *matching.Activity {
	for i := range c.Activities {
		if c.Activities[i].ID == id {
			return &c.Activities[i]
		}
	}
	return nil
}

func venueOptions(ctx context.Context, q *dbgen.Queries, areaID string, act *matching.Activity, limit int32) ([]dbgen.ListVenueOptionsRow, error) {
	if len(act.VenueCategories) == 0 {
		return nil, nil
	}
	cats := make([]string, len(act.VenueCategories))
	for i, c := range act.VenueCategories {
		cats[i] = string(c)
	}
	return q.ListVenueOptions(ctx, dbgen.ListVenueOptionsParams{AreaID: areaID, Categories: cats, MaxRows: limit})
}

// chooseVenue checks an admin's choice against the venues found for the
// activity in the area, or picks one as the matcher would ("" = pick).
func chooseVenue(ctx context.Context, q *dbgen.Queries, areaID string, act *matching.Activity, placeID string) (*string, error) {
	if placeID == "" {
		return pickVenue(ctx, q, matching.AreaID(areaID), act)
	}
	opts, err := venueOptions(ctx, q, areaID, act, 10000)
	if err != nil {
		return nil, err
	}
	for _, o := range opts {
		if o.GooglePlaceID == placeID {
			return &placeID, nil
		}
	}
	return nil, reviewInvalid("venue: not a venue found for " + string(act.ID) + " in that area")
}

// planText checks an admin's English title and summary and stores them as
// localised text ({"en": "..."}); "" = none.
func planText(title, summary string) (t, s []byte, err error) {
	title, summary = strings.TrimSpace(title), strings.TrimSpace(summary)
	if utf8.RuneCountInString(title) > MaxTitle {
		return nil, nil, reviewInvalid("title: at most 80 characters")
	}
	if utf8.RuneCountInString(summary) > MaxSummary {
		return nil, nil, reviewInvalid("summary: at most 500 characters")
	}
	if title != "" {
		t = english(title)
	}
	if summary != "" {
		s = english(summary)
	}
	return t, s, nil
}

// englishActivityNames maps a city's activity ids to their English names.
func englishActivityNames(ctx context.Context, q *dbgen.Queries, cityID string) (map[string]string, error) {
	rows, err := q.ListCityActivities(ctx, cityID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.ID] = i18n.Pick(r.Name, i18n.Authoring)
	}
	return out, nil
}

func reviewPlanToProto(r dbgen.GetReviewPlanRow, names map[string]string) *isshov1.ReviewPlan {
	out := &isshov1.ReviewPlan{
		Id:                r.ID.String(),
		CityId:            r.CityID,
		ClosedReason:      r.ClosedReason,
		AreaId:            r.AreaID,
		AreaName:          r.AreaName,
		ActivityId:        r.ActivityID,
		FollowUpId:        deref(r.FollowUpID),
		PrimaryLanguage:   r.PrimaryLanguage,
		SecondaryLanguage: deref(r.SecondaryLanguage),
		Walk:              r.Walk,
		Audience:          r.Audience,
		Score:             r.Score,
		Signups:           r.Signups,
		WindowDays:        int32(r.WindowDays),
		SignupDeadline:    timestamppb.New(r.SignupDeadline.Time),
		CreatedAt:         timestamppb.New(r.CreatedAt.Time),
	}
	for k, v := range reviewStatuses {
		if v == r.Status {
			out.Status = k
		}
	}
	if r.Title != nil {
		out.StoredTitle = i18n.Pick(r.Title, i18n.Authoring)
		out.Title = out.StoredTitle
	} else {
		out.Title = Title(names[r.ActivityID], names[deref(r.FollowUpID)])
	}
	if r.Description != nil {
		out.Summary = i18n.Pick(r.Description, i18n.Authoring)
	}
	if r.WindowStart.Valid {
		out.WindowStart = r.WindowStart.Time.Format(time.DateOnly)
	}
	if r.CreatedBy.Valid {
		out.CreatedBy = uuid.UUID(r.CreatedBy.Bytes).String()
	}
	if r.ReviewedBy.Valid {
		out.ReviewedBy = uuid.UUID(r.ReviewedBy.Bytes).String()
	}
	if r.ReviewedAt.Valid {
		out.ReviewedAt = timestamppb.New(r.ReviewedAt.Time)
	}
	return out
}
