package rpc

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

type planService struct {
	cities *cities.Service
	plans  *plans.Service
}

// language is the display language of a public call, which has no account to
// take it from: empty is English, and any language the cities don't show is
// refused, as it is for an account (people.Languages).
func language(requested string) (string, error) {
	if requested == "" {
		return people.DefaultLanguage, nil
	}
	if !slices.Contains(people.Languages, requested) {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("display_language must be en or vi"))
	}
	return requested, nil
}

// ListPlans lists a city's plans. It is public and returns no person's data:
// a plan's fields are the same for everyone.
func (s planService) ListPlans(ctx context.Context, req *connect.Request[isshov1.ListPlansRequest]) (*connect.Response[isshov1.ListPlansResponse], error) {
	lang, err := language(req.Msg.GetDisplayLanguage())
	if err != nil {
		return nil, err
	}
	city, err := s.cities.Get(ctx, req.Msg.GetCity())
	if errors.Is(err, cities.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such city"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the city"))
	}
	// A closed city's plans are kept but not shown (docs/spec/cities.md).
	if city.State == cities.Closed {
		return connect.NewResponse(&isshov1.ListPlansResponse{}), nil
	}
	listed, err := s.plans.List(ctx, city.Slug, lang)
	if err != nil {
		// The error may quote the database; the caller gets none of it.
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the plans"))
	}
	out := make([]*isshov1.Plan, len(listed))
	for i, p := range listed {
		out[i] = planMessage(p, city)
	}
	return connect.NewResponse(&isshov1.ListPlansResponse{Plans: out}), nil
}

// GetPlan returns one plan. A plan that doesn't exist is NOT_FOUND, and so is
// one in a closed city: the same error, so a caller can't tell the two apart.
func (s planService) GetPlan(ctx context.Context, req *connect.Request[isshov1.GetPlanRequest]) (*connect.Response[isshov1.GetPlanResponse], error) {
	lang, err := language(req.Msg.GetDisplayLanguage())
	if err != nil {
		return nil, err
	}
	p, err := s.plans.Get(ctx, req.Msg.GetId(), lang)
	if errors.Is(err, plans.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such plan"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the plan"))
	}
	city, err := s.cities.Get(ctx, p.City)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the plan's city"))
	}
	if city.State == cities.Closed {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such plan"))
	}
	return connect.NewResponse(&isshov1.GetPlanResponse{Plan: planMessage(p, city)}), nil
}

func planMessage(p plans.Plan, city cities.City) *isshov1.Plan {
	m := &isshov1.Plan{
		Id: p.ID, City: p.City, Language: p.Language,
		Title: p.Title, Description: p.Description,
		Price: int32(p.Price), Currency: city.Currency,
		MeetingAddress: p.MeetingAddress, MeetingPoint: p.MeetingPoint,
		WhatToBring: p.WhatToBring, PartsOfDay: p.PartsOfDay,
		StartsAt: p.StartsAt, EndsAt: p.EndsAt, TimeZone: city.TimeZone,
	}
	for _, st := range p.Stops {
		stop := &isshov1.Stop{Position: int32(st.Position), StayMinutes: int32(st.StayMinutes), Name: st.Name, Note: st.Note}
		if st.Leg != nil {
			stop.Leg = &isshov1.Leg{Mode: st.Leg.Mode, Minutes: int32(st.Leg.Minutes)}
		}
		m.Stops = append(m.Stops, stop)
	}
	return m
}
