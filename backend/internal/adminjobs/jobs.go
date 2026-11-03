// Package adminjobs holds the admin SPA's long operations as River jobs:
// what the API queues (the args), what the worker runs (ApplyCityPack and
// DiscoverAreaVenues), what the job records when it ends (Output) and how
// AdminService.GetAdminJob reads that back. A request can't outlast
// Cloudflare's 100 s cut-off, so the RPCs that start these only validate,
// insert the job and return its id; the SPA polls for the result.
//
// There is no table of our own: the job's state and times are River's, and
// its result or failure is the output the worker records on the job
// (river.RecordOutput), which River keeps in the job's metadata whether the
// attempt succeeded or not.
package adminjobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/db"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// Queue is where the admin jobs run: the worker's `maintenance` queue
// (worker.QueueMaintenance is this constant).
const Queue = "maintenance"

// LiveStates are the job states a unique job is counted in: one queued,
// running or waiting to retry blocks another of its kind (and args).
var LiveStates = []rivertype.JobState{
	rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
	rivertype.JobStateRetryable, rivertype.JobStateScheduled,
}

// Time limits and attempts, set explicitly as every kind does (River's
// defaults, a minute and 25 attempts, fit neither).
const (
	// ApplyCityPackTimeout: one transaction to seed the pack, then (for new
	// or moved areas) a travel-time matrix from Google, which takes minutes in
	// a big city (refresh_maps_city gets ten for the same rebuild).
	ApplyCityPackTimeout = 10 * time.Minute
	// ApplyCityPackMaxAttempts is 1: the seed is committed before Google is
	// asked, so a retry would find nothing new or moved and report an empty
	// apply over the real one. The admin applies again instead (it is
	// idempotent), and the worker's daily refresh rebuilds a failed matrix.
	ApplyCityPackMaxAttempts = 1

	// DiscoverAreaVenuesTimeout: up to cities.AreaSearchBudget Nearby Searches.
	DiscoverAreaVenuesTimeout = 10 * time.Minute
	// DiscoverAreaVenuesMaxAttempts: nothing is saved until every search has
	// answered, so a second attempt is safe, and one covers a blip.
	DiscoverAreaVenuesMaxAttempts = 2
)

// Kinds, as GetAdminJob recognises them.
const (
	KindApplyCityPack      = "apply_city_pack"
	KindDiscoverAreaVenues = "discover_area_venues"
)

// ApplyCityPackArgs applies a validated pack. The pack rides in the args
// because the worker has no GitHub client: the API read it from the default
// branch and checked it, and the job applies exactly that. Only the city
// counts for uniqueness (river:"unique"), so a second apply of a city while
// one is queued or running is skipped and gets the first job's id.
type ApplyCityPackArgs struct {
	City string      `json:"city" river:"unique"`
	Pack cities.Pack `json:"pack"`
}

func (ApplyCityPackArgs) Kind() string { return KindApplyCityPack }

func (ApplyCityPackArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Queue, MaxAttempts: ApplyCityPackMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: LiveStates}}
}

// DiscoverAreaVenuesArgs finds one area's venues. One search per area at a
// time: two would only pay Google twice for the same places.
type DiscoverAreaVenuesArgs struct {
	City string `json:"city"`
	Area string `json:"area"`
}

func (DiscoverAreaVenuesArgs) Kind() string { return KindDiscoverAreaVenues }

func (DiscoverAreaVenuesArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Queue, MaxAttempts: DiscoverAreaVenuesMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: LiveStates}}
}

// Output is what a job records: the result of the operation, or why it
// failed. One shape for both kinds, so GetAdminJob reads either.
type Output struct {
	ApplyCityPack      *cities.Diff `json:"apply_city_pack,omitempty"`
	DiscoverAreaVenues *VenuesFound `json:"discover_area_venues,omitempty"`
	Failure            *Failure     `json:"failure,omitempty"`
}

// VenuesFound is DiscoverAreaVenues' result.
type VenuesFound struct {
	Searches int `json:"searches"`
	Venues   int `json:"venues"`
}

