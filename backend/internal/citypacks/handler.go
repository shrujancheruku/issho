// Package citypacks serves CityPackService: the city-pack editor in the
// admin SPA (M8). Packs are files in the repository, so the editor saves
// changes in a city's pending change (internal/repoedit): one rolling pull
// request per city, which the owner merges (owner's decision). Once it is
// merged, the default branch's pack is applied to the database and Google
// fills in what it provides (travel times, venues) for new or moved areas.
package citypacks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

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
	"github.com/shrujancheruku/issho/backend/internal/repoedit"
)

const (
	// Dir holds the packs in the repository.
	Dir = "data/citypacks"
	// Kind is the editor's pending-change kind: one pull request per city.
	Kind = "citypack"
)

// Inserter queues jobs (*river.Client).
type Inserter interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Handler serves CityPackService. cmd/api limits the service to admins
// (auth.AdminOnly); each call checks again (auth.RequireAdmin) so a wiring mistake fails closed.
type Handler struct {
	DB *pgxpool.Pool
	// Repo is nil without GITHUB_TOKEN.
	Repo *repoedit.Repo
	// Places is nil without GOOGLE_MAPS_API_KEY on the API; the worker (which
	// runs the jobs) has a key of its own.
	Places gmaps.PlaceSearcher
	// Jobs queues the admin jobs (*river.Client, insert-only on the API).
	Jobs Inserter
}

func path(id string) string { return Dir + "/" + id + ".json" }

// parse reads a pack file's bytes.
func parse(id string, raw []byte) (p cities.Pack, err error) {
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("%s: %w", path(id), err)
	}
	return p, nil
}

// basePack reads a pack from the default branch, or ok false when there's
// no such file.
func (h *Handler) basePack(ctx context.Context, id string) (p cities.Pack, ok bool, err error) {
	raw, ok, err := h.Repo.Read(ctx, path(id))
	if err != nil || !ok {
		return p, false, err
	}
	p, err = parse(id, raw)
	return p, err == nil, err
}

func prProto(pr repoedit.PR) *isshov1.CityPackPullRequest {
	return &isshov1.CityPackPullRequest{Number: int32(pr.Number), Title: pr.Title, Url: pr.URL}
}

func (h *Handler) ListCityPacks(ctx context.Context, _ *connect.Request[isshov1.ListCityPacksRequest]) (*connect.Response[isshov1.ListCityPacksResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	out := &isshov1.ListCityPacksResponse{Configured: h.Repo != nil}
	byID := map[string]*isshov1.CityPackSummary{}
	get := func(id string) *isshov1.CityPackSummary {
		if s, ok := byID[id]; ok {
			return s
		}
		s := &isshov1.CityPackSummary{Id: id, Name: id}
		byID[id] = s
		return s
	}
	if h.Repo != nil {
		files, err := h.Repo.List(ctx, Dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if id, ok := strings.CutSuffix(f, ".json"); ok {
				get(id).InRepo = true
			}
		}
		prs, err := h.Repo.Pending(ctx, Kind)
		if err != nil {
			return nil, err
		}
		for _, id := range slices.Sorted(maps.Keys(prs)) {
			out.PullRequests = append(out.PullRequests, prProto(prs[id]))
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
	if req.Msg.CityId == "" {
		if h.Repo == nil {
			return nil, errs.Refused("the city-pack editor needs GITHUB_TOKEN on the API")
		}
		return connect.NewResponse(out), nil
	}
	// The city's pending change when it has one, so the editor continues it
	// rather than starting from the default branch's pack.
	files, pr, err := h.Repo.Files(ctx, Kind, req.Msg.CityId, []string{path(req.Msg.CityId)})
	if err != nil {
		return nil, err
	}
	raw, ok := files[path(req.Msg.CityId)]
	if !ok {
		return nil, errs.NotFound("no such city pack")
	}
	p, err := parse(req.Msg.CityId, raw)
	if err != nil {
		return nil, err
	}
	out.Pack = toProto(p)
	if pr != nil {
		out.PullRequest = prProto(*pr)
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ProposeCityPack(ctx context.Context, req *connect.Request[isshov1.ProposeCityPackRequest]) (*connect.Response[isshov1.ProposeCityPackResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if req.Msg.Pack == nil {
		return nil, errs.Invalid("pack: required")
	}
	p := fromProto(req.Msg.Pack)
	// The checks (areas can't be removed, what the editor doesn't show is
	// kept) are against the default branch's pack, not the pending one.
	base, exists, err := h.basePack(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	var old *cities.Pack
	if exists {
		old = &base
	}
	prop, err := cities.Propose(ctx, h.DB, p, old, req.Msg.Note)
	if err != nil {
		return nil, err
	}
	file := path(prop.Pack.ID)
	// Rolling: this commits to the city's open pull request and rewrites its
	// body (the whole change from the default branch), or opens one.
	pr, ok, err := h.Repo.Change(ctx, repoedit.Edit{
		Kind: Kind, Key: prop.Pack.ID, Paths: []string{file}, Message: prop.Title, Title: prop.Title,
	}, func(files map[string][]byte) (bool, string, error) {
		files[file] = prop.Content
		return true, prop.Body, nil
	})
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errs.Invalid("nothing changed")
	}
	return connect.NewResponse(&isshov1.ProposeCityPackResponse{PullRequest: prProto(pr)}), nil
}

// ApplyCityPack validates the pack on the default branch, as before, then
// queues the apply as a job (internal/adminjobs) and returns its id: the
// apply may rebuild a city's travel times, which can outlast a request.
func (h *Handler) ApplyCityPack(ctx context.Context, req *connect.Request[isshov1.ApplyCityPackRequest]) (*connect.Response[isshov1.ApplyCityPackResponse], error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	p, ok, err := h.basePack(ctx, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errs.NotFound("no such city pack on the default branch")
	}
	if err := cities.CheckPack(ctx, h.DB, p); err != nil {
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
