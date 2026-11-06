package citypacks

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/adminjobs"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	gmaps "github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/mapstest"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
	"github.com/shrujancheruku/issho/backend/internal/repoedit"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// fakeRepo is the repository in memory (repoedit's Host): branches of files,
// and open pull requests. files is main's.
type fakeRepo struct {
	files    map[string][]byte
	branches map[string]map[string][]byte
	prs      []github.PR
	commits  []string // messages
	content  []byte   // the last pack file committed
}

func newRepo(files map[string][]byte) *fakeRepo {
	return &fakeRepo{files: files, branches: map[string]map[string][]byte{"main": files}}
}

// merge lands a pull request's branch on main and closes it.
func (f *fakeRepo) merge(t *testing.T) {
	t.Helper()
	if len(f.prs) != 1 {
		t.Fatalf("merge: %d open pull requests", len(f.prs))
	}
	for p, c := range f.branches[f.prs[0].Branch] {
		f.files[p] = c
	}
	f.prs = nil
}

func (f *fakeRepo) File(_ context.Context, path, branch string) ([]byte, string, error) {
	b, ok := f.branches[branch][path]
	if !ok {
		return nil, "", github.ErrNotFound
	}
	return b, "sha-" + path, nil
}

func (f *fakeRepo) Dir(_ context.Context, dir, _ string) ([]string, error) {
	var out []string
	for p := range f.files {
		if name, ok := strings.CutPrefix(p, dir+"/"); ok {
			out = append(out, name)
		}
	}
	return out, nil
}

func (f *fakeRepo) OpenPRs(_ context.Context, prefix string) ([]github.PR, error) {
	var out []github.PR
	for _, pr := range f.prs {
		if strings.HasPrefix(pr.Branch, prefix) {
			out = append(out, pr)
		}
	}
	return out, nil
}

func (f *fakeRepo) CreateBranch(_ context.Context, branch string) error {
	f.branches[branch] = maps.Clone(f.files)
	return nil
}

func (f *fakeRepo) CommitFiles(_ context.Context, branch string, files map[string][]byte, message string) error {
	for p, c := range files {
		f.branches[branch][p] = c
		f.content = c
	}
	f.commits = append(f.commits, message)
	return nil
}

func (f *fakeRepo) OpenPR(_ context.Context, branch, title, body string) (github.PR, error) {
	pr := github.PR{Number: 100 + len(f.prs), Title: title, URL: "https://github.com/o/r/pull/x", Branch: branch, Body: body}
	f.prs = append(f.prs, pr)
	return pr, nil
}

func (f *fakeRepo) UpdatePR(_ context.Context, number int, body string) error {
	for i := range f.prs {
		if f.prs[i].Number == number {
			f.prs[i].Body = body
		}
	}
	return nil
}

// inlineJobs is the River client with the queue taken out: Insert runs the
// job at once with the worker's own functions (adminjobs) against the same
// database and fakes, and keeps what the job recorded. The queue itself
// (state, uniqueness, GetAdminJob) is tested in internal/worker.
type inlineJobs struct {
	pool   *pgxpool.Pool
	router gmaps.Router
	places gmaps.PlaceSearcher
	args   []river.JobArgs
	out    map[int64]adminjobs.Output
}

func (j *inlineJobs) Insert(ctx context.Context, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	j.args = append(j.args, args)
	id := int64(len(j.args))
	var out adminjobs.Output
	switch a := args.(type) {
	case adminjobs.ApplyCityPackArgs:
		out, _ = adminjobs.ApplyCityPack(ctx, j.pool, j.router, a)
	case adminjobs.DiscoverAreaVenuesArgs:
		out, _ = adminjobs.DiscoverAreaVenues(ctx, j.pool, j.places, a)
	}
	if j.out == nil {
		j.out = map[int64]adminjobs.Output{}
	}
	j.out[id] = out
	return &rivertype.JobInsertResult{Job: &rivertype.JobRow{ID: id}}, nil
}

