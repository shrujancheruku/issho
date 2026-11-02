package curation

import (
	"context"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

func TestNonAdminRefused(t *testing.T) {
	// The admin check runs before the service is touched, so none is needed.
	h := &Handler{}
	ctx := auth.WithCaller(context.Background(), auth.Caller{UserID: uuid.New()})
	if _, err := h.ListVenues(ctx, connect.NewRequest(&isshov1.ListVenuesRequest{})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("ListVenues as a user: %v", err)
	}
	if _, err := h.AddVenue(context.Background(), connect.NewRequest(&isshov1.AddVenueRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("AddVenue without a caller: %v", err)
	}
}

// TestIntegrationHandler covers the mapping between the protos and the
// service (the rules have their tests in service_test.go): every RPC once
// through Connect's types, and the enum checks the handler owns. Skipped
// unless TEST_DATABASE_URL is set.
func TestIntegrationHandler(t *testing.T) {
	f := newFixture(t)
	h := &Handler{Svc: f.svc}
	adminCtx := auth.WithCaller(context.Background(), auth.Caller{UserID: f.adminID, Admin: true})

	cities, err := h.ListVenueCities(adminCtx, connect.NewRequest(&isshov1.ListVenueCitiesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var city *isshov1.VenueCity
	for _, c := range cities.Msg.Cities {
		if c.Id == f.city {
			city = c
		}
	}
	if city == nil || city.Candidates != 3 || len(city.Areas) != 2 || city.Areas[0].Name != "Alpha" || len(cities.Msg.Categories) < 2 {
		t.Fatalf("city = %v", city)
	}

	list := func(status isshov1.VenueStatus) *isshov1.ListVenuesResponse {
		t.Helper()
		res, err := h.ListVenues(adminCtx, connect.NewRequest(&isshov1.ListVenuesRequest{CityId: f.city, Status: status}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg
	}
	if _, err := h.ListVenues(adminCtx, connect.NewRequest(&isshov1.ListVenuesRequest{CityId: f.city})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no status: %v", err)
	}
	cands := list(isshov1.VenueStatus_VENUE_STATUS_CANDIDATE)
	if v := cands.Venues[0]; cands.Total != 3 || v.PlaceId != "zz-p1" || v.Name != "スナック ゆき" || v.Flag != "snack bar?" || v.AreaName != "Alpha" ||
		v.Status != isshov1.VenueStatus_VENUE_STATUS_CANDIDATE || v.ReviewedAt != nil || v.MapsUrl == "" {
		t.Errorf("first candidate = %v (total %d)", cands.Venues[0], cands.Total)
	}

	set, err := h.SetVenueStatus(adminCtx, connect.NewRequest(&isshov1.SetVenueStatusRequest{
		CityId: f.city, PlaceIds: []string{"zz-p1", "elsewhere"}, Status: isshov1.VenueStatus_VENUE_STATUS_REJECTED,
	}))
	if err != nil || set.Msg.Changed != 1 {
		t.Fatalf("set = %v, %v", set, err)
	}
	if rej := list(isshov1.VenueStatus_VENUE_STATUS_REJECTED); len(rej.Venues) != 1 || rej.Venues[0].ReviewedAt == nil {
		t.Errorf("rejected = %v", rej)
	}
	if _, err := h.SetVenueStatus(adminCtx, connect.NewRequest(&isshov1.SetVenueStatusRequest{CityId: f.city, PlaceIds: []string{"zz-p2"}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no status: %v", err)
	}

	if _, err := h.SetApprovedVenuesOnly(adminCtx, connect.NewRequest(&isshov1.SetApprovedVenuesOnlyRequest{CityId: "nowhere"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown city: %v", err)
	}

	res, err := h.SearchVenues(adminCtx, connect.NewRequest(&isshov1.SearchVenuesRequest{CityId: f.city, Query: "snack"}))
	if err != nil || res.Msg.Searches != 2 {
		t.Fatalf("search = %v, %v", res, err)
	}
	found := map[string]*isshov1.FoundVenue{}
	for _, v := range res.Msg.Venues {
		found[v.PlaceId] = v
	}
	if v := found["zz-p1"]; !v.Known || v.Status != isshov1.VenueStatus_VENUE_STATUS_REJECTED || v.Flag != "snack bar?" {
		t.Errorf("known p1 = %v", v)
	}
	if v := found["zz-p4"]; v.Known || !slices.Contains(v.AddableCategoryIds, f.bar) || v.Flag != "snack bar?" || v.AreaId != f.areaA {
		t.Errorf("new p4 = %v", v)
	}
	if v := found["zz-p5"]; !v.Closed {
		t.Errorf("closed p5 = %v", v)
	}

	add, err := h.AddVenue(adminCtx, connect.NewRequest(&isshov1.AddVenueRequest{CityId: f.city, PlaceId: "zz-p4", CategoryId: f.bar}))
	if err != nil || add.Msg.AreaId != f.areaA {
		t.Fatalf("add = %v, %v", add, err)
	}
	if v := list(isshov1.VenueStatus_VENUE_STATUS_APPROVED).Venues; len(v) != 1 || v[0].PlaceId != "zz-p4" || !v[0].Added {
		t.Errorf("approved = %v", v)
	}
}
