// Package google implements maps.Router and maps.PlaceSearcher with Google Maps
// Platform: the Routes API (Compute Route Matrix) and Places API (New) Nearby
// Search. It talks plain REST/JSON so there's no SDK dependency.
//
// Every request sends a field mask, which Google requires and which also
// decides the billing SKU: we ask only for ids, types, locations and durations
// (all in the Essentials/Pro tiers, none of the pricier Enterprise fields).
package google

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/googleapi"
)

const (
	defaultRoutesURL     = "https://routes.googleapis.com/distanceMatrix/v2:computeRouteMatrix"
	defaultPlacesURL     = "https://places.googleapis.com/v1/places:searchNearby"
	defaultTextSearchURL = "https://places.googleapis.com/v1/places:searchText"
	defaultDirectionsURL = "https://routes.googleapis.com/directions/v2:computeRoutes"
	// Place Details: GET {defaultDetailsURL}/{place id}.
	defaultDetailsURL = "https://places.googleapis.com/v1/places"

	// Compute Route Matrix allows up to 625 origins × destinations without
	// traffic awareness.
	maxElements = 625
	// Nearby Search (New) returns at most 20 places and has no pagination.
	nearbyMaxResults = 20
)

type Client struct {
	api *googleapi.Client
	// Overridable for tests.
	RoutesURL, PlacesURL, DetailsURL, DirectionsURL, TextSearchURL string
}

func New(apiKey string) *Client {
	return &Client{
		api: googleapi.New(apiKey), RoutesURL: defaultRoutesURL, PlacesURL: defaultPlacesURL, DetailsURL: defaultDetailsURL,
		DirectionsURL: defaultDirectionsURL, TextSearchURL: defaultTextSearchURL,
	}
}

var (
	_ maps.Router        = (*Client)(nil)
	_ maps.PlaceSearcher = (*Client)(nil)
	_ maps.PlaceLookup   = (*Client)(nil)
	_ maps.HoursLookup   = (*Client)(nil)
	_ maps.RouteFinder   = (*Client)(nil)
	_ maps.WalkRouter    = (*Client)(nil)
	_ maps.PlaceFinder   = (*Client)(nil)
	_ maps.PlaceSearch   = (*Client)(nil)
)

// ---- Routes: Compute Route Matrix ----

type latLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type waypoint struct {
	Waypoint struct {
		Location struct {
			LatLng latLng `json:"latLng"`
		} `json:"location"`
	} `json:"waypoint"`
}

type matrixBody struct {
	Origins           []waypoint `json:"origins"`
	Destinations      []waypoint `json:"destinations"`
	TravelMode        string     `json:"travelMode"`
	RoutingPreference string     `json:"routingPreference,omitempty"`
}

type matrixElement struct {
	OriginIndex      int    `json:"originIndex"`
	DestinationIndex int    `json:"destinationIndex"`
	Duration         string `json:"duration"`
	Condition        string `json:"condition"`
	Status           *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
}

func travelMode(m maps.Mode) (string, error) {
	switch m {
	case maps.Walk:
		return "WALK", nil
	case maps.Drive:
		return "DRIVE", nil
	}
	return "", fmt.Errorf("unsupported travel mode %q", m)
}

func toWaypoints(pts []maps.LatLng) []waypoint {
	out := make([]waypoint, len(pts))
	for i, p := range pts {
		out[i].Waypoint.Location.LatLng = latLng{p.Lat, p.Lng}
	}
	return out
}

