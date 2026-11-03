// Package curation serves CurationService: venue curation in the admin SPA
// (M8). Admins approve or reject the places discovery found, look places up
// by name, add ones discovery missed, and can make a city use only
// approved venues. Only place ids and our own data are stored; names and
// addresses are fetched live from Google (Maps terms). The rules live in
// Service; the Handler checks the caller and maps to and from the protos.
package curation

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// Handler serves CurationService. cmd/api limits the service to admins
// (auth.AdminOnly); each call checks again (auth.RequireAdmin) so a wiring mistake fails closed.
type Handler struct {
	Svc *Service
}

var statuses = map[isshov1.VenueStatus]string{
	isshov1.VenueStatus_VENUE_STATUS_CANDIDATE: "candidate",
	isshov1.VenueStatus_VENUE_STATUS_APPROVED:  "approved",
	isshov1.VenueStatus_VENUE_STATUS_REJECTED:  "rejected",
}

func statusOf(s string) isshov1.VenueStatus {
	for k, v := range statuses {
		if v == s {
			return k
		}
	}
	return isshov1.VenueStatus_VENUE_STATUS_UNSPECIFIED
}

// VenueStatusProto converts a venue's status for ReviewService; unlike
// statusOf an unknown value is an error, not UNSPECIFIED.
func VenueStatusProto(s string) (isshov1.VenueStatus, error) {
	if v := statusOf(s); v != isshov1.VenueStatus_VENUE_STATUS_UNSPECIFIED {
		return v, nil
	}
	return 0, fmt.Errorf("unknown venue status %q", s)
}

func (h *Handler) ListVenueCities(ctx context.Context, _ *connect.Request[isshov1.ListVenueCitiesRequest]) (*connect.Response[isshov1.ListVenueCitiesResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	cities, areas, cats, err := h.Svc.Cities(ctx)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListVenueCitiesResponse{}
	for _, c := range cities {
		vc := &isshov1.VenueCity{
			Id: c.ID, Name: c.Name, ApprovedVenuesOnly: c.ApprovedVenuesOnly,
			Candidates: c.Candidates, Approved: c.Approved, Rejected: c.Rejected,
		}
		for _, a := range areas {
			if a.CityID == c.ID {
				vc.Areas = append(vc.Areas, &isshov1.VenueArea{Id: a.ID, Name: a.Name})
			}
		}
		out.Cities = append(out.Cities, vc)
	}
	for _, c := range cats {
		out.Categories = append(out.Categories, &isshov1.VenueCategory{Id: c.ID, Name: c.Name})
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) SetApprovedVenuesOnly(ctx context.Context, req *connect.Request[isshov1.SetApprovedVenuesOnlyRequest]) (*connect.Response[isshov1.SetApprovedVenuesOnlyResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.Svc.SetApprovedOnly(ctx, req.Msg.CityId, req.Msg.ApprovedVenuesOnly); err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.SetApprovedVenuesOnlyResponse{}), nil
}

func (h *Handler) ListVenues(ctx context.Context, req *connect.Request[isshov1.ListVenuesRequest]) (*connect.Response[isshov1.ListVenuesResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	m := req.Msg
	status, ok := statuses[m.Status]
	if !ok {
		return nil, errs.Invalid("status: candidate, approved or rejected")
	}
	places, total, next, err := h.Svc.List(ctx, ListParams{
		CityID: m.CityId, AreaID: m.AreaId, CategoryID: m.CategoryId, Status: status,
		PageSize: m.PageSize, Token: m.PageToken,
	})
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListVenuesResponse{Total: total, NextPageToken: next, Venues: make([]*isshov1.Venue, len(places))}
	for i, p := range places {
		v := &isshov1.Venue{
			PlaceId: p.GooglePlaceID, AreaId: p.AreaID, AreaName: p.AreaName, CategoryIds: p.Categories,
			Status: statusOf(p.Status), Added: p.Added, Plans: p.Plans,
			Name: p.Name, Address: p.Address, MapsUrl: p.MapsURL, Flag: p.Flag,
		}
		if p.ReviewedAt.Valid {
			v.ReviewedAt = timestamppb.New(p.ReviewedAt.Time)
		}
		out.Venues[i] = v
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) SetVenueStatus(ctx context.Context, req *connect.Request[isshov1.SetVenueStatusRequest]) (*connect.Response[isshov1.SetVenueStatusResponse], error) {
	c, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	status, ok := statuses[m.Status]
	if !ok {
		return nil, errs.Invalid("status: candidate, approved or rejected")
	}
	n, err := h.Svc.SetStatus(ctx, m.CityId, m.PlaceIds, status, c.UserID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.SetVenueStatusResponse{Changed: int32(n)}), nil
}

func (h *Handler) SearchVenues(ctx context.Context, req *connect.Request[isshov1.SearchVenuesRequest]) (*connect.Response[isshov1.SearchVenuesResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	m := req.Msg
	res, err := h.Svc.Search(ctx, SearchParams{CityID: m.CityId, AreaID: m.AreaId, Query: m.Query})
	if err != nil {
		return nil, err
	}
	out := &isshov1.SearchVenuesResponse{Searches: int32(res.Searches)}
	for _, f := range res.Venues {
		out.Venues = append(out.Venues, &isshov1.FoundVenue{
			PlaceId: f.ID, Name: f.Details.Name, Address: f.Details.Address, MapsUrl: f.Details.MapsURL,
			AreaId: f.AreaID, AreaName: f.AreaName, Flag: Flag(f.Details.Name), Closed: f.Closed,
			Known: f.Known, Status: statusOf(f.Status), CategoryIds: f.CategoryIDs, AddableCategoryIds: f.Addable,
		})
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) AddVenue(ctx context.Context, req *connect.Request[isshov1.AddVenueRequest]) (*connect.Response[isshov1.AddVenueResponse], error) {
	c, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	areaID, err := h.Svc.Add(ctx, m.CityId, m.PlaceId, m.CategoryId, c.UserID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.AddVenueResponse{AreaId: areaID}), nil
}
