package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// Editing a plan's stops by hand (M8): a walk's stops replaced whole, or
// another plan's venues swapped. Draft and open plans only; an event
// already scheduled shares the plan's stops, so its people see the change.

// Limits on a walk an admin edits.
const (
	MinWalkPlaces = 3 // where to meet, one stop, where to finish
	MaxWalkPlaces = 10
	MaxStopNote   = 300
	maxPlaceQuery = 100
)

// walkRules are the composer's: an edited walk is held to the same
// distance from the area, and warned about the rest.
var walkRules = walks.DefaultRules()

// reviewPlan loads a plan for editing; NOT_FOUND if there's none.
func reviewPlan(ctx context.Context, q *dbgen.Queries, planID string) (uuid.UUID, dbgen.GetReviewPlanRow, error) {
	id, err := parsePlanID(planID)
	if err != nil {
		return id, dbgen.GetReviewPlanRow{}, err
	}
	r, err := q.GetReviewPlan(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, r, connect.NewError(connect.CodeNotFound, errors.New("no such plan"))
	}
	return id, r, err
}

// lockEditable locks a plan in a transaction and checks it can still be
// edited.
func lockEditable(ctx context.Context, q *dbgen.Queries, id uuid.UUID, walk bool) error {
	p, err := q.LockPlanForEdit(ctx, id)
	if err != nil {
		return err
	}
	if p.Status != PlanDraft && p.Status != PlanOpen {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("only draft and open plans can be edited"))
	}
	if p.Walk != walk {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the plan changed since: reload it"))
	}
	return nil
}

func (h *ReviewHandler) FindPlanPlaces(ctx context.Context, req *connect.Request[isshov1.FindPlanPlacesRequest]) (*connect.Response[isshov1.FindPlanPlacesResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(req.Msg.Query)
	if query == "" || utf8.RuneCountInString(query) > maxPlaceQuery {
		return nil, errs.Invalid("query: 1 to 100 characters")
	}
	if h.Places == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("searching needs GOOGLE_MAPS_API_KEY"))
	}
	_, r, err := reviewPlan(ctx, dbgen.New(h.Svc.DB), req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	centre := maps.LatLng{Lat: r.AreaLat, Lng: r.AreaLng}
	found, err := h.Places.Search(ctx, maps.SearchRequest{Query: query, Near: centre, RadiusM: walkRules.MaxFromCenterM, Language: i18n.Authoring})
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("Google search failed: "+err.Error()))
	}
	out := &isshov1.FindPlanPlacesResponse{}
	for _, f := range found {
		out.Places = append(out.Places, &isshov1.FoundPlace{
			PlaceId: f.ID, Name: f.Details.Name, Address: f.Details.Address, MapsUrl: f.Details.MapsURL,
			PrimaryType: f.PrimaryType, Meters: int32(maps.DistanceM(centre, f.Location)), Closed: f.Closed,
		})
	}
	return connect.NewResponse(out), nil
}