// TestIntegrationCityPacks: proposing a new city and a change to it as pull
// requests (with refusals), applying the merged pack (areas new and moved,
// travel times rebuilt) and finding an area's venues. Skipped unless
// TEST_DATABASE_URL is set.
func TestIntegrationCityPacks(t *testing.T) {
	ctx := context.Background()
	pool := testfixture.Pool(t)
	const city, act, cat = "zz-packs", "zz-packs-act", "zz-packs-cafe"
	cleanup := func() {
		for _, s := range []string{
			"delete from app.cities where id = '" + city + "'",
			"delete from app.activities where id = '" + act + "'",
			"delete from app.venue_categories where id = '" + cat + "'",
		} {
			if _, err := pool.Exec(ctx, s); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	for _, s := range []string{
		`insert into app.venue_categories (id, name, google_types) values ('` + cat + `', '{"en":"Cafe"}', '{cafe}')`,
		`insert into app.activities (id, name, category, indoor, energy, cost_level, conversational, min_group_size, max_group_size, venue_categories)
		 values ('` + act + `', '{"en":"Coffee"}', 'food', true, 1, 1, true, 2, 6, '{` + cat + `}')`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	repo := newRepo(map[string][]byte{"data/citypacks/other.json": []byte(`{"id":"other"}`)})
	fm := &mapstest.Fake{NearbyFunc: func(r gmaps.NearbyRequest) []gmaps.Place {
		return []gmaps.Place{{ID: "zz-packs-place-" + r.Types[0], PrimaryType: r.Types[0], Location: r.Center}}
	}}
	jobs := &inlineJobs{pool: pool, router: fm, places: fm}
	ed := repoedit.New(repo, "main")
	ed.Now = func() time.Time { return time.Unix(1900000000, 0) }
	h := &Handler{DB: pool, Repo: ed, Places: fm, Jobs: jobs}
	adminCtx := auth.WithCaller(ctx, auth.Caller{UserID: uuid.New(), Email: "admin@example.com", Admin: true})

	pack := &isshov1.CityPack{
		Id: city, Name: map[string]string{"en": "Packs", "ja": "パック"}, Timezone: "Asia/Tokyo", Currency: "jpy",
		Languages: []string{"ja", "en"}, TravelModes: []string{"walk", "drive"},
		Areas: []*isshov1.PackArea{
			{Id: city + "-a", Name: map[string]string{"en": "Alpha"}, Lat: 35.70, Lng: 139.60},
			{Id: city + "-b", Name: map[string]string{"en": "Beta"}, Lat: 35.71, Lng: 139.61},
		},
		ActivityOverrides: []*isshov1.PackOverride{{ActivityId: act, Name: " Kissa ", Months: []int32{4, 5}}},
	}
	propose := func(p *isshov1.CityPack) (*isshov1.ProposeCityPackResponse, error) {
		res, err := h.ProposeCityPack(adminCtx, &isshov1.ProposeCityPackRequest{Pack: p, Note: "Launching soon."})
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	bad := func(name string, change func(p *isshov1.CityPack)) {
		t.Helper()
		var p isshov1.CityPack
		b, _ := json.Marshal(pack)
		_ = json.Unmarshal(b, &p)
		change(&p)
		if _, err := propose(&p); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad("a bad id", func(p *isshov1.CityPack) { p.Id = "Zz Packs" })
	bad("an unknown time zone", func(p *isshov1.CityPack) { p.Timezone = "Mars/Olympus" })
	bad("an area id without the city", func(p *isshov1.CityPack) { p.Areas[1].Id = "beta" })
	bad("an unknown activity", func(p *isshov1.CityPack) { p.ActivityOverrides[0].ActivityId = "nope" })
	bad("a name in another language", func(p *isshov1.CityPack) { p.Areas[0].Name["fr"] = "Alpha" })

	// A new city: a new file on a new branch, in a pull request.
	res, err := propose(pack)
	if err != nil {
		t.Fatal(err)
	}
	if res.PullRequest.Number != 100 || res.PullRequest.Title != "feat(data): add the Packs city pack" || len(repo.prs) != 1 {
		t.Errorf("new city: %v %v", res.PullRequest, repo.prs)
	}
	pr, branch := repo.prs[0], "citypack/zz-packs-1900000000"
	if pr.Branch != branch || repo.commits[0] != "feat(data): add the Packs city pack" ||
		strings.Contains(pr.Body, "admin@example.com") || !strings.Contains(pr.Body, "Launching soon.") || !strings.Contains(pr.Body, "New city Packs") {
		t.Errorf("proposal = %+v, commits %q", pr, repo.commits)
	}
	if _, onMain := repo.files["data/citypacks/zz-packs.json"]; onMain {
		t.Error("a proposal must not touch the default branch")
	}
	var written cities.Pack
	if err := json.Unmarshal(repo.content, &written); err != nil || written.Currency != "JPY" || written.ActivityOverrides[0].Name["en"] != "Kissa" {
		t.Errorf("written = %+v, %v", written, err)
	}
	// The editor opens the pending pack, not main's (there is none yet).
	pending, err := h.GetCityPack(adminCtx, &isshov1.GetCityPackRequest{CityId: city})
	if err != nil || pending.PullRequest.GetNumber() != 100 || pending.Pack.GetCurrency() != "JPY" {
		t.Errorf("GetCityPack of a pending new city: %+v %v", pending, err)
	}
	// Saving again commits to the same pull request, rewriting its body.
	pack.Areas[1].Name["en"] = "Beta Town"
	if res, err = propose(pack); err != nil || res.PullRequest.Number != 100 || len(repo.prs) != 1 || len(repo.commits) != 2 || len(repo.branches) != 2 {
		t.Fatalf("a second proposal: %v %v (prs %d, commits %d, branches %d)", res, err, len(repo.prs), len(repo.commits), len(repo.branches))
	}
	if !strings.Contains(repo.prs[0].Body, "New city Packs") || !strings.Contains(string(repo.branches[branch]["data/citypacks/zz-packs.json"]), "Beta Town") {
		t.Errorf("rolling pull request: %q", repo.prs[0].Body)
	}
	if lp, err := h.ListCityPacks(adminCtx, &isshov1.ListCityPacksRequest{}); err != nil || len(lp.PullRequests) != 1 || lp.PullRequests[0].Number != 100 {
		t.Errorf("list with a pending change: %v %v", lp, err)
	}
	pack.Areas[1].Name["en"] = "Beta"
	if _, err := propose(pack); err != nil {
		t.Fatal(err)
	}

	// An invalid pack on main fails the call at once, with the usual error
	// and no job queued.
	repo.files["data/citypacks/bad.json"] = []byte(`{"id":"bad","timezone":"Mars/Olympus","currency":"JPY"}`)
	if _, err := h.ApplyCityPack(adminCtx, &isshov1.ApplyCityPackRequest{CityId: "bad"}); connect.CodeOf(err) != connect.CodeInvalidArgument || len(jobs.args) != 0 {
		t.Errorf("an invalid pack: %v (%d jobs queued)", err, len(jobs.args))
	}

	// Merged: apply it. Both areas are new; travel times are built.
	repo.merge(t)
	// applyJob queues the apply and returns what its job recorded; apply is
	// the report of one that succeeded, the response the RPC returned
	// before it became a job.
	applyJob := func() adminjobs.Output {
		t.Helper()
		res, err := h.ApplyCityPack(adminCtx, &isshov1.ApplyCityPackRequest{CityId: city})
		if err != nil || res.JobId == 0 {
			t.Fatalf("apply = %v, %v", res, err)
		}
		if len(res.NewAreas) != 0 || res.TravelTimes != 0 {
			t.Errorf("the RPC answered with a report; it belongs to the job: %v", res)
		}
		return jobs.out[res.JobId]
	}
	apply := func() *isshov1.ApplyCityPackResponse {
		t.Helper()
		out := applyJob()
		if out.Failure != nil || out.ApplyCityPack == nil {
			t.Fatalf("apply job failed: %+v", out.Failure)
		}
		return adminjobs.ApplyReport(*out.ApplyCityPack)
	}
	if a := apply(); len(a.NewAreas) != 2 || !a.TravelTimesRebuilt || a.TravelTimes == 0 {
		t.Fatalf("first apply = %v", a)
	}
	var months []int16
	if err := pool.QueryRow(ctx, "select months from app.city_activity_overrides where city_id = $1", city).Scan(&months); err != nil || len(months) != 2 {
		t.Errorf("override months = %v, %v", months, err)
	}

	// Venues for the first area of a new city: a search per kind of place
	// its activities use (a seeded database has the whole catalogue, all
	// enabled in a new city), each finding one place here.
	disc, err := h.DiscoverAreaVenues(adminCtx, &isshov1.DiscoverAreaVenuesRequest{CityId: city, AreaId: city + "-a"})
	if err != nil || disc.JobId == 0 {
		t.Fatalf("discover = %v, %v", disc, err)
	}
	found := jobs.out[disc.JobId].DiscoverAreaVenues
	if found == nil || found.Searches < 1 || found.Venues != found.Searches || fm.Nearbys() != found.Searches {
		t.Fatalf("discover job = %+v", jobs.out[disc.JobId])
	}
	list, err := h.ListCityPacks(adminCtx, &isshov1.ListCityPacksRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list.Packs {
		// Alpha has venues now (Beta may too: city-wide finds go to the
		// nearest area).
		if p.Id == city && (!p.InRepo || !p.Seeded || p.Name != "Packs" || p.Areas != 2 || len(p.AreasWithoutTravelTimes) != 0 ||
			slices.Contains(p.AreasWithoutVenues, city+"-a")) {
			t.Errorf("listed = %v", p)
		}
	}

	// A change: refused when it removes an area or changes nothing.
	got, err := h.GetCityPack(adminCtx, &isshov1.GetCityPackRequest{CityId: city})
	if err != nil {
		t.Fatal(err)
	}
	p := got.Pack
	var mainPack isshov1.CityPack // main's, kept to revert to later
	raw0, _ := json.Marshal(p)
	_ = json.Unmarshal(raw0, &mainPack)
	if _, err := propose(p); connect.CodeOf(err) != connect.CodeInvalidArgument || strings.Contains(err.Error(), "pull request") {
		t.Errorf("no change: %v", err)
	}
	removed := &isshov1.CityPack{Id: p.Id, Name: p.Name, Timezone: p.Timezone, Currency: p.Currency, Languages: p.Languages, TravelModes: p.TravelModes, Areas: p.Areas[:1]}
	if _, err := propose(removed); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("an area removed: %v", err)
	}
	// Beta moves 1 km; a third area.
	p.Areas[1].Lat += 0.009
	p.Areas = append(p.Areas, &isshov1.PackArea{Id: city + "-c", Name: map[string]string{"en": "Gamma"}, Lat: 35.72, Lng: 139.62})
	if _, err := propose(p); err != nil {
		t.Fatal(err)
	}
	if len(repo.prs) != 1 || repo.commits[len(repo.commits)-1] != "feat(data): update the Packs city pack" ||
		!strings.Contains(repo.prs[0].Body, "Area zz-packs-b moved 1001 m") || !strings.Contains(repo.prs[0].Body, "New area Gamma") {
		t.Errorf("change proposal = %+v, commits %q", repo.prs, repo.commits)
	}
	// A further save rides on the same pull request. Its body is the whole
	// change from the default branch, and the checks still run against it.
	pend, err := h.GetCityPack(adminCtx, &isshov1.GetCityPackRequest{CityId: city})
	if err != nil || int(pend.PullRequest.GetNumber()) != repo.prs[0].Number || len(pend.Pack.Areas) != 3 {
		t.Fatalf("GetCityPack of a pending change: %+v %v", pend, err)
	}
	prs, commits := len(repo.prs), len(repo.commits)
	pend.Pack.Areas[2].Lat += 0.001
	if _, err := propose(pend.Pack); err != nil || len(repo.prs) != prs || len(repo.commits) != commits+1 ||
		!strings.Contains(repo.prs[0].Body, "Area zz-packs-b moved 1001 m") || !strings.Contains(repo.prs[0].Body, "New area Gamma") {
		t.Errorf("rolling change: %v prs %d, commits %d, body %q", err, len(repo.prs), len(repo.commits), repo.prs[0].Body)
	}
	removedPending := &isshov1.CityPack{Id: p.Id, Name: p.Name, Timezone: p.Timezone, Currency: p.Currency, Languages: p.Languages, TravelModes: p.TravelModes, Areas: p.Areas[:1]}
	if _, err := propose(removedPending); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("an area removed from main's pack, though the pending one has it: %v", err)
	}
	// Back to main's pack while a pending change holds edits: the editor can't
	// revert it, and says to close the pull request.
	if _, err := propose(&mainPack); connect.CodeOf(err) != connect.CodeInvalidArgument ||
		!strings.Contains(err.Error(), "pull request #"+strconv.Itoa(repo.prs[0].Number)) || !strings.Contains(err.Error(), "close") {
		t.Errorf("reverting to main with a pending change: %v", err)
	}
	repo.merge(t)
	before := fm.Matrices()
	if a := apply(); len(a.NewAreas) != 1 || a.NewAreas[0] != city+"-c" || len(a.MovedAreas) != 1 || a.MovedAreas[0] != city+"-b" || fm.Matrices() == before {
		t.Errorf("second apply = %v", a)
	}
	// Applying again changes nothing and calls nobody.
	before = fm.Matrices()
	if a := apply(); len(a.NewAreas)+len(a.MovedAreas) != 0 || a.TravelTimesRebuilt || fm.Matrices() != before {
		t.Errorf("third apply = %v", a)
	}

	// An authoritative apply reports what it took away: Gamma is replaced by
	// Delta (its people, crews and plans move), the override the pack
	// dropped, and then Epsilon, an unreferenced area, deleted outright.
	edit := func(change func(p *cities.Pack)) {
		t.Helper()
		cur := cities.Pack{}
		if err := json.Unmarshal(repo.content, &cur); err != nil {
			t.Fatal(err)
		}
		change(&cur)
		raw, err := cities.EncodePack(cur)
		if err != nil {
			t.Fatal(err)
		}
		repo.files["data/citypacks/zz-packs.json"] = raw
	}
	// Gamma has 1 person, 2 crews and 3 plans, so the counts are distinct.
	fx := testfixture.New(t, "zz-packs-fx", "Asia/Tokyo", 1)
	fx.AddProfile(t, fx.Users[0], "ja")
	seed := []string{"update app.profiles set home_area_id = '" + city + "-c' where user_id = '" + fx.Users[0].String() + "'"}
	for range 2 {
		seed = append(seed, "insert into app.crews (city_id, area_id, offer_until) values ('"+city+"', '"+city+"-c', now())")
	}
	for range 3 {
		seed = append(seed, `insert into app.plans (city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
		   values ('`+city+`', '`+city+`-c', '`+act+`', 'ja', 1, 1, current_date, 7, now())`)
	}
	for _, q := range seed {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	edit(func(p *cities.Pack) {
		p.Areas[2] = cities.Area{ID: city + "-d", Name: cities.Text{"en": "Delta"}, Lat: 35.72, Lng: 139.62, Replaces: []string{city + "-c"}}
		p.Areas = append(p.Areas, cities.Area{ID: city + "-e", Name: cities.Text{"en": "Epsilon"}, Lat: 35.73, Lng: 139.63})
		p.ActivityOverrides = nil
	})
	rem := apply()
	if len(rem.ReplacedAreas) != 1 || rem.ReplacedAreas[0].OldAreaId != city+"-c" || rem.ReplacedAreas[0].NewAreaId != city+"-d" ||
		rem.ReplacedAreas[0].People != 1 || rem.ReplacedAreas[0].Crews != 2 || rem.ReplacedAreas[0].Plans != 3 ||
		len(rem.RemovedOverrides) != 1 || len(rem.RemovedAreas) != 0 {
		t.Errorf("replacing apply = %v", rem)
	}
	// edit starts from the proposed pack, so Delta (with its replaces) is
	// restated; Epsilon is the one that goes.
	edit(func(p *cities.Pack) {
		p.Areas[2] = cities.Area{ID: city + "-d", Name: cities.Text{"en": "Delta"}, Lat: 35.72, Lng: 139.62, Replaces: []string{city + "-c"}}
		p.ActivityOverrides = nil
	})
	if rem := apply(); !slices.Equal(rem.RemovedAreas, []string{city + "-e"}) || len(rem.ReplacedAreas) != 0 {
		t.Errorf("removing apply = %v", rem)
	}
	if _, err := h.ApplyCityPack(adminCtx, &isshov1.ApplyCityPackRequest{CityId: "nowhere"}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("no such pack: %v", err)
	}
	// Errors: only "not seeded" is NotFound, and Google's text stays out of
	// what the admin sees. Bad input fails the call at once, with no job;
	// Google failing fails the job.
	discover := func(cityID, areaID string) error {
		_, err := h.DiscoverAreaVenues(adminCtx, &isshov1.DiscoverAreaVenuesRequest{CityId: cityID, AreaId: areaID})
		return err
	}
	if err := discover("nowhere", "x"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("discover in an unknown city: %v", err)
	}
	if err := discover(city, "zz-packs-nope"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("discover in an unknown area: %v", err)
	}
	queued := len(jobs.args)
	fm.NearbyErr = errors.New("quota exceeded for key AIza-secret")
	if err := discover(city, city+"-a"); err != nil {
		t.Errorf("discover with Google down is queued: %v", err)
	}
	if f := jobs.out[int64(queued+1)].Failure; f == nil || strings.Contains(f.Message, "AIza") || strings.Contains(f.Message, "quota") {
		t.Errorf("discover job with Google down: %+v", jobs.out[int64(queued+1)])
	}
	fm.NearbyErr = nil
	if len(jobs.args) != queued+1 {
		t.Errorf("bad input queued a job: %d jobs, want %d", len(jobs.args), queued+1)
	}
	// Moving an area makes apply want the matrix; Google being down leaves the pack seeded.
	moved := cities.Pack{}
	if err := json.Unmarshal(repo.content, &moved); err != nil {
		t.Fatal(err)
	}
	// Gamma is gone from the database (replaced above), so keep Delta.
	moved.Areas[2] = cities.Area{ID: city + "-d", Name: cities.Text{"en": "Delta"}, Lat: 35.72, Lng: 139.62, Replaces: []string{city + "-c"}}
	moved.Areas[0].Lat += 0.02
	moved.Areas[0].Boundary, moved.Areas[0].OSMID = nil, 0
	raw, err := cities.EncodePack(moved)
	if err != nil {
		t.Fatal(err)
	}
	repo.files["data/citypacks/zz-packs.json"] = raw
	fm.MatrixErr = errors.New("quota exceeded for key AIza-secret")
	if out := applyJob(); out.Failure == nil || strings.Contains(out.Failure.Message, "AIza") || !strings.Contains(out.Failure.Message, "seeded") ||
		!strings.Contains(out.Failure.Message, "Moved areas: "+city+"-a") ||
		out.ApplyCityPack == nil || len(out.ApplyCityPack.MovedAreas) != 1 {
		t.Errorf("apply with Google down: %+v", out)
	}
	fm.MatrixErr = nil

	// Without a token, only the list works.
	h.Repo = nil
	if _, err := h.GetCityPack(adminCtx, &isshov1.GetCityPackRequest{CityId: city}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("no token: %v", err)
	}
	if res, err := h.ListCityPacks(adminCtx, &isshov1.ListCityPacksRequest{}); err != nil || res.Configured {
		t.Errorf("list without a token: %v", err)
	}
}