// Matrix splits the origins into chunks so each request stays within Google's
// element limit, keeping all destinations in every request.
func (c *Client) Matrix(ctx context.Context, req maps.MatrixRequest) ([]maps.MatrixElement, error) {
	mode, err := travelMode(req.Mode)
	if err != nil {
		return nil, err
	}
	if len(req.Origins) == 0 || len(req.Destinations) == 0 {
		return nil, nil
	}
	if len(req.Destinations) > maxElements {
		return nil, fmt.Errorf("%d destinations exceed the %d-element limit", len(req.Destinations), maxElements)
	}
	chunk := maxElements / len(req.Destinations)

	var out []maps.MatrixElement
	for start := 0; start < len(req.Origins); start += chunk {
		end := min(start+chunk, len(req.Origins))
		body := matrixBody{
			Origins:      toWaypoints(req.Origins[start:end]),
			Destinations: toWaypoints(req.Destinations),
			TravelMode:   mode,
		}
		if req.Mode == maps.Drive {
			// Typical rather than live traffic: the matrix is reused for weeks.
			// TRAFFIC_UNAWARE is also the cheapest SKU.
			body.RoutingPreference = "TRAFFIC_UNAWARE"
		}
		var elems []matrixElement
		// "status" must be in the mask, otherwise every element looks OK.
		if err := c.post(ctx, c.RoutesURL, "originIndex,destinationIndex,duration,condition,status", body, &elems); err != nil {
			return nil, err
		}
		for _, e := range elems {
			if e.Status != nil && e.Status.Code != 0 {
				return nil, fmt.Errorf("route matrix element %d→%d: code %d: %s", start+e.OriginIndex, e.DestinationIndex, e.Status.Code, e.Status.Message)
			}
			el := maps.MatrixElement{Origin: start + e.OriginIndex, Destination: e.DestinationIndex}
			if e.Condition == "ROUTE_EXISTS" {
				d, err := parseDuration(e.Duration)
				if err != nil {
					return nil, err
				}
				el.Duration, el.OK = d, true
			}
			out = append(out, el)
		}
	}
	return out, nil
}

