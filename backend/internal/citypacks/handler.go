// Package citypacks serves CityPackService: the city-pack editor in the
// admin SPA (M8). Packs are files in the repository, so the editor proposes
// changes as GitHub pull requests (owner's decision) and, once one is
// merged, applies the default branch's pack to the database and fills in
// what Google provides (travel times, venues) for new or moved areas.
package citypacks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/adminjobs"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	gmaps "github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
)

const (
	// Dir holds the packs in the repository.
	Dir = "data/citypacks"
	// BranchPrefix marks the editor's branches (and so its pull requests).
	BranchPrefix = "citypack/"
)

// Repo is the GitHub repository the packs live in (*github.Client).
type Repo interface {
	File(ctx context.Context, path, branch string) ([]byte, string, error)
	Dir(ctx context.Context, path, branch string) ([]string, error)
	Propose(ctx context.Context, branch, path string, content []byte, sha, message, title, body string) (github.PR, error)
	OpenPRs(ctx context.Context, prefix string) ([]github.PR, error)
}

// Inserter queues jobs (*river.Client).
type Inserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Handler serves CityPackService. cmd/api limits the service to admins
// (auth.AdminOnly); each call checks again (auth.RequireAdmin) so a wiring mistake fails closed.
type Handler struct {
	DB *pgxpool.Pool
	// GitHub is nil without GITHUB_TOKEN; Base is its default branch.
	GitHub Repo
	Base   string
	// Places is nil without GOOGLE_MAPS_API_KEY on the API; the worker (which
	// runs the jobs) has a key of its own.
	Places gmaps.PlaceSearcher
	// Jobs queues the admin jobs (*river.Client, insert-only on the API).
	Jobs Inserter
	Now  func() time.Time // nil = time.Now
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) repo() (Repo, error) {
	if h.GitHub == nil {
		return nil, errs.Refused("the city-pack editor needs GITHUB_TOKEN on the API")
	}
	return h.GitHub, nil
}

// githubError maps a GitHub failure for the caller. GitHub's own message
// (URLs, rate-limit details) stays in the log, not in the response.
func githubError(ctx context.Context, err error) error {
	if errors.Is(err, github.ErrNotFound) {
		return errs.NotFound("no such city pack on the default branch")
	}
	return errs.Upstream("github", err)
}

