package geo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/google"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/httpjson"
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
	got, n, err := DiscoverJobs(context.Background(), s, areas, PlanVenues(areas, cats), "ja", 100, nil)
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

// failingAt answers like a working searcher except for the nth call (from 1).
type failingAt struct {
	n, calls int
	reqs     []maps.NearbyRequest
}

func (f *failingAt) Nearby(_ context.Context, r maps.NearbyRequest) ([]maps.Place, error) {
	f.calls++
	f.reqs = append(f.reqs, r)
	if f.calls == f.n {
		return nil, quota429("SearchNearbyRequest per day")
	}
	return []maps.Place{{ID: fmt.Sprintf("p%v", r.Center), PrimaryType: r.Types[0], Location: r.Center}}, nil
}

// A run fails midway; its venues are saved (here: kept) and committed; the
// rerun searches exactly what was left, and -fresh searches everything.
func TestProgressRerunSkipsFinishedSearches(t *testing.T) {
	cats := []VenueCategory{
		{ID: "cafe", GoogleTypes: []string{"cafe"}, RadiusM: 1500},
		{ID: "beach", GoogleTypes: []string{"beach"}, RadiusM: 50000},
	}
	jobs := PlanVenues(areas, cats) // 3 cafes + 1 beach
	path := filepath.Join(t.TempDir(), "sub", "venues-x.json")
	ctx := context.Background()

	prog, err := LoadProgress(path, false)
	if err != nil {
		t.Fatal(err)
	}
	first := &failingAt{n: 3} // two searches finish, the third fails
	got, n, err := DiscoverJobs(ctx, first, areas, jobs, "ja", 100, prog)
	if err == nil || n.Made != 3 || len(got) != 2 {
		t.Fatalf("first run: %d venues, %+v, %v", len(got), n, err)
	}
	if err := prog.Commit(); err != nil { // the venues were saved
		t.Fatal(err)
	}

	prog, err = LoadProgress(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if prog.Finished(jobs) != 2 {
		t.Errorf("dry run counts %d finished, want 2", prog.Finished(jobs))
	}
	second := &failingAt{}
	_, n, err = DiscoverJobs(ctx, second, areas, jobs, "ja", 100, prog)
	if err != nil || n.Made != 2 || n.Resumed != 2 || len(second.reqs) != 2 {
		t.Fatalf("rerun: %+v, %d requests, %v; want the other 2 searches only", n, len(second.reqs), err)
	}
	for _, r := range second.reqs {
		if r.Center == areas[0].LatLng || r.Center == areas[1].LatLng {
			t.Errorf("rerun repeated the search at %v", r.Center)
		}
	}

	// Not committed (saving failed): nothing new is listed.
	prog, _ = LoadProgress(path, false)
	if prog.Finished(jobs) != 2 {
		t.Errorf("uncommitted rerun listed %d, want still 2", prog.Finished(jobs))
	}

	fresh, err := LoadProgress(path, true)
	if err != nil {
		t.Fatal(err)
	}
	all := &failingAt{}
	if _, n, err = DiscoverJobs(ctx, all, areas, jobs, "ja", 100, fresh); err != nil || n.Made != 4 || n.Resumed != 0 {
		t.Errorf("fresh: %+v, %v; want all 4 searches", n, err)
	}

	if err := prog.Clear(); err != nil {
		t.Fatal(err)
	}
	if prog, _ = LoadProgress(path, false); prog.Finished(jobs) != 0 {
		t.Error("a cleared progress file still lists searches")
	}
}

// A finished search that came back full still leads to its finer searches,
// which are searched on the rerun without repeating the parent.
func TestProgressKeepsSplitsOfFinishedSearches(t *testing.T) {
	wc := maps.LatLng{Lat: 16.0, Lng: 108.2}
	wards := []Area{{ID: "x-w", LatLng: wc, Boundary: square(wc, 3000)}}
	cats := []VenueCategory{{ID: "cafe", GoogleTypes: []string{"cafe"}, RadiusM: 800}}
	jobs := PlanVenues(wards, cats)
	dense := &denseSearcher{dense: wc}
	path := filepath.Join(t.TempDir(), "p.json")

	// Run one stops after the first search (budget): the children are skipped.
	prog, _ := LoadProgress(path, false)
	_, n, err := DiscoverJobs(context.Background(), dense, wards, jobs, "vi", 1, prog)
	if err != nil || n.Made != 1 || n.Skipped == 0 {
		t.Fatalf("%+v, %v", n, err)
	}
	if err := prog.Commit(); err != nil {
		t.Fatal(err)
	}

	prog, _ = LoadProgress(path, false)
	again := &denseSearcher{dense: wc}
	_, n, err = DiscoverJobs(context.Background(), again, wards, jobs, "vi", 1000, prog)
	if err != nil || n.Resumed != 1 || n.Made != len(again.reqs) || n.Made < 2 {
		t.Fatalf("%+v, %d requests, %v; want the parent resumed and its children searched", n, len(again.reqs), err)
	}
	for _, r := range again.reqs {
		if r.Center == jobs[0].Center && r.RadiusM == jobs[0].RadiusM {
			t.Error("the finished parent search was repeated")
		}
	}
}