// parseDuration reads protobuf JSON durations such as "1234s" or "3.5s".
// Zero values may be omitted entirely (same origin and destination).
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	secs, err := strconv.ParseFloat(strings.TrimSuffix(s, "s"), 64)
	if err != nil || !strings.HasSuffix(s, "s") {
		return 0, fmt.Errorf("bad duration %q", s)
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// ---- Routes: Compute Routes (one walking route) ----

// WalkingRoute asks Routes for the walk between two places, addressed by
// place id (no coordinates needed). The field mask keeps it to the
// essentials SKU: duration, distance and the line.
func (c *Client) WalkingRoute(ctx context.Context, fromPlace, toPlace string) (maps.Route, error) {
	type place struct {
		PlaceID string `json:"placeId"`
	}
	body := struct {
		Origin      place  `json:"origin"`
		Destination place  `json:"destination"`
		TravelMode  string `json:"travelMode"`
	}{place{fromPlace}, place{toPlace}, "WALK"}
	var resp struct {
		Routes []struct {
			Duration       string `json:"duration"`
			DistanceMeters int    `json:"distanceMeters"`
			Polyline       struct {
				EncodedPolyline string `json:"encodedPolyline"`
			} `json:"polyline"`
		} `json:"routes"`
	}
	if err := c.post(ctx, c.DirectionsURL, "routes.duration,routes.distanceMeters,routes.polyline.encodedPolyline", body, &resp); err != nil {
		return maps.Route{}, err
	}
	if len(resp.Routes) == 0 {
		return maps.Route{}, errors.New("no walking route")
	}
	r := resp.Routes[0]
	d, err := parseDuration(r.Duration)
	if err != nil {
		return maps.Route{}, err
	}
	return maps.Route{Polyline: r.Polyline.EncodedPolyline, Duration: d, Meters: r.DistanceMeters}, nil
}

// WalkThrough asks Routes for one walk through several places. With
// optimize, Google may reorder the places in between (Compute Routes'
// optimizeWaypointOrder, the Advanced SKU); the field mask is otherwise the
// same essentials as WalkingRoute, plus each leg.
func (c *Client) WalkThrough(ctx context.Context, places []string, optimize bool) (maps.WalkRoute, error) {
	if len(places) < 2 {
		return maps.WalkRoute{}, errors.New("a walk needs at least two places")
	}
	// Google allows 25 places in between; walks have at most a dozen.
	if len(places) > 27 {
		return maps.WalkRoute{}, fmt.Errorf("%d places exceed the 25 intermediates Routes allows", len(places))
	}
	type place struct {
		PlaceID string `json:"placeId"`
	}
	body := struct {
		Origin        place   `json:"origin"`
		Destination   place   `json:"destination"`
		Intermediates []place `json:"intermediates,omitempty"`
		TravelMode    string  `json:"travelMode"`
		Optimize      bool    `json:"optimizeWaypointOrder,omitempty"`
	}{Origin: place{places[0]}, Destination: place{places[len(places)-1]}, TravelMode: "WALK", Optimize: optimize}
	for _, p := range places[1 : len(places)-1] {
		body.Intermediates = append(body.Intermediates, place{p})
	}
	type leg struct {
		Duration       string `json:"duration"`
		DistanceMeters int    `json:"distanceMeters"`
	}
	var resp struct {
		Routes []struct {
			leg
			Polyline struct {
				EncodedPolyline string `json:"encodedPolyline"`
			} `json:"polyline"`
			Legs           []leg `json:"legs"`
			OptimizedOrder []int `json:"optimizedIntermediateWaypointIndex"`
		} `json:"routes"`
	}
	mask := "routes.duration,routes.distanceMeters,routes.polyline.encodedPolyline,routes.legs.duration,routes.legs.distanceMeters"
	if optimize {
		mask += ",routes.optimizedIntermediateWaypointIndex"
	}
	if err := c.post(ctx, c.DirectionsURL, mask, body, &resp); err != nil {
		return maps.WalkRoute{}, err
	}
	if len(resp.Routes) == 0 {
		return maps.WalkRoute{}, errors.New("no walking route")
	}
	r := resp.Routes[0]
	if len(r.Legs) != len(places)-1 {
		return maps.WalkRoute{}, fmt.Errorf("%d legs for %d places", len(r.Legs), len(places))
	}
	total, err := parseDuration(r.Duration)
	if err != nil {
		return maps.WalkRoute{}, err
	}
	w := maps.WalkRoute{Route: maps.Route{Polyline: r.Polyline.EncodedPolyline, Duration: total, Meters: r.DistanceMeters}}
	for _, l := range r.Legs {
		d, err := parseDuration(l.Duration)
		if err != nil {
			return maps.WalkRoute{}, err
		}
		w.Legs = append(w.Legs, maps.Route{Duration: d, Meters: l.DistanceMeters})
	}
	// Google lists the intermediates' original indexes in walking order;
	// without optimizing (or with nothing in between) it's the same order.
	w.Order = append(w.Order, 0)
	if optimize && len(r.OptimizedOrder) == len(places)-2 {
		for _, i := range r.OptimizedOrder {
			if i < 0 || i >= len(places)-2 {
				return maps.WalkRoute{}, fmt.Errorf("optimized order names place %d of %d", i, len(places)-2)
			}
			w.Order = append(w.Order, i+1)
		}
	} else {
		for i := 1; i < len(places)-1; i++ {
			w.Order = append(w.Order, i)
		}
	}
	w.Order = append(w.Order, len(places)-1)
	return w, nil
}

// ---- Places API (New): Text Search ----

// Find looks a place up by text with Text Search (New), biased to a circle.
// The field mask (id, name, types, location, business status) is the Pro
// SKU. Text Search always returns its nearest match, even for a name that
// doesn't exist, so the caller checks the name (in English, where Google
// has one) as well as the type and location; the name is never kept.
func (c *Client) Find(ctx context.Context, req maps.FindRequest) (maps.Found, bool, error) {
	if strings.TrimSpace(req.Query) == "" {
		return maps.Found{}, false, errors.New("text search needs a query")
	}
	var body struct {
		TextQuery      string `json:"textQuery"`
		LanguageCode   string `json:"languageCode"`
		MaxResultCount int    `json:"maxResultCount"`
		LocationBias   struct {
			Circle struct {
				Center latLng  `json:"center"`
				Radius float64 `json:"radius"`
			} `json:"circle"`
		} `json:"locationBias"`
	}
	body.TextQuery, body.MaxResultCount, body.LanguageCode = req.Query, 1, "en"
	body.LocationBias.Circle.Center = latLng{req.Near.Lat, req.Near.Lng}
	body.LocationBias.Circle.Radius = req.RadiusM
	var resp struct {
		Places []struct {
			ID             string   `json:"id"`
			PrimaryType    string   `json:"primaryType"`
			Types          []string `json:"types"`
			Location       latLng   `json:"location"`
			BusinessStatus string   `json:"businessStatus"`
			DisplayName    struct {
				Text string `json:"text"`
			} `json:"displayName"`
		} `json:"places"`
	}
	if err := c.post(ctx, c.TextSearchURL, "places.id,places.displayName,places.primaryType,places.types,places.location,places.businessStatus", body, &resp); err != nil {
		return maps.Found{}, false, err
	}
	if len(resp.Places) == 0 {
		return maps.Found{}, false, nil
	}
	p := resp.Places[0]
	return maps.Found{
		Place: maps.Place{ID: p.ID, PrimaryType: p.PrimaryType, Types: p.Types, Location: maps.LatLng{Lat: p.Location.Latitude, Lng: p.Location.Longitude}},
		Name:  p.DisplayName.Text,
		// Streets and areas have no business status: they're "open".
		Closed: p.BusinessStatus == "CLOSED_PERMANENTLY" || p.BusinessStatus == "CLOSED_TEMPORARILY",
	}, true, nil
}

// textSearchMax is the most Text Search returns on one page.
const textSearchMax = 20

// searchPlace is a Text Search or Place Details result with the fields
// Search and Locate ask for (all Pro SKU).
type searchPlace struct {
	ID                    string   `json:"id"`
	PrimaryType           string   `json:"primaryType"`
	Types                 []string `json:"types"`
	Location              latLng   `json:"location"`
	BusinessStatus        string   `json:"businessStatus"`
	ShortFormattedAddress string   `json:"shortFormattedAddress"`
	GoogleMapsURI         string   `json:"googleMapsUri"`
	DisplayName           struct {
		Text string `json:"text"`
	} `json:"displayName"`
}

func (p searchPlace) result() maps.SearchResult {
	loc := maps.LatLng{Lat: p.Location.Latitude, Lng: p.Location.Longitude}
	return maps.SearchResult{
		Place:   maps.Place{ID: p.ID, PrimaryType: p.PrimaryType, Types: p.Types, Location: loc},
		Details: maps.PlaceDetails{Name: p.DisplayName.Text, Address: p.ShortFormattedAddress, MapsURL: p.GoogleMapsURI, Location: loc},
		Closed:  p.BusinessStatus == "CLOSED_PERMANENTLY" || p.BusinessStatus == "CLOSED_TEMPORARILY",
	}
}

const searchFields = "id,displayName,primaryType,types,location,businessStatus,shortFormattedAddress,googleMapsUri"

// Search looks places up by text inside a box around req.Near: a
// restriction, not a bias, so a search for "スナック" in one area doesn't
// fill up with the next area's. One page (up to 20 places).
func (c *Client) Search(ctx context.Context, req maps.SearchRequest) ([]maps.SearchResult, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("text search needs a query")
	}
	type corner struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	var body struct {
		TextQuery           string `json:"textQuery"`
		LanguageCode        string `json:"languageCode,omitempty"`
		PageSize            int    `json:"pageSize"`
		LocationRestriction struct {
			Rectangle struct {
				Low  corner `json:"low"`
				High corner `json:"high"`
			} `json:"rectangle"`
		} `json:"locationRestriction"`
	}
	body.TextQuery, body.LanguageCode, body.PageSize = req.Query, req.Language, textSearchMax
	// Metres to degrees: 111,320 m per degree of latitude, fewer per
	// degree of longitude away from the equator.
	dLat := req.RadiusM / 111320
	dLng := req.RadiusM / (111320 * math.Cos(req.Near.Lat*math.Pi/180))
	body.LocationRestriction.Rectangle.Low = corner{req.Near.Lat - dLat, req.Near.Lng - dLng}
	body.LocationRestriction.Rectangle.High = corner{req.Near.Lat + dLat, req.Near.Lng + dLng}
	var resp struct {
		Places []searchPlace `json:"places"`
	}
	mask := "places." + strings.ReplaceAll(searchFields, ",", ",places.")
	if err := c.post(ctx, c.TextSearchURL, mask, body, &resp); err != nil {
		return nil, err
	}
	out := make([]maps.SearchResult, 0, len(resp.Places))
	for _, p := range resp.Places {
		out = append(out, p.result())
	}
	return out, nil
}

