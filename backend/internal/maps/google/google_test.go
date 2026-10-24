package google

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New("test-key")
	c.RoutesURL, c.PlacesURL, c.DetailsURL, c.DirectionsURL, c.TextSearchURL = srv.URL, srv.URL, srv.URL, srv.URL, srv.URL
	c.api.Backoff = time.Millisecond
	return c
}

func pts(n int) []maps.LatLng {
	out := make([]maps.LatLng, n)
	for i := range out {
		out[i] = maps.LatLng{Lat: 35 + float64(i)/100, Lng: 139}
	}
	return out
}

// fakeMatrix answers like Compute Route Matrix: one element per pair, every
// route 600s except global origin 0 → destination 1, which has no route.
func fakeMatrix(t *testing.T, calls *atomic.Int32, maxElements *atomic.Int32, body *matrixBody) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Goog-Api-Key") != "test-key" {
			t.Errorf("missing api key header")
		}
		if r.URL.Query().Get("key") != "" {
			t.Errorf("api key must not be in the URL")
		}
		if fm := r.Header.Get("X-Goog-FieldMask"); fm == "" {
			t.Errorf("missing field mask")
		}
		var b matrixBody
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			t.Fatal(err)
		}
		*body = b
		calls.Add(1)
		n := int32(len(b.Origins) * len(b.Destinations))
		if n > maxElements.Load() {
			maxElements.Store(n)
		}
		var out []map[string]any
		for o := range b.Origins {
			for d := range b.Destinations {
				// The real API reports the origin index within this request.
				e := map[string]any{"originIndex": o, "destinationIndex": d, "status": map[string]any{}}
				if o == 0 && d == 1 && b.Origins[0].Waypoint.Location.LatLng.Latitude == 35 {
					e["condition"] = "ROUTE_NOT_FOUND"
				} else {
					e["condition"], e["duration"] = "ROUTE_EXISTS", "600s"
				}
				out = append(out, e)
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	}
}

func TestMatrixChunksTo625Elements(t *testing.T) {
	var calls, maxEl atomic.Int32
	var last matrixBody
	c := newTestClient(t, fakeMatrix(t, &calls, &maxEl, &last))

	got, err := c.Matrix(context.Background(), maps.MatrixRequest{Origins: pts(40), Destinations: pts(25), Mode: maps.Walk})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1000 {
		t.Fatalf("got %d elements, want 1000", len(got))
	}
	if maxEl.Load() > maxElements {
		t.Errorf("a request had %d elements, limit is %d", maxEl.Load(), maxElements)
	}
	if calls.Load() != 2 { // 25 origins per request → ceil(40/25)
		t.Errorf("got %d requests, want 2", calls.Load())
	}
	if last.TravelMode != "WALK" || last.RoutingPreference != "" {
		t.Errorf("unexpected walk body: %+v", last)
	}

	// Origin indexes must be global, not per chunk, and unique.
	seen := map[[2]int]bool{}
	for _, e := range got {
		k := [2]int{e.Origin, e.Destination}
		if seen[k] {
			t.Fatalf("duplicate element %v", k)
		}
		seen[k] = true
		wantOK := !(e.Origin == 0 && e.Destination == 1)
		if e.OK != wantOK {
			t.Errorf("element %v: OK=%v, want %v", k, e.OK, wantOK)
		}
		if e.OK && e.Duration != 10*time.Minute {
			t.Errorf("element %v: duration %v", k, e.Duration)
		}
	}
}

func TestMatrixDriveIsTrafficUnaware(t *testing.T) {
	var calls, maxEl atomic.Int32
	var last matrixBody
	c := newTestClient(t, fakeMatrix(t, &calls, &maxEl, &last))
	_, err := c.Matrix(context.Background(), maps.MatrixRequest{Origins: pts(25), Destinations: pts(25), Mode: maps.Drive})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Errorf("625 drive elements should fit one request, got %d", calls.Load())
	}
	if last.TravelMode != "DRIVE" || last.RoutingPreference != "TRAFFIC_UNAWARE" {
		t.Errorf("unexpected drive body: %+v", last)
	}
}

