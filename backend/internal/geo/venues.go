package geo

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// WideRadiusM marks categories searched once per city instead of once per
// area. With a 30 km radius every area would return the same beaches and
// trails, paying for the same results over and over.
const WideRadiusM = 10000

// MaxRadiusM is the largest circle Nearby Search accepts.
const MaxRadiusM = 50000

// pageSize is the most places one Nearby Search returns (Google: 20, no
// further pages). A search that returns this many may have missed some.
const pageSize = 20

type VenueCategory struct {
	ID          string
	GoogleTypes []string
	RadiusM     int32
}

// VenueJob is one Nearby Search call.
type VenueJob struct {
	Category VenueCategory
	Center   maps.LatLng
	RadiusM  float64
	From     string // area id, or "" for a city-wide search
	// Within is the boundary of the area searched, if it has one. The search
	// then starts as one circle over the whole area and splits where it
	// comes back full (see split).
	Within maps.Boundary
}

// Venue is a discovered place assigned to the nearest area.
type Venue struct {
	PlaceID    string
	AreaID     string
	CategoryID string
}

// PlanVenues lists the first Nearby Search calls for a city: one per area
// and category, except wide categories, which get one search from the city
// centre. An area without a boundary is searched within the category's
// radius of its point; one with a boundary, over the whole of it, which
// adds searches as it goes (runVenueJobs).
func PlanVenues(areas []Area, cats []VenueCategory) []VenueJob {
	var jobs []VenueJob
	centre := Centre(areas)
	for _, c := range cats {
		if c.RadiusM >= WideRadiusM {
			jobs = append(jobs, VenueJob{Category: c, Center: centre, RadiusM: float64(c.RadiusM)})
			continue
		}
		for _, a := range areas {
			j := VenueJob{Category: c, Center: a.LatLng, RadiusM: float64(c.RadiusM), From: a.ID}
			if len(a.Boundary) > 0 {
				j.Center, j.RadiusM = a.Boundary.Circle()
				j.RadiusM = min(max(j.RadiusM, float64(c.RadiusM)), MaxRadiusM)
				j.Within = a.Boundary
			}
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// split replaces a search that came back full with seven smaller circles
// that together cover it (the centre and six around it at √3/2 of the
// radius), keeping only those that reach into the area. Half the radius
// covers it exactly, with no margin for rounding, so they get 55%. It
// stops once the circle is no bigger than the category's radius: within
// that, the most popular places are the ones we want, as for an area
// without a boundary. A sparse part of the area (hills, farmland) comes
// back with room to spare and costs no more searches.
func split(j VenueJob) []VenueJob {
	if j.Within == nil || j.RadiusM <= float64(j.Category.RadiusM) {
		return nil
	}
	r := j.RadiusM * 0.55
	centres := []maps.LatLng{j.Center}
	for k := range 6 {
		centres = append(centres, offset(j.Center, j.RadiusM*math.Sqrt(3)/2, float64(k)*math.Pi/3))
	}
	var out []VenueJob
	for _, c := range centres {
		if j.Within.DistanceM(c) <= r {
			k := j
			k.Center, k.RadiusM = c, r
			out = append(out, k)
		}
	}
	return out
}

// offset moves p by d metres on a bearing (radians from north), on the
// same sphere as maps.DistanceM.
func offset(p maps.LatLng, d, bearing float64) maps.LatLng {
	const mPerDegree = 6371000 * math.Pi / 180
	return maps.LatLng{
		Lat: p.Lat + d*math.Cos(bearing)/mPerDegree,
		Lng: p.Lng + d*math.Sin(bearing)/(mPerDegree*math.Cos(p.Lat*math.Pi/180)),
	}
}

// MaxSearches is how many searches jobs could take if every search inside
// a boundary came back full: the bound a dry run reports.
func MaxSearches(jobs []VenueJob) int {
	n := 0
	for len(jobs) > 0 {
		j := jobs[len(jobs)-1]
		jobs = append(jobs[:len(jobs)-1], split(j)...)
		n++
	}
	return n
}

// Searches counts a discovery run's Nearby Searches (each one billed).
type Searches struct {
	Made int
	// Skipped is how many finer searches the budget left out: parts of
	// bounded areas whose search came back full and weren't split.
	Skipped int
}

// DiscoverVenues runs the plan, making at most budget searches. Each place
// is assigned to the area it's in (Locate), not the area whose search found
// it: a café 1.4 km from area A may be 300 m from area B. A place found by
// several searches or categories is kept once per category.
//
// The worst case for bounded areas (every search full, all the way down) is
// far beyond what any run needs: for Da Nang's wards, over 250,000 searches,
// mostly over Hải Vân's 400 km² of hills. The budget makes the cost a choice,
// not a property of the data. Searches go coarse to fine across every area,
// so a run that hits it has covered every area, just less finely.
func DiscoverVenues(ctx context.Context, s maps.PlaceSearcher, areas []Area, cats []VenueCategory, language string, budget int) ([]Venue, Searches, error) {
	if len(areas) == 0 {
		return nil, Searches{}, nil
	}
	return runVenueJobs(ctx, s, areas, PlanVenues(areas, cats), language, budget)
}

// PlanAreaVenues lists the Nearby Search calls for one area added to a city
// (or moved): its own searches, and the city-wide ones only when wide is
// set (a new city has none yet).
func PlanAreaVenues(areas []Area, areaID string, cats []VenueCategory, wide bool) []VenueJob {
	var jobs []VenueJob
	for _, j := range PlanVenues(areas, cats) {
		if j.From == areaID || (j.From == "" && wide) {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// DiscoverAreaVenues runs PlanAreaVenues within budget, as DiscoverVenues.
// Places are still assigned among all the city's areas.
func DiscoverAreaVenues(ctx context.Context, s maps.PlaceSearcher, areas []Area, areaID string, cats []VenueCategory, wide bool, language string, budget int) ([]Venue, Searches, error) {
	return runVenueJobs(ctx, s, areas, PlanAreaVenues(areas, areaID, cats, wide), language, budget)
}

func runVenueJobs(ctx context.Context, s maps.PlaceSearcher, areas []Area, jobs []VenueJob, language string, budget int) ([]Venue, Searches, error) {
	var n Searches
	if len(jobs) > budget {
		return nil, n, fmt.Errorf("a budget of %d searches is less than the %d first ones", budget, len(jobs))
	}
	seen := map[[2]string]bool{}
	var out []Venue
	for len(jobs) > 0 {
		j := jobs[0]
		jobs = jobs[1:]
		places, err := s.Nearby(ctx, maps.NearbyRequest{
			Center: j.Center, RadiusM: j.RadiusM, Types: j.Category.GoogleTypes, Max: pageSize, Language: language,
		})
		n.Made++
		if err != nil {
			where := j.From
			if where == "" {
				where = "city centre"
			}
			return nil, n, fmt.Errorf("%s near %s: %w", j.Category.ID, where, err)
		}
		if len(places) >= pageSize {
			// All of a circle's children or none, so what is searched
			// finely is searched completely.
			if kids := split(j); n.Made+len(jobs)+len(kids) <= budget {
				jobs = append(jobs, kids...)
			} else {
				n.Skipped += len(kids)
			}
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
			a, inside := Locate(areas, p.Location)
			// A search over a bounded area reaches past the city's edge;
			// what it finds there isn't in any area, so it isn't the city's.
			if j.Within != nil && !inside {
				continue
			}
			seen[k] = true
			out = append(out, Venue{PlaceID: p.ID, AreaID: a.ID, CategoryID: j.Category.ID})
		}
	}
	return out, n, nil
}

// Locate returns the area p is in: the first whose boundary contains it,
// and true; otherwise the nearest area by its point, and false. Areas
// without boundaries are only ever found the second way.
func Locate(areas []Area, p maps.LatLng) (Area, bool) {
	for _, a := range areas {
		if a.Boundary.Contains(p) {
			return a, true
		}
	}
	return Nearest(areas, p), false
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
