// Package curation serves CurationService: venue curation in the admin SPA
// (M8). Admins approve or reject the places discovery found, look places up
// by name, add ones discovery missed, and can make a city use only
// approved venues. Only place ids and our own data are stored; names and
// addresses are fetched live from Google (Maps terms).
package curation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

const (
	// PageSize is how many places ListVenues returns: each costs a Place
	// Details call for its name.
	PageSize = 50
	// MaxBatch is how many places SetVenueStatus takes at once.
	MaxBatch = 100
	// MaxQuery bounds a search's text.
	MaxQuery = 100
	// searchRadiusM is half the side of the box searched around an area's
	// centre: about the radius discovery searches most categories in.
	searchRadiusM = 1500
	// lookups bounds concurrent Google calls for one request.
	lookups = 8
)

// Handler serves CurationService. cmd/api limits the service to admins
// (auth.AdminOnly); admin() checks again so a wiring mistake fails closed.
type Handler struct {
	DB *pgxpool.Pool
	// Places is nil without GOOGLE_MAPS_API_KEY: names are then blank, and
	// searching and adding are refused.
	Places maps.PlaceSearch
	Now    func() time.Time // nil = time.Now
}

func (h *Handler) now() time.Time {
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

func invalid(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
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

// area is one of a city's areas, with its English name.
type area struct {
	geo.Area
	Name string
}

// cityAreas returns a city's areas; NOT_FOUND for an unknown city.
func cityAreas(ctx context.Context, q *dbgen.Queries, cityID string) ([]area, error) {
	rows, err := q.ListCurationAreas(ctx)
	if err != nil {
		return nil, err
	}
	var out []area
	for _, r := range rows {
		if r.CityID == cityID {
			out = append(out, area{Area: geo.Area{ID: r.ID, LatLng: maps.LatLng{Lat: r.Lat, Lng: r.Lng}}, Name: r.Name})
		}
	}
	if len(out) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such city"))
	}
	return out, nil
}

func nearest(areas []area, p maps.LatLng) area {
	plain := make([]geo.Area, len(areas))
	for i, a := range areas {
		plain[i] = a.Area
	}
	n := geo.Nearest(plain, p)
	for _, a := range areas {
		if a.ID == n.ID {
			return a
		}
	}
	return areas[0]
}

func (h *Handler) ListVenueCities(ctx context.Context, _ *connect.Request[isshov1.ListVenueCitiesRequest]) (*connect.Response[isshov1.ListVenueCitiesResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := dbgen.New(h.DB)
	cities, err := q.ListVenueCities(ctx)
	if err != nil {
		return nil, err
	}
	areas, err := q.ListCurationAreas(ctx)
	if err != nil {
		return nil, err
	}
	cats, err := q.ListAllVenueCategories(ctx)
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
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	n, err := dbgen.New(h.DB).SetApprovedVenuesOnly(ctx, dbgen.SetApprovedVenuesOnlyParams{ID: req.Msg.CityId, ApprovedVenuesOnly: req.Msg.ApprovedVenuesOnly})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such city"))
	}
	return connect.NewResponse(&isshov1.SetApprovedVenuesOnlyResponse{}), nil
}

func (h *Handler) ListVenues(ctx context.Context, req *connect.Request[isshov1.ListVenuesRequest]) (*connect.Response[isshov1.ListVenuesResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	m := req.Msg
	status, ok := statuses[m.Status]
	if !ok {
		return nil, invalid("status: candidate, approved or rejected")
	}
	if m.Offset < 0 {
		return nil, invalid("offset: not negative")
	}
	rows, err := dbgen.New(h.DB).ListCurationVenues(ctx, dbgen.ListCurationVenuesParams{
		CityID: m.CityId, AreaID: m.AreaId, CategoryID: m.CategoryId, Status: status,
		MaxRows: PageSize, Skip: m.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListVenuesResponse{Venues: make([]*isshov1.Venue, len(rows))}
	ids := make([]string, len(rows))
	for i, r := range rows {
		out.Total = r.Total
		ids[i] = r.GooglePlaceID
		v := &isshov1.Venue{
			PlaceId: r.GooglePlaceID, AreaId: r.AreaID, AreaName: r.AreaName, CategoryIds: r.Categories,
			Status: statusOf(r.Status), Added: r.Added, Plans: r.Plans,
		}
		if r.ReviewedAt.Valid {
			v.ReviewedAt = timestamppb.New(r.ReviewedAt.Time)
		}
		out.Venues[i] = v
	}
	// Names are fetched live (Maps terms); a failure leaves one blank.
	h.details(ctx, ids, func(i int, d maps.PlaceDetails) {
		v := out.Venues[i]
		v.Name, v.Address, v.MapsUrl, v.Flag = d.Name, d.Address, d.MapsURL, Flag(d.Name)
	})
	return connect.NewResponse(out), nil
}

// details fetches places' English details, a few at a time, calling set
// (from one goroutine at a time) for each that succeeded.
func (h *Handler) details(ctx context.Context, ids []string, set func(int, maps.PlaceDetails)) {
	if h.Places == nil {
		return
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, lookups)
	for i, id := range ids {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			if d, err := h.Places.Details(ctx, id, "en"); err == nil {
				mu.Lock()
				set(i, d)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
}

func (h *Handler) SetVenueStatus(ctx context.Context, req *connect.Request[isshov1.SetVenueStatusRequest]) (*connect.Response[isshov1.SetVenueStatusResponse], error) {
	c, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	status, ok := statuses[m.Status]
	if !ok {
		return nil, invalid("status: candidate, approved or rejected")
	}
	if len(m.PlaceIds) == 0 || len(m.PlaceIds) > MaxBatch {
		return nil, invalid("place_ids: 1 to 100")
	}
	if _, err := cityAreas(ctx, dbgen.New(h.DB), m.CityId); err != nil {
		return nil, err
	}
	var by pgtype.UUID
	var at pgtype.Timestamptz
	// Back to candidate clears the review: nobody has decided.
	if status != "candidate" {
		by, at = pgtype.UUID{Bytes: c.UserID, Valid: true}, pgtype.Timestamptz{Time: h.now(), Valid: true}
	}
	q := dbgen.New(h.DB)
	known, err := q.ListKnownVenues(ctx, dbgen.ListKnownVenuesParams{CityID: m.CityId, PlaceIds: m.PlaceIds})
	if err != nil {
		return nil, err
	}
	if _, err := q.SetVenueStatus(ctx, dbgen.SetVenueStatusParams{
		CityID: m.CityId, PlaceIds: m.PlaceIds, Status: status, ReviewedBy: by, At: at,
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.SetVenueStatusResponse{Changed: int32(len(known))}), nil
}

func (h *Handler) SearchVenues(ctx context.Context, req *connect.Request[isshov1.SearchVenuesRequest]) (*connect.Response[isshov1.SearchVenuesResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	m := req.Msg
	query := strings.TrimSpace(m.Query)
	if query == "" || utf8.RuneCountInString(query) > MaxQuery {
		return nil, invalid("query: 1 to 100 characters")
	}
	if h.Places == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("searching needs GOOGLE_MAPS_API_KEY"))
	}
	q := dbgen.New(h.DB)
	areas, err := cityAreas(ctx, q, m.CityId)
	if err != nil {
		return nil, err
	}
	search := areas
	if m.AreaId != "" {
		search = nil
		for _, a := range areas {
			if a.ID == m.AreaId {
				search = []area{a}
			}
		}
		if search == nil {
			return nil, invalid("area_id: not in that city")
		}
	}

	// One search per area, a few at a time; results in area order, each
	// place once.
	results := make([][]maps.SearchResult, len(search))
	errs := make([]error, len(search))
	var wg sync.WaitGroup
	sem := make(chan struct{}, lookups)
	for i, a := range search {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], errs[i] = h.Places.Search(ctx, maps.SearchRequest{Query: query, Near: a.LatLng, RadiusM: searchRadiusM, Language: "en"})
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("Google search failed: "+err.Error()))
	}
	var found []maps.SearchResult
	seen := map[string]bool{}
	for _, rs := range results {
		for _, r := range rs {
			if r.ID != "" && !seen[r.ID] {
				seen[r.ID] = true
				found = append(found, r)
			}
		}
	}

	ids := make([]string, len(found))
	for i, f := range found {
		ids[i] = f.ID
	}
	known, err := q.ListKnownVenues(ctx, dbgen.ListKnownVenuesParams{CityID: m.CityId, PlaceIds: ids})
	if err != nil {
		return nil, err
	}
	byID := map[string]dbgen.ListKnownVenuesRow{}
	for _, k := range known {
		byID[k.GooglePlaceID] = k
	}
	cats, err := q.ListAllVenueCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := &isshov1.SearchVenuesResponse{Searches: int32(len(search))}
	for _, f := range found {
		near := nearest(areas, f.Location)
		v := &isshov1.FoundVenue{
			PlaceId: f.ID, Name: f.Details.Name, Address: f.Details.Address, MapsUrl: f.Details.MapsURL,
			AreaId: near.ID, AreaName: near.Name, Flag: Flag(f.Details.Name), Closed: f.Closed,
		}
		if k, ok := byID[f.ID]; ok {
			v.Known, v.Status, v.CategoryIds, v.AreaId = true, statusOf(k.Status), k.Categories, k.AreaID
			for _, a := range areas {
				if a.ID == k.AreaID {
					v.AreaName = a.Name
				}
			}
		} else if !f.Closed {
			for _, c := range cats {
				if fits(c, f.Place, near) {
					v.AddableCategoryIds = append(v.AddableCategoryIds, c.ID)
				}
			}
		}
		out.Venues = append(out.Venues, v)
	}
	return connect.NewResponse(out), nil
}

// fits says whether a place could be one of a category's venues: one of
// its Google types (any, not only the primary: an admin is vouching for
// it) is the category's, and it's within the category's search radius of
// its nearest area, as discovery would have found it.
func fits(c dbgen.ListAllVenueCategoriesRow, p maps.Place, near area) bool {
	if maps.DistanceM(near.LatLng, p.Location) > float64(c.SearchRadiusM) {
		return false
	}
	for _, t := range p.Types {
		if slices.Contains(c.GoogleTypes, t) {
			return true
		}
	}
	return false
}

func (h *Handler) AddVenue(ctx context.Context, req *connect.Request[isshov1.AddVenueRequest]) (*connect.Response[isshov1.AddVenueResponse], error) {
	c, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if h.Places == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("adding needs GOOGLE_MAPS_API_KEY"))
	}
	if m.PlaceId == "" || strings.ContainsAny(m.PlaceId, "/?#") {
		return nil, invalid("place_id: a Google place id")
	}
	q := dbgen.New(h.DB)
	areas, err := cityAreas(ctx, q, m.CityId)
	if err != nil {
		return nil, err
	}
	cats, err := q.ListAllVenueCategories(ctx)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(cats, func(c dbgen.ListAllVenueCategoriesRow) bool { return c.ID == m.CategoryId })
	if i < 0 {
		return nil, invalid("category_id: no such category")
	}
	// Checked with Google, not taken from the search the admin saw.
	p, err := h.Places.Locate(ctx, m.PlaceId)
	if err != nil {
		return nil, invalid("place_id: Google doesn't know it")
	}
	if p.Closed {
		return nil, invalid("Google says it's closed")
	}
	near := nearest(areas, p.Location)
	if !fits(cats[i], p.Place, near) {
		return nil, invalid("it isn't that kind of place by Google's types, or it's too far from the city's areas")
	}
	now := pgtype.Timestamptz{Time: h.now(), Valid: true}
	by := pgtype.UUID{Bytes: c.UserID, Valid: true}
	err = pgx.BeginFunc(ctx, h.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := q.AddVenue(ctx, dbgen.AddVenueParams{
			GooglePlaceID: m.PlaceId, CityID: m.CityId, AreaID: near.ID, CategoryID: m.CategoryId, AddedBy: by, At: now,
		}); err != nil {
			return err
		}
		// Curation is per place: its other categories are approved too.
		n, err := q.SetVenueStatus(ctx, dbgen.SetVenueStatusParams{
			CityID: m.CityId, PlaceIds: []string{m.PlaceId}, Status: "approved", ReviewedBy: by, At: now,
		})
		if err == nil && n == 0 {
			// AddVenue skipped it: another city has the place in that category.
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("another city already has this place"))
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.AddVenueResponse{AreaId: near.ID}), nil
}