// Locate fetches a place's types, location and status (and its English
// name and address) with Place Details, to check a place an admin adds.
func (c *Client) Locate(ctx context.Context, id string) (maps.SearchResult, error) {
	if id == "" || strings.ContainsAny(id, "/?#") {
		return maps.SearchResult{}, fmt.Errorf("invalid place id %q", id)
	}
	var p searchPlace
	u := c.DetailsURL + "/" + url.PathEscape(id) + "?languageCode=en"
	if err := c.api.Get(ctx, u, map[string]string{"X-Goog-FieldMask": searchFields}, &p); err != nil {
		return maps.SearchResult{}, err
	}
	return p.result(), nil
}

// ---- Places API (New): Nearby Search ----

type nearbyBody struct {
	// Primary types, not includedTypes: includedTypes matches a place if ANY of
	// its types matches, which brought in gyms for "sauna", supermarkets for
	// "market" and hostess bars for "night_club".
	IncludedPrimaryTypes []string `json:"includedPrimaryTypes"`
	MaxResultCount       int      `json:"maxResultCount"`
	RankPreference       string   `json:"rankPreference"`
	LanguageCode         string   `json:"languageCode,omitempty"`
	LocationRestriction  struct {
		Circle struct {
			Center latLng  `json:"center"`
			Radius float64 `json:"radius"`
		} `json:"circle"`
	} `json:"locationRestriction"`
}