// Compiled once: check runs on every proposal and apply.
var (
	cityID   = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`)
	currency = regexp.MustCompile(`^[A-Z]{3}$`)
)

func path(id string) string { return Dir + "/" + id + ".json" }

// basePack reads a pack from the default branch: its contents and blob
// sha, or ok false when there's no such file.
func (h *Handler) basePack(ctx context.Context, id string) (p cities.Pack, sha string, ok bool, err error) {
	repo, err := h.repo()
	if err != nil {
		return p, "", false, err
	}
	raw, sha, err := repo.File(ctx, path(id), h.Base)
	if errors.Is(err, github.ErrNotFound) {
		return p, "", false, nil
	}
	if err != nil {
		return p, "", false, githubError(ctx, err)
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, "", false, fmt.Errorf("%s: %w", path(id), err)
	}
	return p, sha, true, nil
}

func (h *Handler) ListCityPacks(ctx context.Context, _ *connect.Request[isshov1.ListCityPacksRequest]) (*connect.Response[isshov1.ListCityPacksResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	out := &isshov1.ListCityPacksResponse{Configured: h.GitHub != nil}
	byID := map[string]*isshov1.CityPackSummary{}
	get := func(id string) *isshov1.CityPackSummary {
		if s, ok := byID[id]; ok {
			return s
		}
		s := &isshov1.CityPackSummary{Id: id, Name: id}
		byID[id] = s
		return s
	}
	if h.GitHub != nil {
		files, err := h.GitHub.Dir(ctx, Dir, h.Base)
		if err != nil {
			return nil, githubError(ctx, err)
		}
		for _, f := range files {
			if id, ok := strings.CutSuffix(f, ".json"); ok {
				get(id).InRepo = true
			}
		}
		prs, err := h.GitHub.OpenPRs(ctx, BranchPrefix)
		if err != nil {
			return nil, githubError(ctx, err)
		}
		for _, p := range prs {
			out.PullRequests = append(out.PullRequests, &isshov1.CityPackPullRequest{Number: int32(p.Number), Title: p.Title, Url: p.URL})
		}
	}
	seeded, err := cities.PackStatuses(ctx, h.DB)
	if err != nil {
		return nil, err
	}
	for _, c := range seeded {
		s := get(c.ID)
		s.Seeded, s.Name, s.Areas = true, c.Name, int32(c.Areas)
		s.AreasWithoutTravelTimes, s.AreasWithoutVenues = c.AreasWithoutTravelTimes, c.AreasWithoutVenues
	}
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		out.Packs = append(out.Packs, byID[id])
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetCityPack(ctx context.Context, req *connect.Request[isshov1.GetCityPackRequest]) (*connect.Response[isshov1.GetCityPackResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	out := &isshov1.GetCityPackResponse{}
	acts, err := cities.CatalogActivities(ctx, h.DB)
	if err != nil {
		return nil, err
	}
	for _, a := range acts {
		out.Activities = append(out.Activities, &isshov1.PackActivity{Id: a.ID, Name: a.Name, Category: a.Category})
	}
	// "" = only the catalogue, for a new city.
	if req.Msg.CityId != "" {
		p, _, ok, err := h.basePack(ctx, req.Msg.CityId)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errs.NotFound("no such city pack on the default branch")
		}
		out.Pack = toProto(p)
	} else if _, err := h.repo(); err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// check validates a pack as CI does, plus what CI can't know: the time zone
// loads and the currency looks like one.
func (h *Handler) check(ctx context.Context, p cities.Pack) error {
	if !cityID.MatchString(p.ID) {
		return errs.Invalid("id: lowercase letters, digits and dashes, starting with a letter")
	}
	acts, err := cities.CatalogActivities(ctx, h.DB)
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, a := range acts {
		ids[a.ID] = true
	}
	if err := cities.ValidatePack(p, ids); err != nil {
		return errs.Invalid("%v", err)
	}
	if _, err := time.LoadLocation(p.Timezone); err != nil || !strings.Contains(p.Timezone, "/") {
		return errs.Invalid("timezone %q: an IANA name like Asia/Tokyo", p.Timezone)
	}
	if !currency.MatchString(p.Currency) {
		return errs.Invalid("currency %q: an ISO 4217 code like JPY", p.Currency)
	}
	return nil
}

func (h *Handler) ProposeCityPack(ctx context.Context, req *connect.Request[isshov1.ProposeCityPackRequest]) (*connect.Response[isshov1.ProposeCityPackResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if req.Msg.Pack == nil {
		return nil, errs.Invalid("pack: required")
	}
	repo, err := h.repo()
	if err != nil {
		return nil, err
	}
	p := fromProto(req.Msg.Pack)
	base, sha, exists, err := h.basePack(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	keepFileOnly(&p, base)
	keepOverrideNames(&p, base)
	if err := h.check(ctx, p); err != nil {
		return nil, err
	}
	var old *cities.Pack
	if exists {
		old = &base
		// Area ids are referenced by profiles, plans and venues: an area
		// can be renamed or moved, never removed (seeding keeps it anyway).
		for _, a := range base.Areas {
			if !slices.ContainsFunc(p.Areas, func(b cities.Area) bool { return b.ID == a.ID }) {
				return nil, errs.Invalid("area %s: areas can't be removed or have their id changed", a.ID)
			}
		}
	}
	changes := Changes(old, p)
	if len(changes) == 0 {
		return nil, errs.Invalid("nothing changed")
	}
	content, err := cities.EncodePack(p)
	if err != nil {
		return nil, err
	}
	name := p.Name["en"]
	title := fmt.Sprintf("feat(data): update the %s city pack", name)
	if !exists {
		title = fmt.Sprintf("feat(data): add the %s city pack", name)
	}
	var body strings.Builder
	// The repository is public: the pull request says what changed, never
	// who proposed it.
	body.WriteString("Proposed from the admin SPA's city-pack editor.\n\n")
	if note := strings.TrimSpace(req.Msg.Note); note != "" {
		fmt.Fprintf(&body, "%s\n\n", note)
	}
	body.WriteString("**Changes**\n")
	for _, ch := range changes {
		fmt.Fprintf(&body, "- %s\n", ch)
	}
	body.WriteString("\n**After merging:** apply the pack on the admin SPA's Cities page (it seeds the database and rebuilds travel times when areas changed), then find venues for new or moved areas.\n")
	branch := fmt.Sprintf("%s%s-%d", BranchPrefix, p.ID, h.now().Unix())
	pr, err := repo.Propose(ctx, branch, path(p.ID), content, sha, title, title, body.String())
	if err != nil {
		return nil, githubError(ctx, err)
	}
	return connect.NewResponse(&isshov1.ProposeCityPackResponse{PullRequest: &isshov1.CityPackPullRequest{
		Number: int32(pr.Number), Title: pr.Title, Url: pr.URL,
	}}), nil
}

// ApplyCityPack validates the pack on the default branch, as before, then
// queues the apply as a job (internal/adminjobs) and returns its id: the
// apply may rebuild a city's travel times, which can outlast a request.
func (h *Handler) ApplyCityPack(ctx context.Context, req *connect.Request[isshov1.ApplyCityPackRequest]) (*connect.Response[isshov1.ApplyCityPackResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	p, _, ok, err := h.basePack(ctx, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errs.NotFound("no such city pack on the default branch")
	}
	if err := h.check(ctx, p); err != nil {
		return nil, err
	}
	// The same pack applied again while one is queued or running is skipped
	// (unique jobs) and the caller follows the first; a changed pack queues
	// behind it.
	args, err := adminjobs.NewApplyCityPackArgs(p)
	if err != nil {
		return nil, err
	}
	id, err := h.insert(ctx, args)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.ApplyCityPackResponse{JobId: id}), nil
}

// DiscoverAreaVenues checks the request (a key, a seeded city, an area in it)
// and queues the search as a job: up to a few hundred Nearby Searches.
func (h *Handler) DiscoverAreaVenues(ctx context.Context, req *connect.Request[isshov1.DiscoverAreaVenuesRequest]) (*connect.Response[isshov1.DiscoverAreaVenuesResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if h.Places == nil {
		return nil, errs.Refused("finding venues needs GOOGLE_MAPS_API_KEY")
	}
	areas, _, err := geo.LoadCity(ctx, dbgen.New(h.DB), req.Msg.CityId)
	switch {
	case errors.Is(err, geo.ErrCityNotFound):
		// Only "not seeded" is NotFound: a database failure must not look like it.
		return nil, errs.NotFound("no such city")
	case err != nil:
		return nil, err
	case !slices.ContainsFunc(areas, func(a geo.Area) bool { return a.ID == req.Msg.AreaId }):
		return nil, errs.Invalid("area_id: not in that city")
	}
	id, err := h.insert(ctx, adminjobs.DiscoverAreaVenuesArgs{City: req.Msg.CityId, Area: req.Msg.AreaId})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.DiscoverAreaVenuesResponse{JobId: id}), nil
}

// insert queues a job and returns its id, or the id of the identical job
// already queued or running.
func (h *Handler) insert(ctx context.Context, args river.JobArgs) (int64, error) {
	if h.Jobs == nil {
		return 0, errs.Refused("the admin jobs are not set up on this API")
	}
	res, err := h.Jobs.Insert(ctx, args, nil)
	if err != nil {
		return 0, err
	}
	return res.Job.ID, nil
}

// Changes describes a pack's changes from the default branch's (nil for a
// new city), in words for the pull request; none = nothing changed.
func Changes(old *cities.Pack, p cities.Pack) []string {
	if old == nil {
		return []string{fmt.Sprintf("New city %s (%s): %d areas, languages %s, %s, %s",
			p.Name["en"], p.ID, len(p.Areas), strings.Join(p.Languages, ", "), p.Timezone, p.Currency)}
	}
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	if !maps.Equal(old.Name, p.Name) {
		add("Name: %v → %v", old.Name, p.Name)
	}
	if old.Timezone != p.Timezone {
		add("Time zone: %s → %s", old.Timezone, p.Timezone)
	}
	if old.Currency != p.Currency {
		add("Currency: %s → %s", old.Currency, p.Currency)
	}
	if !slices.Equal(old.Languages, p.Languages) {
		add("Languages: %s → %s", strings.Join(old.Languages, ", "), strings.Join(p.Languages, ", "))
	}
	if !slices.Equal(old.TravelModes, p.TravelModes) {
		add("Travel modes: %s → %s", strings.Join(old.TravelModes, ", "), strings.Join(p.TravelModes, ", "))
	}
	if old.Comment != p.Comment {
		add("Comment changed")
	}
	oldAreas := map[string]cities.Area{}
	for _, a := range old.Areas {
		oldAreas[a.ID] = a
	}
	for _, a := range p.Areas {
		o, ok := oldAreas[a.ID]
		switch {
		case !ok:
			add("New area %s (%s) at %.5f, %.5f", a.Name["en"], a.ID, a.Lat, a.Lng)
		default:
			if !maps.Equal(o.Name, a.Name) {
				add("Area %s renamed: %v → %v", a.ID, o.Name, a.Name)
			}
			if d := gmaps.DistanceM(gmaps.LatLng{Lat: o.Lat, Lng: o.Lng}, gmaps.LatLng{Lat: a.Lat, Lng: a.Lng}); d > 0 {
				add("Area %s moved %.0f m", a.ID, d)
			}
		}
	}
	oldOv := map[string]cities.ActivityOverride{}
	for _, o := range old.ActivityOverrides {
		oldOv[o.ActivityID] = o
	}
	seen := map[string]bool{}
	for _, o := range p.ActivityOverrides {
		seen[o.ActivityID] = true
		was, ok := oldOv[o.ActivityID]
		if !ok || was.Disabled != o.Disabled || !maps.Equal(was.Name, o.Name) || !slices.Equal(was.Months, o.Months) {
			add("Activity %s: %s", o.ActivityID, describe(o))
		}
	}
	for _, o := range old.ActivityOverrides {
		if !seen[o.ActivityID] {
			add("Activity %s: back to the catalogue's defaults", o.ActivityID)
		}
	}
	return out
}

func describe(o cities.ActivityOverride) string {
	if o.Disabled {
		return "not offered"
	}
	var parts []string
	if o.Name["en"] != "" {
		parts = append(parts, fmt.Sprintf("called %q", o.Name["en"]))
	}
	if len(o.Months) > 0 {
		ms := make([]string, len(o.Months))
		for i, m := range o.Months {
			ms[i] = time.Month(m).String()[:3]
		}
		parts = append(parts, "in season "+strings.Join(ms, ", "))
	}
	if len(parts) == 0 {
		return "offered, as in the catalogue"
	}
	return strings.Join(parts, ", ")
}

func toProto(p cities.Pack) *isshov1.CityPack {
	out := &isshov1.CityPack{
		Id: p.ID, Comment: p.Comment, Name: p.Name, Timezone: p.Timezone, Currency: p.Currency,
		Languages: p.Languages, TravelModes: p.TravelModes,
	}
	for _, a := range p.Areas {
		out.Areas = append(out.Areas, &isshov1.PackArea{Id: a.ID, Name: a.Name, Lat: a.Lat, Lng: a.Lng})
	}
	for _, o := range p.ActivityOverrides {
		po := &isshov1.PackOverride{ActivityId: o.ActivityID, Disabled: o.Disabled, Name: o.Name["en"]}
		for _, m := range o.Months {
			po.Months = append(po.Months, int32(m))
		}
		out.ActivityOverrides = append(out.ActivityOverrides, po)
	}
	return out
}

// keepFileOnly copies the fields the editor doesn't show from the default
// branch's pack, or its pull requests would drop them: each area's osm_id and
// boundary (`tool area-boundaries` writes them) and replaces (written by hand
// when an area is retired). An area moved out of its boundary then fails
// validation, which is the point.
func keepFileOnly(p *cities.Pack, base cities.Pack) {
	for i, a := range p.Areas {
		if j := slices.IndexFunc(base.Areas, func(b cities.Area) bool { return b.ID == a.ID }); j >= 0 {
			p.Areas[i].OSMID, p.Areas[i].Boundary, p.Areas[i].Replaces = base.Areas[j].OSMID, base.Areas[j].Boundary, base.Areas[j].Replaces
		}
	}
}

// keepOverrideNames carries an override's non-English names over from the
// default branch's pack: the editor shows and sends only the English one
// (PackOverride.name), so saving would otherwise drop e.g. a Japanese name.
// They are kept only while the override still has an English name; clearing
// it means the admin is removing the custom name, and an English-less name
// would fail validation. The editor's English name always wins.
func keepOverrideNames(p *cities.Pack, base cities.Pack) {
	for i, o := range p.ActivityOverrides {
		if o.Name["en"] == "" {
			continue
		}
		j := slices.IndexFunc(base.ActivityOverrides, func(b cities.ActivityOverride) bool { return b.ActivityID == o.ActivityID })
		if j < 0 {
			continue
		}
		for lang, name := range base.ActivityOverrides[j].Name {
			if _, ok := o.Name[lang]; !ok {
				p.ActivityOverrides[i].Name[lang] = name
			}
		}
	}
}

// fromProto builds a pack from the editor's, trimming text; empty names
// are dropped.
func fromProto(p *isshov1.CityPack) cities.Pack {
	text := func(m map[string]string) cities.Text {
		t := cities.Text{}
		for k, v := range m {
			if v = strings.TrimSpace(v); v != "" {
				t[k] = v
			}
		}
		return t
	}
	out := cities.Pack{
		ID: strings.TrimSpace(p.Id), Comment: p.Comment, Name: text(p.Name), Timezone: strings.TrimSpace(p.Timezone),
		Currency: strings.ToUpper(strings.TrimSpace(p.Currency)), Languages: p.Languages, TravelModes: p.TravelModes,
	}
	for _, a := range p.Areas {
		out.Areas = append(out.Areas, cities.Area{ID: strings.TrimSpace(a.Id), Name: text(a.Name), Lat: a.Lat, Lng: a.Lng})
	}
	for _, o := range p.ActivityOverrides {
		ov := cities.ActivityOverride{ActivityID: o.ActivityId, Disabled: o.Disabled}
		if n := strings.TrimSpace(o.Name); n != "" {
			ov.Name = cities.Text{"en": n}
		}
		for _, m := range o.Months {
			ov.Months = append(ov.Months, int16(m))
		}
		out.ActivityOverrides = append(out.ActivityOverrides, ov)
	}
	return out
}
