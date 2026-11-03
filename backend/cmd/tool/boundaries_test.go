package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/cities"
)

// redirectNominatim sends the tool's Nominatim requests to handler instead of
// OpenStreetMap, by swapping the default client's transport (the lookup URL is
// a constant), and counts them.
func redirectNominatim(t *testing.T, handler http.HandlerFunc) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		if !strings.Contains(r.UserAgent(), "issho") {
			t.Errorf("User-Agent = %q; Nominatim's policy needs an identifying one", r.UserAgent())
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "nominatim.openstreetmap.org" {
			return nil, fmt.Errorf("unexpected host %s: tests must not leave the machine", r.URL.Host)
		}
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return http.DefaultTransport.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	return &n
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// square is a GeoJSON polygon of half-side d degrees around a point.
func square(lat, lng, d float64) string {
	c := func(la, ln float64) string { return fmt.Sprintf("[%v,%v]", ln, la) }
	return fmt.Sprintf(`{"type":"Polygon","coordinates":[[%s,%s,%s,%s,%s]]}`,
		c(lat-d, lng-d), c(lat-d, lng+d), c(lat+d, lng+d), c(lat+d, lng-d), c(lat-d, lng-d))
}

func boundaryPack(t *testing.T) (path string, before []byte) {
	t.Helper()
	p := cities.Pack{
		ID: "zz", Name: cities.Text{"en": "ZZ"}, Timezone: "UTC", Currency: "USD",
		Languages: []string{"en"}, TravelModes: []string{"walk"},
		Areas: []cities.Area{
			{ID: "zz-a", Name: cities.Text{"en": "A"}, Lat: 10, Lng: 20, OSMID: 111},
			{ID: "zz-b", Name: cities.Text{"en": "B"}, Lat: 11, Lng: 21, OSMID: 222},
			{ID: "zz-c", Name: cities.Text{"en": "C"}, Lat: 12, Lng: 22}, // no osm_id: left alone
		},
	}
	b, err := cities.EncodePack(p)
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(t.TempDir(), "zz.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path, b
}

func TestAreaBoundaries(t *testing.T) {
	path, before := boundaryPack(t)
	lookup := func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("osm_ids"); got != "R111,R222" {
			t.Errorf("osm_ids = %q; want R111,R222 (only areas with an osm_id)", got)
		}
		_, _ = fmt.Fprintf(w, `[{"osm_type":"relation","osm_id":111,"geojson":%s},{"osm_type":"relation","osm_id":222,"geojson":%s},{"osm_type":"node","osm_id":333,"geojson":{"type":"Point","coordinates":[0,0]}}]`,
			square(10, 20, 0.01), square(11, 21, 0.01))
	}

	t.Run("writes the outlines", func(t *testing.T) {
		n := redirectNominatim(t, lookup)
		if err := areaBoundaries([]string{path}); err != nil {
			t.Fatal(err)
		}
		if n.Load() != 1 {
			t.Errorf("%d Nominatim requests, want 1 for two areas", n.Load())
		}
		p, err := cities.ReadPack(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Areas[0].Boundary) != 1 || len(p.Areas[1].Boundary) != 1 || len(p.Areas[2].Boundary) != 0 {
			t.Errorf("boundaries = %v / %v / %v; want one ring, one ring, none", p.Areas[0].Boundary, p.Areas[1].Boundary, p.Areas[2].Boundary)
		}
		// A second run writes the same file.
		first, _ := os.ReadFile(path)
		if err := areaBoundaries([]string{path}); err != nil {
			t.Fatal(err)
		}
		if again, _ := os.ReadFile(path); string(again) != string(first) {
			t.Error("a second run changed the file")
		}
	})

	// Each refusal leaves the file as it was.
	refusals := map[string]struct {
		handler http.HandlerFunc
		want    string
	}{
		"nominatim down": {func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }, "nominatim"},
		"no boundary for a relation": {func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `[{"osm_type":"relation","osm_id":111,"geojson":%s}]`, square(10, 20, 0.01))
		}, "no boundary for relation 222"},
		"a point, not a polygon": {func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, `[{"osm_type":"relation","osm_id":111,"geojson":{"type":"Point","coordinates":[20,10]}}]`)
		}, "not a polygon"},
		// The outline fails the checks CI runs: it doesn't hold the area's point.
		"outline misses the area": {func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `[{"osm_type":"relation","osm_id":111,"geojson":%s},{"osm_type":"relation","osm_id":222,"geojson":%s}]`,
				square(50, 50, 0.01), square(11, 21, 0.01))
		}, path},
	}
	for name, tc := range refusals {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, before, 0o644); err != nil {
				t.Fatal(err)
			}
			redirectNominatim(t, tc.handler)
			err := areaBoundaries([]string{path})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v; want one mentioning %q", err, tc.want)
			}
			if got, _ := os.ReadFile(path); string(got) != string(before) {
				t.Error("the file changed")
			}
		})
	}
}

// Usage errors and a pack with nothing to fetch never reach Nominatim.
func TestAreaBoundariesNoRequest(t *testing.T) {
	n := redirectNominatim(t, func(http.ResponseWriter, *http.Request) {})
	if err := areaBoundaries(nil); err == nil {
		t.Error("no file: want a usage error")
	}
	if err := areaBoundaries([]string{filepath.Join(t.TempDir(), "missing.json")}); err == nil {
		t.Error("missing file: want an error")
	}
	p := cities.Pack{
		ID: "zz", Name: cities.Text{"en": "ZZ"}, Timezone: "UTC", Currency: "USD", Languages: []string{"en"}, TravelModes: []string{"walk"},
		Areas: []cities.Area{{ID: "zz-a", Name: cities.Text{"en": "A"}, Lat: 10, Lng: 20}},
	}
	b, err := cities.EncodePack(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "zz.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := areaBoundaries([]string{path}); err == nil || !strings.Contains(err.Error(), "no area has an osm_id") {
		t.Errorf("err = %v; want no area has an osm_id", err)
	}
	if n.Load() != 0 {
		t.Errorf("%d Nominatim requests, want none", n.Load())
	}
}
