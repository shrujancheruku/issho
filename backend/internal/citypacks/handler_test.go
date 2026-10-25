package citypacks

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	gmaps "github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
)

func TestChanges(t *testing.T) {
	old := cities.Pack{
		ID: "x", Name: cities.Text{"en": "X"}, Timezone: "Asia/Tokyo", Currency: "JPY", Languages: []string{"ja"}, TravelModes: []string{"walk"},
		Areas:             []cities.Area{{ID: "x-a", Name: cities.Text{"en": "A"}, Lat: 35, Lng: 139}},
		ActivityOverrides: []cities.ActivityOverride{{ActivityID: "hanami", Months: []int16{3, 4}}, {ActivityID: "surf", Disabled: true}},
	}
	if got := Changes(&old, old); len(got) != 0 {
		t.Errorf("no change: %v", got)
	}
	p := old
	p.Languages = []string{"ja", "en"}
	p.Areas = []cities.Area{{ID: "x-a", Name: cities.Text{"en": "A2"}, Lat: 35.001, Lng: 139}, {ID: "x-b", Name: cities.Text{"en": "B"}, Lat: 35.1, Lng: 139.1}}
	p.ActivityOverrides = []cities.ActivityOverride{{ActivityID: "hanami", Months: []int16{4}}, {ActivityID: "karaoke", Name: cities.Text{"en": "Sing"}}}
	want := []string{
		"Languages: ja → ja, en",
		"Area x-a renamed: map[en:A] → map[en:A2]",
		"Area x-a moved 111 m",
		"New area B (x-b) at 35.10000, 139.10000",
		"Activity hanami: in season Apr",
		`Activity karaoke: called "Sing"`,
		"Activity surf: back to the catalogue's defaults",
	}
	got := Changes(&old, p)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("changes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := Changes(nil, p); len(got) != 1 || !strings.HasPrefix(got[0], "New city X (x): 2 areas") {
		t.Errorf("new city: %v", got)
	}
}

// fakeRepo is a repository with files on main and records proposals.
type fakeRepo struct {
	files    map[string][]byte
	proposed []string // branch, path, message, body per proposal, joined
	content  []byte
	sha      string
}

