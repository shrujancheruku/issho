package geo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/google"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/httpjson"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// quota429 is what Places answers when a quota is used up: the limit is
// named in the message.
func quota429(limit string) error {
	body := fmt.Sprintf(`{"error":{"code":429,"message":"Quota exceeded for quota metric 'SearchNearbyRequest' and limit '%s'","status":"RESOURCE_EXHAUSTED"}}`, limit)
	return errs.Upstream("google", &httpjson.StatusError{StatusCode: 429, Body: body})
}

// scripted answers each call from a list: nil = one place, else the error.
type scripted struct {
	answers []error
	calls   int
}

func (s *scripted) Nearby(_ context.Context, r maps.NearbyRequest) ([]maps.Place, error) {
	i := s.calls
	s.calls++
	if i < len(s.answers) && s.answers[i] != nil {
		return nil, s.answers[i]
	}
	return []maps.Place{{ID: fmt.Sprintf("p%d", i), PrimaryType: r.Types[0], Location: r.Center}}, nil
}

func TestDiscoverJobsKeepsFoundVenuesOnFailure(t *testing.T) {
	cats := []VenueCategory{{ID: "cafe", GoogleTypes: []string{"cafe"}, RadiusM: 1500}}
	s := &scripted{answers: []error{nil, nil, quota429("SearchNearbyRequest per day")}}
	got, n, err := DiscoverJobs(context.Background(), s, areas, PlanVenues(areas, cats), "ja", 100)
	if err == nil {
		t.Fatal("a failed search was not reported")
	}
	if len(got) != 2 || n.Made != 3 {
		t.Errorf("got %d venues after %d searches, want the 2 found before the failure", len(got), n.Made)
	}
	if q, perMin := IsQuotaError(err); !q || perMin {
		t.Errorf("IsQuotaError = %v, %v; want a quota error that is not per minute", q, perMin)
	}
}

func fakeClock(p *Pacer) *[]time.Duration {
	now := time.Unix(1000, 0)
	var slept []time.Duration
	p.now = func() time.Time { return now }
	p.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		now = now.Add(d)
		return nil
	}
	return &slept
}

func TestPacerWaitsOutPerMinuteQuota(t *testing.T) {
	s := &scripted{answers: []error{quota429("SearchNearbyRequest per minute"), quota429("SearchNearbyRequest per minute")}}
	p := NewPacer(s, 0)
	slept := fakeClock(p)
	places, err := p.Nearby(context.Background(), maps.NearbyRequest{Types: []string{"cafe"}})
	if err != nil || len(places) != 1 || s.calls != 3 {
		t.Fatalf("%d places, %d calls, %v; want the third try to succeed", len(places), s.calls, err)
	}
	if len(*slept) != 2 || (*slept)[0] != p.QuotaWait {
		t.Errorf("slept %v, want two quota waits of %v", *slept, p.QuotaWait)
	}
}

func TestPacerStopsOnDailyQuotaAndOnTooManyWaits(t *testing.T) {
	daily := &scripted{answers: []error{quota429("SearchNearbyRequest per day")}}
	p := NewPacer(daily, 0)
	slept := fakeClock(p)
	if _, err := p.Nearby(context.Background(), maps.NearbyRequest{Types: []string{"cafe"}}); err == nil || daily.calls != 1 || len(*slept) != 0 {
		t.Errorf("daily quota: %v after %d calls and %d waits; want an error at once", err, daily.calls, len(*slept))
	}

	perMin := quota429("SearchNearbyRequest per minute")
	always := &scripted{answers: []error{perMin, perMin, perMin, perMin, perMin, perMin, perMin}}
	p = NewPacer(always, 0)
	slept = fakeClock(p)
	if _, err := p.Nearby(context.Background(), maps.NearbyRequest{Types: []string{"cafe"}}); err == nil {
		t.Error("a quota that never clears was waited out forever")
	}
	if len(*slept) != p.MaxWaits || always.calls != p.MaxWaits+1 {
		t.Errorf("%d waits, %d calls; want %d waits", len(*slept), always.calls, p.MaxWaits)
	}

	other := &scripted{answers: []error{errors.New("boom")}}
	p = NewPacer(other, 0)
	slept = fakeClock(p)
	if _, err := p.Nearby(context.Background(), maps.NearbyRequest{Types: []string{"cafe"}}); err == nil || len(*slept) != 0 {
		t.Errorf("other error: %v, %d waits; want it returned at once", err, len(*slept))
	}
}