func (h *ReviewHandler) SetPlanStops(ctx context.Context, req *connect.Request[isshov1.SetPlanStopsRequest]) (*connect.Response[isshov1.SetPlanStopsResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	in := req.Msg.Stops
	if len(in) < MinWalkPlaces || len(in) > MaxWalkPlaces {
		return nil, errs.Invalid("stops: %d to %d places, where to meet first and where to finish last", MinWalkPlaces, MaxWalkPlaces)
	}
	stops := make([]WalkStop, len(in))
	seen := map[string]bool{}
	for i, s := range in {
		n := i + 1
		if s.PlaceId == "" || strings.ContainsAny(s.PlaceId, "/?#") {
			return nil, errs.Invalid("stop %d: a Google place id", n)
		}
		if seen[s.PlaceId] {
			return nil, errs.Invalid("stop %d: the same place as an earlier stop", n)
		}
		seen[s.PlaceId] = true
		if s.Minutes < 0 || s.Minutes > 120 {
			return nil, errs.Invalid("stop %d: 0 to 120 minutes there", n)
		}
		note := strings.TrimSpace(s.Note)
		if utf8.RuneCountInString(note) > MaxStopNote {
			return nil, errs.Invalid("stop %d: a note of at most 300 characters", n)
		}
		// The role follows the position, as the composer saves it.
		role := walks.Stop
		switch i {
		case 0:
			role = walks.Meet
		case len(in) - 1:
			role = walks.Finish
		}
		stops[i] = WalkStop{Role: role, PlaceID: s.PlaceId, Minutes: int(s.Minutes), Note: english(note)}
	}
	if h.Places == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("editing stops needs GOOGLE_MAPS_API_KEY"))
	}
	q := dbgen.New(h.Svc.DB)
	id, r, err := reviewPlan(ctx, q, req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	if !r.Walk {
		return nil, errs.Invalid("not a walk: change its venues instead")
	}
	if err := h.checkPlaces(ctx, stops, maps.LatLng{Lat: r.AreaLat, Lng: r.AreaLng}); err != nil {
		return nil, err
	}

	// The route in the admin's order (not optimised: they chose it).
	ids := make([]string, len(stops))
	for i, s := range stops {
		ids[i] = s.PlaceID
	}
	var route *maps.WalkRoute
	out := &isshov1.SetPlanStopsResponse{}
	if h.Routes != nil {
		rt, err := h.Routes.WalkThrough(ctx, ids, false)
		if err == nil && len(rt.Legs) == len(stops)-1 {
			route = &rt
		} else {
			// Saved without a route; the worker works it out on its next pass.
			out.Warnings = append(out.Warnings, "Google couldn't work out the walking route just now: walking times will follow")
		}
	}
	if route != nil {
		for i := 1; i < len(stops); i++ {
			stops[i].Walk = route.Legs[i-1].Duration
		}
		out.Warnings = append(out.Warnings, walkWarnings(stops, route.Duration)...)
		out.Minutes = int32(walkMinutesTotal(stops, route.Duration))
	}

	err = pgx.BeginFunc(ctx, h.Svc.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := lockEditable(ctx, q, id, true); err != nil {
			return err
		}
		if err := q.DeletePlanStops(ctx, id); err != nil {
			return err
		}
		p := dbgen.InsertPlanStopsParams{PlanID: id}
		for i, st := range stops {
			p.Positions = append(p.Positions, int16(i))
			p.Roles = append(p.Roles, string(st.Role))
			p.PlaceIds = append(p.PlaceIds, st.PlaceID)
			p.Minutes = append(p.Minutes, int16(st.Minutes))
			p.Notes = append(p.Notes, st.Note)
			secs := int32(-1) // none: the first stop, or no route yet
			if i > 0 && route != nil {
				secs = int32(st.Walk.Seconds())
			}
			p.WalkSeconds = append(p.WalkSeconds, secs)
		}
		if err := q.InsertPlanStops(ctx, p); err != nil {
			return err
		}
		if route == nil {
			return q.ClearPlanRoute(ctx, id)
		}
		return q.SetPlanRoute(ctx, dbgen.SetPlanRouteParams{
			ID: id, Polyline: &route.Polyline, Seconds: ptrInt32(int32(route.Duration.Seconds())),
			At: pgtype.Timestamptz{Time: h.now(), Valid: true},
		})
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// checkPlaces checks each stop's place with Google, a few at a time: it
// exists, isn't closed, and is within the composer's distance of the
// area's centre.
func (h *ReviewHandler) checkPlaces(ctx context.Context, stops []WalkStop, centre maps.LatLng) error {
	problems := make([]string, len(stops))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, s := range stops {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			p, err := h.Places.Locate(ctx, s.PlaceID)
			switch {
			case err != nil:
				problems[i] = "Google doesn't know the place"
			case p.Closed:
				problems[i] = p.Details.Name + " is closed"
			case maps.DistanceM(centre, p.Location) > walkRules.MaxFromCenterM:
				problems[i] = fmt.Sprintf("%s is %.1f km from the area's centre (at most %.0f)", p.Details.Name, maps.DistanceM(centre, p.Location)/1000, walkRules.MaxFromCenterM/1000)
			}
		})
	}
	wg.Wait()
	for i, p := range problems {
		if p != "" {
			return errs.Invalid("stop %d: %s", i+1, p)
		}
	}
	return nil
}

// walkMinutesTotal is the whole outing: time at the stops plus walking.
func walkMinutesTotal(stops []WalkStop, walking time.Duration) int {
	n := int(walking.Round(time.Minute) / time.Minute)
	for _, s := range stops {
		n += s.Minutes
	}
	return n
}

