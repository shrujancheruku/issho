// Package geo builds the data the matching engine needs about a city's
// geography: the area-to-area travel-time matrix and the venue list. It calls
// a maps provider through the maps interfaces and writes to the database.
package geo

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// MaxWalkM is the straight-line distance beyond which walking between two
// areas isn't requested. Nobody walks 5 km to an outing, and skipping those
// pairs cuts the walk matrix (a billed element per pair) to a fraction.
const MaxWalkM = 4000

// MaxCacheAge is how long travel times may be kept. Google's terms allow only
// temporary caching of Routes content; rows older than this are deleted, so the
// matrix must be rebuilt regularly (the M6 worker will schedule it).
const MaxCacheAge = 30 * 24 * time.Hour

type Area struct {
	ID string
	maps.LatLng
}

// MatrixJob is one provider call: an origin to its destinations in one mode.
type MatrixJob struct {
	Mode   maps.Mode
	Origin Area
	Dests  []Area
}

// TravelTime is one row of app.area_travel_times.
type TravelTime struct {
	From, To string
	Mode     maps.Mode
	Minutes  int32
}

// PlanMatrix lists the provider calls needed for a city. Same-area pairs are
// never requested: they're stored as 0 minutes. Walk only covers nearby areas.
// Each origin is its own job, which keeps jobs small and makes failures local.
func PlanMatrix(areas []Area, modes []maps.Mode) []MatrixJob {
	var jobs []MatrixJob
	for _, mode := range modes {
		for _, o := range areas {
			var dests []Area
			for _, d := range areas {
				if d.ID == o.ID {
					continue
				}
				if mode == maps.Walk && maps.DistanceM(o.LatLng, d.LatLng) > MaxWalkM {
					continue
				}
				dests = append(dests, d)
			}
			if len(dests) > 0 {
				jobs = append(jobs, MatrixJob{Mode: mode, Origin: o, Dests: dests})
			}
		}
	}
	return jobs
}

// Elements is the number of billed origin/destination pairs in the plan.
func Elements(jobs []MatrixJob) int {
	n := 0
	for _, j := range jobs {
		n += len(j.Dests)
	}
	return n
}

// BuildMatrix runs the plan and returns every travel time, including 0-minute
// same-area rows for each mode. Pairs with no route are left out, which the
// matching engine reads as "not reachable in this mode".
func BuildMatrix(ctx context.Context, r maps.Router, areas []Area, modes []maps.Mode, departure time.Time) ([]TravelTime, error) {
	var out []TravelTime
	for _, mode := range modes {
		for _, a := range areas {
			out = append(out, TravelTime{From: a.ID, To: a.ID, Mode: mode})
		}
	}
	for _, j := range PlanMatrix(areas, modes) {
		dests := make([]maps.LatLng, len(j.Dests))
		for i, d := range j.Dests {
			dests[i] = d.LatLng
		}
		els, err := r.Matrix(ctx, maps.MatrixRequest{
			Origins: []maps.LatLng{j.Origin.LatLng}, Destinations: dests, Mode: j.Mode, Departure: departure,
		})
		if err != nil {
			return nil, fmt.Errorf("%s from %s: %w", j.Mode, j.Origin.ID, err)
		}
		for _, e := range els {
			if !e.OK {
				continue
			}
			if e.Destination < 0 || e.Destination >= len(j.Dests) {
				return nil, fmt.Errorf("%s from %s: provider returned destination index %d", j.Mode, j.Origin.ID, e.Destination)
			}
			out = append(out, TravelTime{
				From: j.Origin.ID, To: j.Dests[e.Destination].ID, Mode: j.Mode,
				// Round up: "12 minutes" should never hide a 12.5-minute trip.
				Minutes: int32(math.Ceil(e.Duration.Minutes())),
			})
		}
	}
	return out, nil
}

// NextDeparture returns the next Saturday at 11:00 in loc, the typical start of
// a weekend outing. Transit times depend on the timetable, so the matrix is
// computed for when events actually happen rather than for "now".
func NextDeparture(now time.Time, loc *time.Location) time.Time {
	n := now.In(loc)
	days := (int(time.Saturday) - int(n.Weekday()) + 7) % 7
	d := time.Date(n.Year(), n.Month(), n.Day()+days, 11, 0, 0, 0, loc)
	if !d.After(n) {
		d = d.AddDate(0, 0, 7)
	}
	return d
}
