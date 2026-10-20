package geo

import (
	"context"
	"fmt"
	"slices"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// WideRadiusM marks categories searched once per city instead of once per
// area. With a 30 km radius every area would return the same beaches and
// trails, paying for the same results over and over.
const WideRadiusM = 10000

type VenueCategory struct {
	ID          string
	GoogleTypes []string
	RadiusM     int32
}

// VenueJob is one Nearby Search call.
type VenueJob struct {
	Category VenueCategory
	Center   maps.LatLng
	From     string // area id, or "" for a city-wide search
}

// Venue is a discovered place assigned to the nearest area.
type Venue struct {
	PlaceID    string
	AreaID     string
	CategoryID string
}

// PlanVenues lists the Nearby Search calls for a city: one per area and
// category, except wide categories, which get one search from the city centre.
func PlanVenues(areas []Area, cats []VenueCategory) []VenueJob {
	var jobs []VenueJob
	centre := Centre(areas)
	for _, c := range cats {
		if c.RadiusM >= WideRadiusM {
			jobs = append(jobs, VenueJob{Category: c, Center: centre})
			continue
		}
		for _, a := range areas {
			jobs = append(jobs, VenueJob{Category: c, Center: a.LatLng, From: a.ID})
		}
	}
	return jobs
}

// DiscoverVenues runs the plan. Each place is assigned to the area nearest to
// it, not the area whose search found it: a café 1.4 km from area A may be
// 300 m from area B. A place found by several searches or categories is kept
// once per category.
func DiscoverVenues(ctx context.Context, s maps.PlaceSearcher, areas []Area, cats []VenueCategory, language string) ([]Venue, error) {
	if len(areas) == 0 {
		return nil, nil
	}
	seen := map[[2]string]bool{}
	var out []Venue
	for _, j := range PlanVenues(areas, cats) {
		places, err := s.Nearby(ctx, maps.NearbyRequest{
			Center: j.Center, RadiusM: float64(j.Category.RadiusM), Types: j.Category.GoogleTypes, Language: language,
		})
		if err != nil {
			where := j.From
			if where == "" {
				where = "city centre"
			}
			return nil, fmt.Errorf("%s near %s: %w", j.Category.ID, where, err)
		}
		for _, p := range places {
			k := [2]string{p.ID, j.Category.ID}
			if p.ID == "" || seen[k] {
				continue
			}
			// The provider's primary-type filter isn't strict (Google returned
			// bars for "karaoke"), so check the primary type ourselves. Places
			// without one are skipped too: precision beats recall here.
			if !slices.Contains(j.Category.GoogleTypes, p.PrimaryType) {
				continue
			}
			seen[k] = true
			out = append(out, Venue{PlaceID: p.ID, AreaID: Nearest(areas, p.Location).ID, CategoryID: j.Category.ID})
		}
	}
	return out, nil
}

// Centre is the mean of the area centroids: good enough at city scale.
func Centre(areas []Area) maps.LatLng {
	var c maps.LatLng
	for _, a := range areas {
		c.Lat += a.Lat
		c.Lng += a.Lng
	}
	n := float64(max(len(areas), 1))
	return maps.LatLng{Lat: c.Lat / n, Lng: c.Lng / n}
}

// Nearest returns the area closest to p. areas must not be empty.
func Nearest(areas []Area, p maps.LatLng) Area {
	best, bestD := areas[0], maps.DistanceM(areas[0].LatLng, p)
	for _, a := range areas[1:] {
		if d := maps.DistanceM(a.LatLng, p); d < bestD {
			best, bestD = a, d
		}
	}
	return best
}
