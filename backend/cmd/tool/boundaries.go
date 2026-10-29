package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// nominatimLookup is OpenStreetMap's Nominatim, which returns an object's
// outline by id. Its usage policy asks for an identifying User-Agent, at most
// one request a second and no bulk downloads: a pack is one or a few calls.
const nominatimLookup = "https://nominatim.openstreetmap.org/lookup"

// areaBoundaries fills each area's boundary in a city pack from its osm_id
// (an OpenStreetMap relation) and rewrites the file. Boundaries are OSM data:
// © OpenStreetMap contributors, ODbL (data/README.md).
func areaBoundaries(args []string) error {
	fs := flag.NewFlagSet("area-boundaries", flag.ExitOnError)
	// Nominatim simplifies the outline to this tolerance, in degrees: 0.0002
	// is about 20 m, plenty for placing venues, and keeps a ward to a few
	// hundred bytes.
	threshold := fs.Float64("threshold", 0.0002, "simplification tolerance in degrees")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("usage: tool area-boundaries [-threshold 0.0002] <citypack.json>")
	}
	path := fs.Arg(0)
	p, err := cities.ReadPack(path)
	if err != nil {
		return err
	}
	var ids []string
	for _, a := range p.Areas {
		if a.OSMID > 0 {
			ids = append(ids, "R"+strconv.FormatInt(a.OSMID, 10))
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("%s: no area has an osm_id", path)
	}
	got := map[int64]maps.Boundary{}
	ctx := context.Background()
	// Nominatim takes up to 50 ids a call.
	for i := 0; i < len(ids); i += 50 {
		if i > 0 {
			time.Sleep(time.Second)
		}
		b, err := lookupBoundaries(ctx, ids[i:min(i+50, len(ids))], *threshold)
		if err != nil {
			return err
		}
		for id, v := range b {
			got[id] = v
		}
	}
	for i, a := range p.Areas {
		if a.OSMID == 0 {
			continue
		}
		b, ok := got[a.OSMID]
		if !ok {
			return fmt.Errorf("area %s: OpenStreetMap has no boundary for relation %d", a.ID, a.OSMID)
		}
		p.Areas[i].Boundary = b.Encode()
	}
	// Same checks as CI, so a boundary that doesn't hold its point (or holds
	// another area's) is caught before the file changes.
	if err := cities.ValidatePack(p, nil); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	out, err := cities.EncodePack(p)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	slog.Info("boundaries written", "pack", path, "areas", len(got), "bytes", len(out))
	return nil
}

func lookupBoundaries(ctx context.Context, ids []string, threshold float64) (map[int64]maps.Boundary, error) {
	q := url.Values{
		"osm_ids": {strings.Join(ids, ",")}, "format": {"json"},
		"polygon_geojson": {"1"}, "polygon_threshold": {strconv.FormatFloat(threshold, 'f', -1, 64)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nominatimLookup+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "issho-tool (https://github.com/shrujancheruku/issho)")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim: %s", res.Status)
	}
	var places []struct {
		OSMType string          `json:"osm_type"`
		OSMID   int64           `json:"osm_id"`
		GeoJSON json.RawMessage `json:"geojson"`
	}
	if err := json.NewDecoder(res.Body).Decode(&places); err != nil {
		return nil, fmt.Errorf("nominatim: %w", err)
	}
	out := map[int64]maps.Boundary{}
	for _, pl := range places {
		if pl.OSMType != "relation" {
			continue
		}
		b, err := outerRings(pl.GeoJSON)
		if err != nil {
			return nil, fmt.Errorf("relation %d: %w", pl.OSMID, err)
		}
		out[pl.OSMID] = b
	}
	return out, nil
}

// outerRings reads a GeoJSON Polygon or MultiPolygon's outer rings, without
// the closing point (maps.Boundary closes rings itself).
func outerRings(raw json.RawMessage) (maps.Boundary, error) {
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, err
	}
	var polys [][][][2]float64
	switch g.Type {
	case "Polygon":
		var p [][][2]float64
		if err := json.Unmarshal(g.Coordinates, &p); err != nil {
			return nil, err
		}
		polys = [][][][2]float64{p}
	case "MultiPolygon":
		if err := json.Unmarshal(g.Coordinates, &polys); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("boundary is a %s, not a polygon", g.Type)
	}
	var b maps.Boundary
	for _, p := range polys {
		if len(p) == 0 || len(p[0]) < 4 {
			continue
		}
		r := p[0][:len(p[0])-1]
		ring := make([]maps.LatLng, len(r))
		for i, c := range r {
			ring[i] = maps.LatLng{Lat: c[1], Lng: c[0]}
		}
		b = append(b, ring)
	}
	if len(b) == 0 {
		return nil, errors.New("no rings")
	}
	return b, nil
}
