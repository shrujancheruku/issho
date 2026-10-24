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

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	gmaps "github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
)

const (
	// Dir holds the packs in the repository.
	Dir = "data/citypacks"
	// BranchPrefix marks the editor's branches (and so its pull requests).
	BranchPrefix = "citypack/"
	// MovedM is how far an area's centre must move for it to need venues
	// and travel times again.
	MovedM = 100
)

// Repo is the GitHub repository the packs live in (*github.Client).
type Repo interface {
	File(ctx context.Context, path, branch string) ([]byte, string, error)
	Dir(ctx context.Context, path, branch string) ([]string, error)
	Propose(ctx context.Context, branch, path string, content []byte, sha, message, title, body string) (github.PR, error)
	OpenPRs(ctx context.Context, prefix string) ([]github.PR, error)
}

// Handler serves CityPackService. cmd/api limits the service to admins
// (auth.AdminOnly); admin() checks again so a wiring mistake fails closed.
type Handler struct {
	DB *pgxpool.Pool
	// GitHub is nil without GITHUB_TOKEN; Base is its default branch.
	GitHub Repo
	Base   string
	// Router and Places are nil without GOOGLE_MAPS_API_KEY.
	Router gmaps.Router
	Places gmaps.PlaceSearcher
	Now    func() time.Time // nil = time.Now
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func admin(ctx context.Context) (auth.Caller, error) {
	c, ok := auth.FromContext(ctx)
	if !ok {
		return c, connect.NewError(connect.CodeUnauthenticated, errors.New("no caller"))
	}
	if !c.Admin {
		return c, connect.NewError(connect.CodePermissionDenied, errors.New("admins only"))
	}
	return c, nil
}

func invalid(format string, a ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, a...))
}

func (h *Handler) repo() (Repo, error) {
	if h.GitHub == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the city-pack editor needs GITHUB_TOKEN on the API"))
	}
	return h.GitHub, nil
}

func githubError(err error) error {
	if errors.Is(err, github.ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("no such city pack on "+"the default branch"))
	}
	return connect.NewError(connect.CodeUnavailable, err)
}