func (f *fakeRepo) File(_ context.Context, path, branch string) ([]byte, string, error) {
	b, ok := f.files[path]
	if !ok || branch != "main" {
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

func (f *fakeRepo) Propose(_ context.Context, branch, path string, content []byte, sha, message, title, body string) (github.PR, error) {
	f.proposed = append(f.proposed, strings.Join([]string{branch, path, message, body}, "|"))
	f.content, f.sha = content, sha
	return github.PR{Number: 42, Title: title, URL: "https://github.com/o/r/pull/42", Branch: branch}, nil
}

func (f *fakeRepo) OpenPRs(context.Context, string) ([]github.PR, error) {
	return []github.PR{{Number: 42, Title: "t", URL: "u"}}, nil
}

// fakeMaps: every pair 10 minutes apart; one cafe found near any search.
type fakeMaps struct{ matrix, nearby int }

func (f *fakeMaps) Matrix(_ context.Context, r gmaps.MatrixRequest) ([]gmaps.MatrixElement, error) {
	f.matrix++
	var out []gmaps.MatrixElement
	for i := range r.Origins {
		for j := range r.Destinations {
			out = append(out, gmaps.MatrixElement{Origin: i, Destination: j, Duration: 10 * time.Minute, OK: true})
		}
	}
	return out, nil
}

func (f *fakeMaps) Nearby(_ context.Context, r gmaps.NearbyRequest) ([]gmaps.Place, error) {
	f.nearby++
	return []gmaps.Place{{ID: "zz-packs-place-" + r.Types[0], PrimaryType: r.Types[0], Location: r.Center}}, nil
}

// TestIntegrationCityPacks: proposing a new city and a change to it as pull
// requests (with refusals), applying the merged pack (areas new and moved,
// travel times rebuilt) and finding an area's venues. Skipped unless
// TEST_DATABASE_URL is set.
func TestIntegrationCityPacks(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
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
	repo := &fakeRepo{files: map[string][]byte{"data/citypacks/other.json": []byte(`{"id":"other"}`)}}
	fm := &fakeMaps{}
	h := &Handler{DB: pool, GitHub: repo, Base: "main", Router: fm, Places: fm, Now: func() time.Time { return time.Unix(1900000000, 0) }}
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
		res, err := h.ProposeCityPack(adminCtx, connect.NewRequest(&isshov1.ProposeCityPackRequest{Pack: p, Note: "Launching soon."}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
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

	// A new city: created as a new file on a new branch.
	res, err := propose(pack)
	if err != nil {
		t.Fatal(err)
	}
	if res.PullRequest.Number != 42 || repo.sha != "" {
		t.Errorf("new city: %v (sha %q)", res.PullRequest, repo.sha)
	}
	parts := strings.Split(repo.proposed[0], "|")
	if parts[0] != "citypack/zz-packs-1900000000" || parts[1] != "data/citypacks/zz-packs.json" || parts[2] != "feat(data): add the Packs city pack" ||
		strings.Contains(parts[3], "admin@example.com") || !strings.Contains(parts[3], "Launching soon.") || !strings.Contains(parts[3], "New city Packs") {
		t.Errorf("proposal = %q", parts)
	}
	var written cities.Pack
	if err := json.Unmarshal(repo.content, &written); err != nil || written.Currency != "JPY" || written.ActivityOverrides[0].Name["en"] != "Kissa" {
		t.Errorf("written = %+v, %v", written, err)
	}

	// Merged: apply it. Both areas are new; travel times are built.
	repo.files["data/citypacks/zz-packs.json"] = repo.content
	apply := func() *isshov1.ApplyCityPackResponse {
		t.Helper()
		res, err := h.ApplyCityPack(adminCtx, connect.NewRequest(&isshov1.ApplyCityPackRequest{CityId: city}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg
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
	disc, err := h.DiscoverAreaVenues(adminCtx, connect.NewRequest(&isshov1.DiscoverAreaVenuesRequest{CityId: city, AreaId: city + "-a"}))
	if err != nil || disc.Msg.Searches < 1 || disc.Msg.Venues != disc.Msg.Searches || fm.nearby != int(disc.Msg.Searches) {
		t.Fatalf("discover = %v, %v", disc, err)
	}
	list, err := h.ListCityPacks(adminCtx, connect.NewRequest(&isshov1.ListCityPacksRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list.Msg.Packs {
		// Alpha has venues now (Beta may too: city-wide finds go to the
		// nearest area).
		if p.Id == city && (!p.InRepo || !p.Seeded || p.Name != "Packs" || p.Areas != 2 || len(p.AreasWithoutTravelTimes) != 0 ||
			slices.Contains(p.AreasWithoutVenues, city+"-a")) {
			t.Errorf("listed = %v", p)
		}
	}

	// A change: refused when it removes an area or changes nothing.
	got, err := h.GetCityPack(adminCtx, connect.NewRequest(&isshov1.GetCityPackRequest{CityId: city}))
	if err != nil {
		t.Fatal(err)
	}
	p := got.Msg.Pack
	if _, err := propose(p); connect.CodeOf(err) != connect.CodeInvalidArgument {
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
	parts = strings.Split(repo.proposed[len(repo.proposed)-1], "|")
	if parts[2] != "feat(data): update the Packs city pack" || repo.sha != "sha-data/citypacks/zz-packs.json" ||
		!strings.Contains(parts[3], "Area zz-packs-b moved 1001 m") || !strings.Contains(parts[3], "New area Gamma") {
		t.Errorf("change proposal = %q (sha %q)", parts, repo.sha)
	}
	repo.files["data/citypacks/zz-packs.json"] = repo.content
	before := fm.matrix
	if a := apply(); len(a.NewAreas) != 1 || a.NewAreas[0] != city+"-c" || len(a.MovedAreas) != 1 || a.MovedAreas[0] != city+"-b" || fm.matrix == before {
		t.Errorf("second apply = %v", a)
	}
	// Applying again changes nothing and calls nobody.
	before = fm.matrix
	if a := apply(); len(a.NewAreas)+len(a.MovedAreas) != 0 || a.TravelTimesRebuilt || fm.matrix != before {
		t.Errorf("third apply = %v", a)
	}
	if _, err := h.ApplyCityPack(adminCtx, connect.NewRequest(&isshov1.ApplyCityPackRequest{CityId: "nowhere"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("no such pack: %v", err)
	}
	// Without a token, only the list works.
	h.GitHub = nil
	if _, err := h.GetCityPack(adminCtx, connect.NewRequest(&isshov1.GetCityPackRequest{CityId: city})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("no token: %v", err)
	}
	if res, err := h.ListCityPacks(adminCtx, connect.NewRequest(&isshov1.ListCityPacksRequest{})); err != nil || res.Msg.Configured {
		t.Errorf("list without a token: %v", err)
	}
}