func (c *Client) Nearby(ctx context.Context, req maps.NearbyRequest) ([]maps.Place, error) {
	if len(req.Types) == 0 {
		return nil, errors.New("nearby search needs at least one place type")
	}
	n := req.Max
	if n <= 0 || n > nearbyMaxResults {
		n = nearbyMaxResults
	}
	body := nearbyBody{
		IncludedPrimaryTypes: req.Types,
		MaxResultCount:       n,
		// Popular places first: they're more likely to be open, findable and
		// good for groups than the closest ones.
		RankPreference: "POPULARITY",
		LanguageCode:   req.Language,
	}
	body.LocationRestriction.Circle.Center = latLng{req.Center.Lat, req.Center.Lng}
	body.LocationRestriction.Circle.Radius = req.RadiusM

	var resp struct {
		Places []struct {
			ID          string   `json:"id"`
			PrimaryType string   `json:"primaryType"`
			Types       []string `json:"types"`
			Location    latLng   `json:"location"`
		} `json:"places"`
	}
	if err := c.post(ctx, c.PlacesURL, "places.id,places.primaryType,places.types,places.location", body, &resp); err != nil {
		return nil, err
	}
	out := make([]maps.Place, 0, len(resp.Places))
	for _, p := range resp.Places {
		out = append(out, maps.Place{ID: p.ID, PrimaryType: p.PrimaryType, Types: p.Types, Location: maps.LatLng{Lat: p.Location.Latitude, Lng: p.Location.Longitude}})
	}
	return out, nil
}