var cityID = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`)

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
		return p, "", false, githubError(err)
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, "", false, fmt.Errorf("%s: %w", path(id), err)
	}
	return p, sha, true, nil
}

func (h *Handler) ListCityPacks(ctx context.Context, _ *connect.Request[isshov1.ListCityPacksRequest]) (*connect.Response[isshov1.ListCityPacksResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := dbgen.New(h.DB)
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
			return nil, githubError(err)
		}
		for _, f := range files {
			if id, ok := strings.CutSuffix(f, ".json"); ok {
				get(id).InRepo = true
			}
		}
		prs, err := h.GitHub.OpenPRs(ctx, BranchPrefix)
		if err != nil {
			return nil, githubError(err)
		}
		for _, p := range prs {
			out.PullRequests = append(out.PullRequests, &isshov1.CityPackPullRequest{Number: int32(p.Number), Title: p.Title, Url: p.URL})
		}
	}
	rows, err := q.ListCities(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		s := get(r.ID)
		s.Seeded = true
		var name cities.Text
		if json.Unmarshal(r.Name, &name) == nil && name["en"] != "" {
			s.Name = name["en"]
		}
	}
	areas, err := q.ListCityPackAreas(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range areas {
		s := get(a.CityID)
		s.Areas++
		if !a.HasTravelTimes {
			s.AreasWithoutTravelTimes = append(s.AreasWithoutTravelTimes, a.ID)
		}
		if !a.HasVenues {
			s.AreasWithoutVenues = append(s.AreasWithoutVenues, a.ID)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		out.Packs = append(out.Packs, byID[id])
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetCityPack(ctx context.Context, req *connect.Request[isshov1.GetCityPackRequest]) (*connect.Response[isshov1.GetCityPackResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	out := &isshov1.GetCityPackResponse{}
	acts, err := dbgen.New(h.DB).ListCatalogActivities(ctx)
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
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such city pack on the default branch"))
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
		return invalid("id: lowercase letters, digits and dashes, starting with a letter")
	}
	acts, err := dbgen.New(h.DB).ListCatalogActivities(ctx)
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, a := range acts {
		ids[a.ID] = true
	}
	if err := cities.ValidatePack(p, ids); err != nil {
		return invalid("%v", err)
	}
	if _, err := time.LoadLocation(p.Timezone); err != nil || !strings.Contains(p.Timezone, "/") {
		return invalid("timezone %q: an IANA name like Asia/Tokyo", p.Timezone)
	}
	if !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(p.Currency) {
		return invalid("currency %q: an ISO 4217 code like JPY", p.Currency)
	}
	return nil
}

func (h *Handler) ProposeCityPack(ctx context.Context, req *connect.Request[isshov1.ProposeCityPackRequest]) (*connect.Response[isshov1.ProposeCityPackResponse], error) {
	c, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.Pack == nil {
		return nil, invalid("pack: required")
	}
	repo, err := h.repo()
	if err != nil {
		return nil, err
	}
	p := fromProto(req.Msg.Pack)
	if err := h.check(ctx, p); err != nil {
		return nil, err
	}
	base, sha, exists, err := h.basePack(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	var old *cities.Pack
	if exists {
		old = &base
		// Area ids are referenced by profiles, plans and venues: an area
		// can be renamed or moved, never removed (seeding keeps it anyway).
		for _, a := range base.Areas {
			if !slices.ContainsFunc(p.Areas, func(b cities.Area) bool { return b.ID == a.ID }) {
				return nil, invalid("area %s: areas can't be removed or have their id changed", a.ID)
			}
		}
	}
	changes := Changes(old, p)
	if len(changes) == 0 {
		return nil, invalid("nothing changed")
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
	fmt.Fprintf(&body, "Proposed from the admin SPA's city-pack editor by %s.\n\n", c.Email)
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
		return nil, githubError(err)
	}
	return connect.NewResponse(&isshov1.ProposeCityPackResponse{PullRequest: &isshov1.CityPackPullRequest{
		Number: int32(pr.Number), Title: pr.Title, Url: pr.URL,
	}}), nil
}

func (h *Handler) ApplyCityPack(ctx context.Context, req *connect.Request[isshov1.ApplyCityPackRequest]) (*connect.Response[isshov1.ApplyCityPackResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	p, _, ok, err := h.basePack(ctx, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such city pack on the default branch"))
	}
	if err := h.check(ctx, p); err != nil {
		return nil, err
	}
	q := dbgen.New(h.DB)
	before, err := q.ListCityAreas(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	was := map[string]gmaps.LatLng{}
	for _, a := range before {
		was[a.ID] = gmaps.LatLng{Lat: a.Lat, Lng: a.Lng}
	}
	if err := cities.SeedPackData(ctx, h.DB, p); err != nil {
		return nil, err
	}
	out := &isshov1.ApplyCityPackResponse{}
	for _, a := range p.Areas {
		at, ok := was[a.ID]
		switch {
		case !ok:
			out.NewAreas = append(out.NewAreas, a.ID)
		case gmaps.DistanceM(at, gmaps.LatLng{Lat: a.Lat, Lng: a.Lng}) > MovedM:
			out.MovedAreas = append(out.MovedAreas, a.ID)
		}
	}
	if (len(out.NewAreas) > 0 || len(out.MovedAreas) > 0) && h.Router != nil {
		areas, modes, err := geo.LoadCity(ctx, q, p.ID)
		if err != nil {
			return nil, err
		}
		rows, err := geo.BuildMatrix(ctx, h.Router, areas, modes)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("seeded, but Google's travel times failed (the worker retries within a day): %w", err))
		}
		if _, err := geo.SaveMatrix(ctx, h.DB, p.ID, rows); err != nil {
			return nil, err
		}
		out.TravelTimesRebuilt, out.TravelTimes = true, int32(len(rows))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) DiscoverAreaVenues(ctx context.Context, req *connect.Request[isshov1.DiscoverAreaVenuesRequest]) (*connect.Response[isshov1.DiscoverAreaVenuesResponse], error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if h.Places == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("finding venues needs GOOGLE_MAPS_API_KEY"))
	}
	m := req.Msg
	q := dbgen.New(h.DB)
	areas, _, err := geo.LoadCity(ctx, q, m.CityId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such city"))
	}
	if !slices.ContainsFunc(areas, func(a geo.Area) bool { return a.ID == m.AreaId }) {
		return nil, invalid("area_id: not in that city")
	}
	cats, err := geo.LoadVenueCategories(ctx, q, m.CityId, nil)
	if err != nil {
		return nil, err
	}
	n, err := q.CountCityVenues(ctx, m.CityId)
	if err != nil {
		return nil, err
	}
	langs, err := q.GetCityLanguages(ctx, m.CityId)
	if err != nil || len(langs) == 0 {
		return nil, fmt.Errorf("city languages: %v", err)
	}
	// A new city has no city-wide venues yet (beaches, trails): find them
	// with its first area.
	venues, searches, err := geo.DiscoverAreaVenues(ctx, h.Places, areas, m.AreaId, cats, n == 0, langs[0])
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("Google search failed: %w", err))
	}
	if err := geo.SaveVenues(ctx, h.DB, m.CityId, venues); err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.DiscoverAreaVenuesResponse{Searches: int32(searches), Venues: int32(len(venues))}), nil
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
