package google

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	c.RoutesURL, c.PlacesURL = srv.URL, srv.URL
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
// route 600s except origin 0 → destination 1, which has no route.
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

func TestMatrixChunksTransitTo100Elements(t *testing.T) {
	var calls, maxEl atomic.Int32
	var last matrixBody
	c := newTestClient(t, fakeMatrix(t, &calls, &maxEl, &last))
	dep := time.Date(2026, 10, 24, 11, 0, 0, 0, time.FixedZone("JST", 9*3600))

	got, err := c.Matrix(context.Background(), maps.MatrixRequest{Origins: pts(25), Destinations: pts(25), Mode: maps.Transit, Departure: dep})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 625 {
		t.Fatalf("got %d elements, want 625", len(got))
	}
	if maxEl.Load() > transitMaxElements {
		t.Errorf("a request had %d elements, limit is %d", maxEl.Load(), transitMaxElements)
	}
	if calls.Load() != 7 { // 4 origins per request → ceil(25/4)
		t.Errorf("got %d requests, want 7", calls.Load())
	}
	if last.TravelMode != "TRANSIT" || last.DepartureTime != "2026-10-24T02:00:00Z" || last.RoutingPreference != "" {
		t.Errorf("unexpected transit body: %+v", last)
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
	_, err := c.Matrix(context.Background(), maps.MatrixRequest{Origins: pts(25), Destinations: pts(25), Mode: maps.Drive, Departure: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Errorf("625 drive elements should fit one request, got %d", calls.Load())
	}
	if last.RoutingPreference != "TRAFFIC_UNAWARE" || last.DepartureTime != "" {
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
		if b.MaxResultCount != 20 || b.LocationRestriction.Circle.Radius != 800 || b.IncludedTypes[0] != "cafe" {
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