// walkWarnings says which of the composer's rules an edited walk breaks.
func walkWarnings(stops []WalkStop, walking time.Duration) []string {
	var out []string
	for i := 1; i < len(stops); i++ {
		if stops[i].Walk > walkRules.MaxLeg {
			out = append(out, fmt.Sprintf("The walk from stop %d to stop %d takes %d minutes (the composer allows %d).",
				i, i+1, int(stops[i].Walk.Round(time.Minute)/time.Minute), int(walkRules.MaxLeg/time.Minute)))
		}
	}
	if walking > walkRules.MaxWalking {
		out = append(out, fmt.Sprintf("%d minutes of walking in all (the composer allows %d).",
			int(walking.Round(time.Minute)/time.Minute), int(walkRules.MaxWalking/time.Minute)))
	}
	switch m := walkMinutesTotal(stops, walking); {
	case m > walkRules.MaxMinutes:
		out = append(out, fmt.Sprintf("The outing takes %d minutes, over %d.", m, walkRules.MaxMinutes))
	case m < walkRules.MinMinutes:
		out = append(out, fmt.Sprintf("The outing takes %d minutes, under %d.", m, walkRules.MinMinutes))
	}
	return out
}

func (h *ReviewHandler) SetPlanVenue(ctx context.Context, req *connect.Request[isshov1.SetPlanVenueRequest]) (*connect.Response[isshov1.SetPlanVenueResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	m := req.Msg
	q := dbgen.New(h.Svc.DB)
	id, r, err := reviewPlan(ctx, q, m.PlanId)
	if err != nil {
		return nil, err
	}
	if r.Walk {
		return nil, errs.Invalid("a walk: edit its stops instead")
	}
	activity := r.ActivityID
	if m.FollowUp {
		if r.FollowUpID == nil {
			return nil, errs.Invalid("follow_up: the plan has nothing after")
		}
		activity = *r.FollowUpID
	}
	_, act, err := cityActivity(ctx, q, r.AreaID, activity)
	if err != nil {
		return nil, err
	}
	chosen, err := chooseVenue(ctx, q, r.AreaID, act, m.PlaceId)
	if err != nil {
		return nil, err
	}
	venue, followUp := r.VenuePlaceID, r.FollowUpVenuePlaceID
	if m.FollowUp {
		followUp = chosen
	} else {
		venue = chosen
	}

	// The walk between the two, as the worker would work it out: from the
	// venue (or the meeting place) to the follow-up's venue.
	var route *maps.WalkRoute
	from := venue
	if from == nil {
		from = r.MeetingPlaceID
	}
	if from != nil && followUp != nil && h.Routes != nil {
		if rt, err := h.Routes.WalkThrough(ctx, []string{*from, *followUp}, false); err == nil {
			route = &rt
		}
	}
	err = pgx.BeginFunc(ctx, h.Svc.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := lockEditable(ctx, q, id, false); err != nil {
			return err
		}
		if err := q.SetPlanVenues(ctx, dbgen.SetPlanVenuesParams{ID: id, VenuePlaceID: venue, FollowUpVenuePlaceID: followUp}); err != nil {
			return err
		}
		if route == nil {
			// None needed, or the worker works it out on its next pass.
			return q.ClearPlanRoute(ctx, id)
		}
		return q.SetPlanRoute(ctx, dbgen.SetPlanRouteParams{
			ID: id, Polyline: &route.Polyline, Seconds: ptrInt32(int32(route.Duration.Seconds())),
			At: pgtype.Timestamptz{Time: h.now(), Valid: true},
		})
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.SetPlanVenueResponse{}), nil
}

// reviewStops are a walk's stops as stored, with their English notes.
func reviewStops(stops []WalkStop) []*isshov1.ReviewStop {
	out := make([]*isshov1.ReviewStop, len(stops))
	for i, s := range stops {
		var note map[string]string
		_ = json.Unmarshal(s.Note, &note)
		out[i] = &isshov1.ReviewStop{PlaceId: s.PlaceID, Role: string(s.Role), Minutes: int32(s.Minutes), Note: note[i18n.Authoring]}
	}
	return out
}
