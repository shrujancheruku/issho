package curation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/paging"
)

const (
	// MaxBatch is how many places SetStatus takes at once.
	MaxBatch = 100
	// MaxQuery bounds a search's text.
	MaxQuery = 100
	// searchRadiusM is half the side of the box searched around an area's
	// centre: about the radius discovery searches most categories in.
	searchRadiusM = 1500
	// lookups bounds concurrent Google calls for one request.
	lookups = 8
)

// Service is venue curation: reviewing places, looking places up by name,
// adding ones discovery missed. Its methods return Connect errors for the
// caller's mistakes (as the other services do); anything else is ours and
// the errs interceptor hides it.
type Service struct {
	DB *pgxpool.Pool
	// Places is nil without GOOGLE_MAPS_API_KEY: names are then blank, and
	// searching and adding are refused.
	Places maps.PlaceSearch
	Now    func() time.Time // nil = time.Now
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// area is one of a city's areas, with its English name.
type area struct {
	geo.Area
	Name string
}

var errNoCity = connect.NewError(connect.CodeNotFound, errors.New("no such city"))

// cityAreas returns a city's areas; NOT_FOUND for an unknown city.
func cityAreas(ctx context.Context, q *dbgen.Queries, cityID string) ([]area, error) {
	rows, err := q.ListCityCurationAreas(ctx, cityID)
	if err != nil {
		return nil, err
	}
	out := make([]area, 0, len(rows))
	for _, r := range rows {
		b, err := geo.ParseBoundary(r.Boundary)
		if err != nil {
			return nil, fmt.Errorf("area %s: %w", r.ID, err)
		}
		out = append(out, area{Area: geo.Area{ID: r.ID, LatLng: maps.LatLng{Lat: r.Lat, Lng: r.Lng}, Boundary: b}, Name: r.Name})
	}
	if len(out) == 0 {
		return nil, errNoCity
	}
	return out, nil
}

// nearest is the area p is in (geo.Locate): the one whose boundary has it,
// or the nearest by point.
func nearest(areas []area, p maps.LatLng) area {
	plain := make([]geo.Area, len(areas))
	for i, a := range areas {
		plain[i] = a.Area
	}
	n, _ := geo.Locate(plain, p)
	for _, a := range areas {
		if a.ID == n.ID {
			return a
		}
	}
	return areas[0]
}

// fits says whether a place could be one of a category's venues: one of
// its Google types (any, not only the primary: an admin is vouching for
// it) is the category's, and it's where discovery could have found it:
// inside its area's boundary, or within the category's search radius of
// the area's point.
func fits(c dbgen.ListAllVenueCategoriesRow, p maps.Place, near area) bool {
	if !near.Boundary.Contains(p.Location) && maps.DistanceM(near.LatLng, p.Location) > float64(c.SearchRadiusM) {
		return false
	}
	for _, t := range p.Types {
		if slices.Contains(c.GoogleTypes, t) {
			return true
		}
	}
	return false
}

// Cities lists every city with its review counts and areas, and the venue
// categories.
func (s *Service) Cities(ctx context.Context) (cities []dbgen.ListVenueCitiesRow, areas []dbgen.ListCurationAreasRow, cats []dbgen.ListAllVenueCategoriesRow, err error) {
	q := dbgen.New(s.DB)
	if cities, err = q.ListVenueCities(ctx); err != nil {
		return
	}
	if areas, err = q.ListCurationAreas(ctx); err != nil {
		return
	}
	cats, err = q.ListAllVenueCategories(ctx)
	return
}

// SetApprovedOnly switches a city to using only approved venues (or back).
func (s *Service) SetApprovedOnly(ctx context.Context, cityID string, on bool) error {
	n, err := dbgen.New(s.DB).SetApprovedVenuesOnly(ctx, dbgen.SetApprovedVenuesOnlyParams{ID: cityID, ApprovedVenuesOnly: on})
	if err != nil {
		return err
	}
	if n == 0 {
		return errNoCity
	}
	return nil
}

// ListParams selects one page of a city's places in a status. PageSize is
// 1 to paging.MaxSize (each place costs a Place Details call for its name);
// Token is the previous page's NextToken.
type ListParams struct {
	CityID, AreaID, CategoryID string
	Status                     string // candidate, approved or rejected
	PageSize                   int32
	Token                      string
}

// Listed is a place with the live details Google gave for it (blank when
// there are none).
type Listed struct {
	dbgen.ListCurationVenuesRow
	Name, Address, MapsURL, Flag string
}

// List returns a page of places, how many match in all, and the token for
// the next page ("" on the last). Names are fetched live (Maps terms); a
// failed lookup leaves one blank.
func (s *Service) List(ctx context.Context, p ListParams) (places []Listed, total int32, next string, err error) {
	size, err := paging.Size(p.PageSize)
	if err != nil {
		return nil, 0, "", err
	}
	scope := paging.Scope("ListVenues", p.CityID, p.AreaID, p.CategoryID, p.Status)
	k, err := paging.Open(p.Token, scope, 3)
	if err != nil {
		return nil, 0, "", err
	}
	arg := dbgen.ListCurationVenuesParams{
		CityID: p.CityID, AreaID: p.AreaID, CategoryID: p.CategoryID, Status: p.Status, MaxRows: size + 1,
	}
	arg.HasAfter, arg.AfterAreaName, arg.AfterAreaID, arg.AfterPlaceID = k.Present, k.String(), k.String(), k.String()
	if err := k.Err(); err != nil {
		return nil, 0, "", err
	}
	rows, err := dbgen.New(s.DB).ListCurationVenues(ctx, arg)
	if err != nil {
		return nil, 0, "", err
	}
	rows, more := paging.Trim(rows, size)
	if more {
		last := rows[len(rows)-1]
		next = paging.Encode(scope, last.AreaName, last.AreaID, last.GooglePlaceID)
	}
	places = make([]Listed, len(rows))
	ids := make([]string, len(rows))
	for i, r := range rows {
		total = r.Total
		ids[i] = r.GooglePlaceID
		places[i].ListCurationVenuesRow = r
	}
	s.details(ctx, ids, func(i int, d maps.PlaceDetails) {
		l := &places[i]
		l.Name, l.Address, l.MapsURL, l.Flag = d.Name, d.Address, d.MapsURL, Flag(d.Name)
	})
	return places, total, next, nil
}

// details fetches places' English details, a few at a time, calling set
// (from one goroutine at a time) for each that succeeded.
func (s *Service) details(ctx context.Context, ids []string, set func(int, maps.PlaceDetails)) {
	if s.Places == nil {
		return
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, lookups)
	for i, id := range ids {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			if d, err := s.Places.Details(ctx, id, "en"); err == nil {
				mu.Lock()
				set(i, d)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
}

// SetStatus approves, rejects or resets (to candidate) places of a city,
// recording who and when, and returns how many distinct places it changed
// (not how many were asked for: unknown ids count for nothing, and a place's
// rows, one per category, count once).
func (s *Service) SetStatus(ctx context.Context, cityID string, placeIDs []string, status string, by uuid.UUID) (int, error) {
	if len(placeIDs) == 0 || len(placeIDs) > MaxBatch {
		return 0, errs.Invalid("place_ids: 1 to %d", MaxBatch)
	}
	q := dbgen.New(s.DB)
	if _, err := cityAreas(ctx, q, cityID); err != nil {
		return 0, err
	}
	var reviewer pgtype.UUID
	var at pgtype.Timestamptz
	// Back to candidate clears the review: nobody has decided.
	if status != "candidate" {
		reviewer, at = pgtype.UUID{Bytes: by, Valid: true}, pgtype.Timestamptz{Time: s.now(), Valid: true}
	}
	changed, err := q.SetVenueStatus(ctx, dbgen.SetVenueStatusParams{
		CityID: cityID, PlaceIds: placeIDs, Status: status, ReviewedBy: reviewer, At: at,
	})
	if err != nil {
		return 0, err
	}
	slices.Sort(changed)
	return len(slices.Compact(changed)), nil
}

// SearchParams is a name search in a city, or in one of its areas.
type SearchParams struct {
	CityID, AreaID, Query string
}

// Found is a place a search found, with what the city already knows of it.
type Found struct {
	maps.SearchResult
	// AreaID and AreaName: where the city has the place, else the area it
	// would go in.
	AreaID, AreaName string
	// Known: the city has the place, in CategoryIDs, with Status.
	Known       bool
	Status      string
	CategoryIDs []string
	// Addable lists the categories a new, open place could be added in.
	Addable []string
}

// Searched is a search's results, in area order, each place once.
type Searched struct {
	Searches int // how many areas were searched
	Venues   []Found
}

// Search looks a name up with Google in a box around each area of the city
// (or one area).
func (s *Service) Search(ctx context.Context, p SearchParams) (Searched, error) {
	query := strings.TrimSpace(p.Query)
	if query == "" || utf8.RuneCountInString(query) > MaxQuery {
		return Searched{}, errs.Invalid("query: 1 to %d characters", MaxQuery)
	}
	if s.Places == nil {
		return Searched{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("searching needs GOOGLE_MAPS_API_KEY"))
	}
	q := dbgen.New(s.DB)
	areas, err := cityAreas(ctx, q, p.CityID)
	if err != nil {
		return Searched{}, err
	}
	search := areas
	if p.AreaID != "" {
		search = nil
		for _, a := range areas {
			if a.ID == p.AreaID {
				search = []area{a}
			}
		}
		if search == nil {
			return Searched{}, errs.Invalid("area_id: not in that city")
		}
	}

	// One search per area, a few at a time.
	results := make([][]maps.SearchResult, len(search))
	failures := make([]error, len(search))
	var wg sync.WaitGroup
	sem := make(chan struct{}, lookups)
	for i, a := range search {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			near, radius := a.LatLng, float64(searchRadiusM)
			if len(a.Boundary) > 0 {
				// A box over the whole area, as discovery searches all of it.
				near, radius = a.Boundary.Circle()
				radius = max(radius, searchRadiusM)
			}
			results[i], failures[i] = s.Places.Search(ctx, maps.SearchRequest{Query: query, Near: near, RadiusM: radius, Language: "en"})
		})
	}
	wg.Wait()
	if err := errors.Join(failures...); err != nil {
		// Google's text stays in the log, not in the response.
		slog.ErrorContext(ctx, "venue search failed", "city", p.CityID, "err", err)
		return Searched{}, connect.NewError(connect.CodeUnavailable, errors.New("Google search failed; try again"))
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
	known, err := q.ListKnownVenues(ctx, dbgen.ListKnownVenuesParams{CityID: p.CityID, PlaceIds: ids})
	if err != nil {
		return Searched{}, err
	}
	byID := map[string]dbgen.ListKnownVenuesRow{}
	for _, k := range known {
		byID[k.GooglePlaceID] = k
	}
	cats, err := q.ListAllVenueCategories(ctx)
	if err != nil {
		return Searched{}, err
	}
	out := Searched{Searches: len(search)}
	for _, f := range found {
		near := nearest(areas, f.Location)
		v := Found{SearchResult: f, AreaID: near.ID, AreaName: near.Name}
		if k, ok := byID[f.ID]; ok {
			v.Known, v.Status, v.CategoryIDs, v.AreaID = true, k.Status, k.Categories, k.AreaID
			for _, a := range areas {
				if a.ID == k.AreaID {
					v.AreaName = a.Name
				}
			}
		} else if !f.Closed {
			for _, c := range cats {
				if fits(c, f.Place, near) {
					v.Addable = append(v.Addable, c.ID)
				}
			}
		}
		out.Venues = append(out.Venues, v)
	}
	return out, nil
}

// Add puts a place in a category of the city, approved, in the area it is
// nearest. It is checked with Google (not taken from the search the admin
// saw): open, and of that category by Google's types and distance. It
// returns the area it went to.
func (s *Service) Add(ctx context.Context, cityID, placeID, categoryID string, by uuid.UUID) (string, error) {
	if s.Places == nil {
		return "", connect.NewError(connect.CodeFailedPrecondition, errors.New("adding needs GOOGLE_MAPS_API_KEY"))
	}
	if placeID == "" || strings.ContainsAny(placeID, "/?#") {
		return "", errs.Invalid("place_id: a Google place id")
	}
	q := dbgen.New(s.DB)
	areas, err := cityAreas(ctx, q, cityID)
	if err != nil {
		return "", err
	}
	cats, err := q.ListAllVenueCategories(ctx)
	if err != nil {
		return "", err
	}
	i := slices.IndexFunc(cats, func(c dbgen.ListAllVenueCategoriesRow) bool { return c.ID == categoryID })
	if i < 0 {
		return "", errs.Invalid("category_id: no such category")
	}
	p, err := s.Places.Locate(ctx, placeID)
	if err != nil {
		return "", errs.Invalid("place_id: Google doesn't know it")
	}
	if p.Closed {
		return "", errs.Invalid("Google says it's closed")
	}
	near := nearest(areas, p.Location)
	if !fits(cats[i], p.Place, near) {
		return "", errs.Invalid("it isn't that kind of place by Google's types, or it's too far from the city's areas")
	}
	now := pgtype.Timestamptz{Time: s.now(), Valid: true}
	reviewer := pgtype.UUID{Bytes: by, Valid: true}
	err = pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := q.AddVenue(ctx, dbgen.AddVenueParams{
			GooglePlaceID: placeID, CityID: cityID, AreaID: near.ID, CategoryID: categoryID, AddedBy: reviewer, At: now,
		}); err != nil {
			return err
		}
		// Curation is per place: its other categories are approved too.
		changed, err := q.SetVenueStatus(ctx, dbgen.SetVenueStatusParams{
			CityID: cityID, PlaceIds: []string{placeID}, Status: "approved", ReviewedBy: reviewer, At: now,
		})
		if err == nil && len(changed) == 0 {
			// AddVenue skipped it: another city has the place in that category.
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("another city already has this place"))
		}
		return err
	})
	if err != nil {
		return "", err
	}
	return near.ID, nil
}