// ---- Places API (New): Place Details ----

// Details fetches what the app shows about a venue. The field mask decides
// the billing SKU: displayName and googleMapsUri make it Place Details Pro.
func (c *Client) Details(ctx context.Context, id, language string) (maps.PlaceDetails, error) {
	if id == "" || strings.ContainsAny(id, "/?#") {
		return maps.PlaceDetails{}, fmt.Errorf("invalid place id %q", id)
	}
	u := c.DetailsURL + "/" + url.PathEscape(id)
	if language != "" {
		u += "?languageCode=" + url.QueryEscape(language)
	}
	var resp struct {
		DisplayName struct {
			Text string `json:"text"`
		} `json:"displayName"`
		ShortFormattedAddress string `json:"shortFormattedAddress"`
		GoogleMapsURI         string `json:"googleMapsUri"`
		Location              latLng `json:"location"`
	}
	mask := "displayName,shortFormattedAddress,googleMapsUri,location"
	if err := c.api.Get(ctx, u, map[string]string{"X-Goog-FieldMask": mask}, &resp); err != nil {
		return maps.PlaceDetails{}, err
	}
	return maps.PlaceDetails{
		Name:     resp.DisplayName.Text,
		Address:  resp.ShortFormattedAddress,
		MapsURL:  resp.GoogleMapsURI,
		Location: maps.LatLng{Lat: resp.Location.Latitude, Lng: resp.Location.Longitude},
	}, nil
}

// Hours fetches a venue's business status and regular opening hours. The
// opening-hours field makes it Place Details Enterprise, the dearest SKU,
// so callers fetch it only when about to schedule.
func (c *Client) Hours(ctx context.Context, id string) (maps.Hours, error) {
	if id == "" || strings.ContainsAny(id, "/?#") {
		return maps.Hours{}, fmt.Errorf("invalid place id %q", id)
	}
	type point struct {
		Day    *int `json:"day"`
		Hour   int  `json:"hour"`
		Minute int  `json:"minute"`
	}
	var resp struct {
		BusinessStatus      string `json:"businessStatus"`
		RegularOpeningHours *struct {
			Periods []struct {
				Open  point  `json:"open"`
				Close *point `json:"close"`
			} `json:"periods"`
		} `json:"regularOpeningHours"`
	}
	u := c.DetailsURL + "/" + url.PathEscape(id)
	if err := c.api.Get(ctx, u, map[string]string{"X-Goog-FieldMask": "businessStatus,regularOpeningHours"}, &resp); err != nil {
		return maps.Hours{}, err
	}
	h := maps.Hours{
		PermanentlyClosed: resp.BusinessStatus == "CLOSED_PERMANENTLY",
		Closed:            resp.BusinessStatus == "CLOSED_PERMANENTLY" || resp.BusinessStatus == "CLOSED_TEMPORARILY",
	}
	if resp.RegularOpeningHours == nil || len(resp.RegularOpeningHours.Periods) == 0 {
		return h, nil
	}
	h.Known = true
	at := func(p point) maps.WeekTime {
		d := 0
		if p.Day != nil {
			d = *p.Day
		}
		return maps.WeekTime{Day: time.Weekday(d), Hour: p.Hour, Minute: p.Minute}
	}
	for _, p := range resp.RegularOpeningHours.Periods {
		period := maps.Period{Open: at(p.Open), NoClose: p.Close == nil}
		if p.Close != nil {
			period.Close = at(*p.Close)
		}
		h.Periods = append(h.Periods, period)
	}
	return h, nil
}

// post calls a Maps endpoint with a field mask (required by Google, and it
// decides the billing SKU).
func (c *Client) post(ctx context.Context, url, fieldMask string, body, out any) error {
	return c.api.Post(ctx, url, map[string]string{"X-Goog-FieldMask": fieldMask}, body, out)
}

// APIError is a non-2xx response from Google.
type APIError = googleapi.APIError