func TestPacerSpacesRequests(t *testing.T) {
	s := &scripted{}
	p := NewPacer(s, 120) // one every 500 ms
	slept := fakeClock(p)
	for range 4 {
		if _, err := p.Nearby(context.Background(), maps.NearbyRequest{Types: []string{"cafe"}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*slept) != 3 {
		t.Fatalf("slept %v, want 3 gaps after the first request", *slept)
	}
	for _, d := range *slept {
		if d != 500*time.Millisecond {
			t.Errorf("gap %v, want 500ms", d)
		}
	}
}

// A real Places client against a fake server: the 429's body must reach
// IsQuotaError through the HTTP client's error mapping.
func TestPacerWithPlacesServer(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch calls.Add(1) {
		case 1, 2, 3: // the HTTP client's own three attempts
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"Quota exceeded for quota metric 'SearchNearbyRequest' and limit 'SearchNearbyRequest per minute'"}}`))
		default:
			_, _ = w.Write([]byte(`{"places":[{"id":"p1","primaryType":"cafe","location":{"latitude":35.7,"longitude":139.58}}]}`))
		}
	}))
	defer srv.Close()
	c := google.New("test-key")
	c.PlacesURL = srv.URL
	p := NewPacer(c, 0)
	p.QuotaWait = time.Millisecond
	places, err := p.Nearby(context.Background(), maps.NearbyRequest{Types: []string{"cafe"}, RadiusM: 500})
	if err != nil || len(places) != 1 {
		t.Fatalf("%v, %d places; want the per-minute 429 waited out", err, len(places))
	}
}

func TestSearchedSkip(t *testing.T) {
	cats := []VenueCategory{
		{ID: "cafe", GoogleTypes: []string{"cafe"}, RadiusM: 1500},
		{ID: "beach", GoogleTypes: []string{"beach"}, RadiusM: 50000},
	}
	done := Searched{
		pairs: map[[2]string]bool{{"x-a", "cafe"}: true, {"x-b", "cafe"}: true},
		cats:  map[string]bool{"cafe": true},
	}
	todo, skipped := done.Skip(PlanVenues(areas, cats))
	if skipped != 2 || len(todo) != 2 { // cafe in x-c, and the beach
		t.Fatalf("skipped %d, %d left: %+v", skipped, len(todo), todo)
	}
	if todo[0].From != "x-c" || todo[1].Category.ID != "beach" {
		t.Errorf("left %+v", todo)
	}
	beach := Searched{cats: map[string]bool{"beach": true}}
	if todo, _ := beach.Skip(PlanVenues(areas, cats)); len(todo) != 3 {
		t.Errorf("a done city-wide search left %d jobs, want only the cafes", len(todo))
	}
}

// A failed run's saved venues make a rerun skip their pairs, and only those
// newer than the cut-off.
func TestIntegrationLoadSearched(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testfixture.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := dbgen.New(pool)

	const city = "zz-geo-resume"
	cleanup := func() {
		if _, err := pool.Exec(ctx, "delete from app.cities where id = $1", city); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if err := q.UpsertCity(ctx, dbgen.UpsertCityParams{
		ID: city, Name: []byte(`{"en":"Test"}`), Timezone: "UTC", Currency: "USD",
		Languages: []string{"en"}, TravelModes: []string{"walk"},
	}); err != nil {
		t.Fatal(err)
	}
	var rows []Area
	for _, a := range areas[:2] {
		id := city + a.ID
		if err := q.UpsertArea(ctx, dbgen.UpsertAreaParams{ID: id, CityID: city, Name: []byte(`{"en":"A"}`), Lat: a.Lat, Lng: a.Lng}); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, Area{ID: id, LatLng: a.LatLng})
	}
	cat := "cafe"
	if err := SaveVenues(ctx, pool, city, []Venue{{PlaceID: "zz-p1", AreaID: rows[0].ID, CategoryID: cat}}); err != nil {
		t.Fatal(err)
	}
	// The second area's venue was last seen long ago.
	if err := SaveVenues(ctx, pool, city, []Venue{{PlaceID: "zz-p2", AreaID: rows[1].ID, CategoryID: cat}}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "update app.venues set last_seen_at = now() - interval '30 days' where google_place_id = 'zz-p2'"); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSearched(ctx, q, city, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	jobs := PlanVenues(rows, []VenueCategory{{ID: cat, GoogleTypes: []string{"cafe"}, RadiusM: 1500}})
	todo, skipped := s.Skip(jobs)
	if skipped != 1 || len(todo) != 1 || todo[0].From != rows[1].ID {
		t.Errorf("skipped %d, left %+v; want only the first area skipped", skipped, todo)
	}
}
