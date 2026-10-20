// Package google implements maps.Router and maps.PlaceSearcher with Google Maps
// Platform: the Routes API (Compute Route Matrix) and Places API (New) Nearby
// Search. It talks plain REST/JSON so there's no SDK dependency.
//
// Every request sends a field mask, which Google requires and which also
// decides the billing SKU: we ask only for ids, types, locations and durations
// (all in the Essentials/Pro tiers, none of the pricier Enterprise fields).
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

const (
	defaultRoutesURL = "https://routes.googleapis.com/distanceMatrix/v2:computeRouteMatrix"
	defaultPlacesURL = "https://places.googleapis.com/v1/places:searchNearby"

	// Compute Route Matrix limits origins × destinations to 100 for TRANSIT and
	// 625 for other modes without traffic awareness.
	transitMaxElements = 100
	otherMaxElements   = 625
	// Nearby Search (New) returns at most 20 places and has no pagination.
	nearbyMaxResults = 20
)

type Client struct {
	key  string
	http *http.Client
	// Overridable for tests.
	RoutesURL, PlacesURL string
	// Retries for 429 and 5xx responses, with exponential backoff from Backoff.
	MaxRetries int
	Backoff    time.Duration
}

func New(apiKey string) *Client {
	return &Client{
		key:        apiKey,
		http:       &http.Client{Timeout: 60 * time.Second},
		RoutesURL:  defaultRoutesURL,
		PlacesURL:  defaultPlacesURL,
		MaxRetries: 3,
		Backoff:    time.Second,
	}
}

var (
	_ maps.Router        = (*Client)(nil)
	_ maps.PlaceSearcher = (*Client)(nil)
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
	DepartureTime     string     `json:"departureTime,omitempty"`
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
	case maps.Transit:
		return "TRANSIT", nil
	case maps.Walk:
		return "WALK", nil
	case maps.Drive:
		return "DRIVE", nil
	case maps.Bicycle:
		return "BICYCLE", nil
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
	limit := otherMaxElements
	if req.Mode == maps.Transit {
		limit = transitMaxElements
	}
	if len(req.Destinations) > limit {
		return nil, fmt.Errorf("%d destinations exceed the %d-element limit for %s", len(req.Destinations), limit, req.Mode)
	}
	chunk := limit / len(req.Destinations)

	var out []maps.MatrixElement
	for start := 0; start < len(req.Origins); start += chunk {
		end := min(start+chunk, len(req.Origins))
		body := matrixBody{
			Origins:      toWaypoints(req.Origins[start:end]),
			Destinations: toWaypoints(req.Destinations),
			TravelMode:   mode,
		}
		switch req.Mode {
		case maps.Drive:
			// Typical rather than live traffic: the matrix is reused for weeks.
			// TRAFFIC_UNAWARE is also the cheapest SKU and rejects departureTime.
			body.RoutingPreference = "TRAFFIC_UNAWARE"
		case maps.Transit:
			if !req.Departure.IsZero() {
				body.DepartureTime = req.Departure.UTC().Format(time.RFC3339)
			}
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

// ---- Places API (New): Nearby Search ----

type nearbyBody struct {
	IncludedTypes       []string `json:"includedTypes"`
	MaxResultCount      int      `json:"maxResultCount"`
	RankPreference      string   `json:"rankPreference"`
	LanguageCode        string   `json:"languageCode,omitempty"`
	LocationRestriction struct {
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
		IncludedTypes:  req.Types,
		MaxResultCount: n,
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

// ---- transport ----

// APIError is a non-2xx response from Google. The body is Google's error JSON,
// which never contains the API key.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("google maps: HTTP %d: %s", e.StatusCode, e.Body)
}

func retryable(code int) bool { return code == http.StatusTooManyRequests || code >= 500 }

func (c *Client) post(ctx context.Context, url, fieldMask string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		err := c.postOnce(ctx, url, fieldMask, payload, out)
		var apiErr *APIError
		if err == nil || !errors.As(err, &apiErr) || !retryable(apiErr.StatusCode) || attempt >= c.MaxRetries {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.Backoff << attempt):
		}
	}
}

func (c *Client) postOnce(ctx context.Context, url, fieldMask string, payload []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// Header rather than ?key= so the key never appears in URLs, which end up in
	// logs and error messages.
	req.Header.Set("X-Goog-Api-Key", c.key)
	req.Header.Set("X-Goog-FieldMask", fieldMask)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	return json.Unmarshal(b, out)
}