func TestMatrixElementErrorFails(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"originIndex":0,"destinationIndex":0,"status":{"code":3,"message":"bad waypoint"}}]`))
	})
	_, err := c.Matrix(context.Background(), maps.MatrixRequest{Origins: pts(1), Destinations: pts(1), Mode: maps.Walk})
	if err == nil {
		t.Fatal("expected an error for a failed element")
	}
}

func TestRetriesOn429ThenFailsLoudly(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`))
	})
	_, err := c.Nearby(context.Background(), maps.NearbyRequest{Types: []string{"cafe"}, RadiusM: 500})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
		t.Fatalf("got %v, want a 429 APIError", err)
	}
	if calls.Load() != int32(c.api.MaxRetries+1) {
		t.Errorf("got %d attempts, want %d", calls.Load(), c.api.MaxRetries+1)
	}
}

func TestNoRetryOn400(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	})
	if _, err := c.Nearby(context.Background(), maps.NearbyRequest{Types: []string{"cafe"}}); err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("400 must not be retried, got %d attempts", calls.Load())
	}
}

func TestNearby(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if fm := r.Header.Get("X-Goog-FieldMask"); fm != "places.id,places.primaryType,places.types,places.location" {
			t.Errorf("field mask %q requests unexpected fields", fm)
		}
		var b nearbyBody
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.MaxResultCount != 20 || b.LocationRestriction.Circle.Radius != 800 || b.IncludedPrimaryTypes[0] != "cafe" {
			t.Errorf("unexpected body %+v", b)
		}
		_, _ = w.Write([]byte(`{"places":[{"id":"ChIJ1","primaryType":"cafe","types":["cafe","food"],"location":{"latitude":35.71,"longitude":139.59}}]}`))
	})
	got, err := c.Nearby(context.Background(), maps.NearbyRequest{
		Center: maps.LatLng{Lat: 35.7, Lng: 139.58}, RadiusM: 800, Types: []string{"cafe"}, Max: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "ChIJ1" || got[0].PrimaryType != "cafe" || got[0].Location.Lat != 35.71 {
		t.Errorf("got %+v", got)
	}
}

func TestParseDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{"": 0, "0s": 0, "1234s": 1234 * time.Second, "3.5s": 3500 * time.Millisecond} {
		if got, err := parseDuration(in); err != nil || got != want {
			t.Errorf("parseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseDuration("12"); err == nil {
		t.Error("expected error for a duration without unit")
	}
}

func TestDetails(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ChIJabc" || r.URL.Query().Get("languageCode") != "ja" {
			t.Errorf("request %s %s", r.Method, r.URL)
		}
		if r.Header.Get("X-Goog-Api-Key") != "test-key" || r.Header.Get("X-Goog-FieldMask") == "" {
			t.Errorf("headers %v", r.Header)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"displayName":           map[string]string{"text": "B-PUMP 荻窪", "languageCode": "ja"},
			"shortFormattedAddress": "杉並区上荻1-2-3",
			"googleMapsUri":         "https://maps.google.com/?cid=1",
			"location":              map[string]float64{"latitude": 35.7, "longitude": 139.6},
		})
	})
	got, err := c.Details(context.Background(), "ChIJabc", "ja")
	if err != nil {
		t.Fatal(err)
	}
	want := maps.PlaceDetails{Name: "B-PUMP 荻窪", Address: "杉並区上荻1-2-3", MapsURL: "https://maps.google.com/?cid=1", Location: maps.LatLng{Lat: 35.7, Lng: 139.6}}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	// An id that could change the path is refused before any request.
	if _, err := c.Details(context.Background(), "../places:searchNearby", "ja"); err == nil {
		t.Error("no error for a path-like id")
	}
}

func TestHours(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Goog-FieldMask") != "businessStatus,regularOpeningHours" {
			t.Errorf("field mask %q", r.Header.Get("X-Goog-FieldMask"))
		}
		switch r.URL.Path {
		case "/open":
			// Sunday (day 0) is sent without "day", as protobuf JSON omits zero values.
			_, _ = w.Write([]byte(`{"businessStatus":"OPERATIONAL","regularOpeningHours":{"periods":[
				{"open":{"hour":10},"close":{"hour":18}},
				{"open":{"day":6,"hour":20},"close":{"day":0,"hour":2}}]}}`))
		case "/gone":
			_, _ = w.Write([]byte(`{"businessStatus":"CLOSED_PERMANENTLY"}`))
		case "/park":
			_, _ = w.Write([]byte(`{"businessStatus":"OPERATIONAL"}`))
		}
	})
	h, err := c.Hours(context.Background(), "open")
	if err != nil {
		t.Fatal(err)
	}
	want := []maps.Period{
		{Open: maps.WeekTime{Day: time.Sunday, Hour: 10}, Close: maps.WeekTime{Day: time.Sunday, Hour: 18}},
		{Open: maps.WeekTime{Day: time.Saturday, Hour: 20}, Close: maps.WeekTime{Day: time.Sunday, Hour: 2}},
	}
	if !h.Known || h.Closed || len(h.Periods) != 2 || h.Periods[0] != want[0] || h.Periods[1] != want[1] {
		t.Errorf("hours = %+v", h)
	}
	if h, _ := c.Hours(context.Background(), "gone"); !h.PermanentlyClosed || !h.Closed {
		t.Errorf("permanently closed = %+v", h)
	}
	if h, _ := c.Hours(context.Background(), "park"); h.Known || h.Closed {
		t.Errorf("no hours = %+v", h)
	}
}