// Failure is why a job failed, for the admin: a message safe to show (what
// was refused; that Google failed) and the reason a screen can branch on.
type Failure struct {
	Message string              `json:"message"`
	Reason  isshov1.ErrorReason `json:"reason,omitempty"`
}

// Messages for failures whose error text must stay in the worker's log.
const (
	msgInternal = "the job failed; the worker's log has the details"
	msgTimedOut = "ran past its time limit; try again"
	msgTravel   = "seeded, but Google's travel times failed (the worker retries within a day)"
	msgSearch   = "Google's venue search failed; try again shortly"
	msgNoPlaces = "finding venues needs GOOGLE_MAPS_API_KEY on the worker"
)

// classify turns a failed run into what is recorded and what the job
// returns: a failure a retry can't fix is cancelled (River stops retrying
// it), the rest is returned as is for River to retry or discard and Sentry to
// hear of.
func classify(err error) (*Failure, error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &Failure{Message: msgTimedOut, Reason: isshov1.ErrorReason_ERROR_REASON_TIMED_OUT}, err
	case errors.Is(err, cities.ErrRefused):
		// Names areas and counts only; nothing was written.
		return &Failure{Message: err.Error()}, river.JobCancel(err)
	case errors.Is(err, geo.ErrCityNotFound):
		return &Failure{Message: "no such city"}, river.JobCancel(err)
	case errors.Is(err, cities.ErrAreaNotInCity):
		return &Failure{Message: "area_id: not in that city"}, river.JobCancel(err)
	case errors.Is(err, cities.ErrTravelTimes):
		return &Failure{Message: msgTravel}, err
	case errors.Is(err, cities.ErrSearch):
		return &Failure{Message: msgSearch}, err
	}
	return &Failure{Message: msgInternal}, err
}

// ApplyCityPack applies the pack as one cities.Apply (a single transaction;
// the travel times follow it) and returns what to record and how the job
// ends. A nil router (no Maps key on the worker) skips the rebuild, as it
// does for the CLI seed.
func ApplyCityPack(ctx context.Context, pool *pgxpool.Pool, router maps.Router, a ApplyCityPackArgs) (Output, error) {
	// The admin never consents to losing curated venues; that is `tool seed
	// -drop-curated-venues`.
	d, err := cities.Apply(ctx, pool, router, a.Pack, cities.Options{})
	if err != nil {
		f, jobErr := classify(err)
		// The pack is seeded when only Google failed: say what changed too.
		if errors.Is(err, cities.ErrTravelTimes) {
			return Output{ApplyCityPack: &d, Failure: f}, jobErr
		}
		return Output{Failure: f}, jobErr
	}
	return Output{ApplyCityPack: &d}, nil
}

// DiscoverAreaVenues finds the area's venues (cities.DiscoverAreaVenues) and
// returns what to record and how the job ends.
func DiscoverAreaVenues(ctx context.Context, pool *pgxpool.Pool, places maps.PlaceSearcher, a DiscoverAreaVenuesArgs) (Output, error) {
	if places == nil {
		// The API checks its own key, but the worker has a key of its own.
		err := errors.New("no GOOGLE_MAPS_API_KEY on the worker")
		return Output{Failure: &Failure{Message: msgNoPlaces, Reason: isshov1.ErrorReason_ERROR_REASON_FEATURE_UNAVAILABLE}}, river.JobCancel(err)
	}
	searches, venues, err := cities.DiscoverAreaVenues(ctx, pool, places, a.City, a.Area)
	if err != nil {
		f, jobErr := classify(fmt.Errorf("area venue search, city %s area %s: %w", a.City, a.Area, err))
		return Output{Failure: f}, jobErr
	}
	return Output{DiscoverAreaVenues: &VenuesFound{Searches: searches, Venues: venues}}, nil
}

// NewClient is the API's River client: it only inserts jobs (no queues, no
// workers, never started). The pool needs no LISTEN/NOTIFY for that.
func NewClient(pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: db.RiverSchema})
}