func TestWalkingRoute(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["travelMode"] != "WALK" || body["origin"].(map[string]any)["placeId"] != "from" || r.Header.Get("X-Goog-FieldMask") == "" {
			t.Errorf("request %v, mask %q", body, r.Header.Get("X-Goog-FieldMask"))
		}
		_, _ = w.Write([]byte(`{"routes":[{"duration":"612s","distanceMeters":780,"polyline":{"encodedPolyline":"_p~iF~ps|U_ulLnnqC"}}]}`))
	})
	r, err := c.WalkingRoute(context.Background(), "from", "to")
	if err != nil {
		t.Fatal(err)
	}
	if r.Polyline != "_p~iF~ps|U_ulLnnqC" || r.Duration != 612*time.Second || r.Meters != 780 {
		t.Errorf("route = %+v", r)
	}
	none := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	if _, err := none.WalkingRoute(context.Background(), "a", "b"); err == nil {
		t.Error("no error without a route")
	}
}

func TestWalkThrough(t *testing.T) {
	var body map[string]any
	var mask string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body = nil
		_ = json.NewDecoder(r.Body).Decode(&body)
		mask = r.Header.Get("X-Goog-FieldMask")
		// Google walks the middle places in the order c, b.
		_, _ = w.Write([]byte(`{"routes":[{"duration":"1500s","distanceMeters":2000,"polyline":{"encodedPolyline":"line"},
			"legs":[{"duration":"300s","distanceMeters":400},{"duration":"600s","distanceMeters":800},{"duration":"600s","distanceMeters":800}],
			"optimizedIntermediateWaypointIndex":[1,0]}]}`))
	})
	w, err := c.WalkThrough(context.Background(), []string{"a", "b", "c", "d"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if body["optimizeWaypointOrder"] != true || body["origin"].(map[string]any)["placeId"] != "a" ||
		body["destination"].(map[string]any)["placeId"] != "d" || len(body["intermediates"].([]any)) != 2 {
		t.Errorf("request %v", body)
	}
	if mask == "" || !strings.Contains(mask, "routes.legs.duration") || !strings.Contains(mask, "optimizedIntermediateWaypointIndex") {
		t.Errorf("mask %q", mask)
	}
	if w.Polyline != "line" || w.Duration != 1500*time.Second || w.Meters != 2000 || len(w.Legs) != 3 ||
		w.Legs[1].Duration != 600*time.Second || !slices.Equal(w.Order, []int{0, 2, 1, 3}) {
		t.Errorf("walk = %+v", w)
	}

	// Without optimizing, the order is the request's and isn't asked for.
	if _, err := c.WalkThrough(context.Background(), []string{"a", "b", "c", "d"}, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["optimizeWaypointOrder"]; ok || strings.Contains(mask, "optimized") {
		t.Errorf("unoptimized request %v, mask %q", body, mask)
	}
	if _, err := c.WalkThrough(context.Background(), []string{"a"}, false); err == nil {
		t.Error("one place is not a walk")
	}
	short := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"routes":[{"duration":"60s","legs":[{"duration":"60s"}]}]}`))
	})
	if _, err := short.WalkThrough(context.Background(), []string{"a", "b", "c"}, false); err == nil {
		t.Error("a missing leg must fail")
	}
}

func TestFind(t *testing.T) {
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		if m := r.Header.Get("X-Goog-FieldMask"); !strings.Contains(m, "displayName") || !strings.Contains(m, "businessStatus") {
			t.Errorf("mask %q", m)
		}
		switch body["textQuery"] {
		case "Kayaba Coffee, Yanaka":
			_, _ = w.Write([]byte(`{"places":[{"id":"p1","displayName":{"text":"Kayaba Coffee"},"primaryType":"cafe","types":["cafe","food"],"location":{"latitude":35.72,"longitude":139.77},"businessStatus":"OPERATIONAL"}]}`))
		case "Gone":
			_, _ = w.Write([]byte(`{"places":[{"id":"p2","businessStatus":"CLOSED_PERMANENTLY"}]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	ctx := context.Background()
	near := maps.LatLng{Lat: 35.7, Lng: 139.7}
	f, ok, err := c.Find(ctx, maps.FindRequest{Query: "Kayaba Coffee, Yanaka", Near: near, RadiusM: 3000})
	if err != nil || !ok || f.ID != "p1" || f.Name != "Kayaba Coffee" || f.PrimaryType != "cafe" || f.Closed || f.Location.Lat != 35.72 {
		t.Errorf("found %+v %v %v", f, ok, err)
	}
	bias := body["locationBias"].(map[string]any)["circle"].(map[string]any)
	if bias["radius"] != 3000.0 || body["maxResultCount"] != 1.0 || body["languageCode"] != "en" {
		t.Errorf("request %v", body)
	}
	if f, ok, _ := c.Find(ctx, maps.FindRequest{Query: "Gone", Near: near, RadiusM: 1}); !ok || !f.Closed {
		t.Errorf("closed place = %+v", f)
	}
	if _, ok, err := c.Find(ctx, maps.FindRequest{Query: "Nowhere", Near: near, RadiusM: 1}); ok || err != nil {
		t.Errorf("no match: %v %v", ok, err)
	}
	if _, _, err := c.Find(ctx, maps.FindRequest{Query: " "}); err == nil {
		t.Error("an empty query must fail")
	}
}

func TestSearchAndLocate(t *testing.T) {
	var body map[string]any
	var path string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		mask := r.Header.Get("X-Goog-FieldMask")
		if r.Method == http.MethodGet {
			if mask != searchFields {
				t.Errorf("details mask %q", mask)
			}
			_, _ = w.Write([]byte(`{"id":"p1","displayName":{"text":"Snack Yuki"},"primaryType":"bar","types":["bar","establishment"],"location":{"latitude":35.7,"longitude":139.6},"businessStatus":"CLOSED_TEMPORARILY"}`))
			return
		}
		if !strings.Contains(mask, "places.shortFormattedAddress") || !strings.Contains(mask, "places.googleMapsUri") {
			t.Errorf("search mask %q", mask)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"places":[{"id":"p1","displayName":{"text":"Snack Yuki"},"primaryType":"bar","types":["bar"],"location":{"latitude":35.7,"longitude":139.6},"businessStatus":"OPERATIONAL","shortFormattedAddress":"1-2-3 Koenji","googleMapsUri":"https://maps.google.com/?cid=1"}]}`))
	})
	ctx := context.Background()
	got, err := c.Search(ctx, maps.SearchRequest{Query: "スナック", Near: maps.LatLng{Lat: 35, Lng: 139}, RadiusM: 1113.2, Language: "en"})
	if err != nil || len(got) != 1 {
		t.Fatalf("Search = %+v, %v", got, err)
	}
	if r := got[0]; r.ID != "p1" || r.Details.Name != "Snack Yuki" || r.Details.Address != "1-2-3 Koenji" || r.Details.MapsURL == "" || r.Closed || r.Location.Lng != 139.6 {
		t.Errorf("result %+v", r)
	}
	rect := body["locationRestriction"].(map[string]any)["rectangle"].(map[string]any)
	low, high := rect["low"].(map[string]any), rect["high"].(map[string]any)
	// 1,113.2 m is 0.01° of latitude.
	if d := high["latitude"].(float64) - low["latitude"].(float64); d < 0.0199 || d > 0.0201 {
		t.Errorf("box %v", rect)
	}
	if body["textQuery"] != "スナック" || body["pageSize"] != 20.0 || body["languageCode"] != "en" {
		t.Errorf("request %v", body)
	}
	if _, err := c.Search(ctx, maps.SearchRequest{Query: ""}); err == nil {
		t.Error("an empty query must fail")
	}

	r, err := c.Locate(ctx, "p1")
	if err != nil || r.PrimaryType != "bar" || !r.Closed || !slices.Contains(r.Types, "establishment") {
		t.Errorf("Locate = %+v, %v", r, err)
	}
	if path != "/p1?languageCode=en" {
		t.Errorf("details path %q", path)
	}
	if _, err := c.Locate(ctx, "a/b"); err == nil {
		t.Error("a bad id must fail")
	}
}
