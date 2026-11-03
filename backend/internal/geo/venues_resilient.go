package geo

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/httpjson"
)

// DiscoverJobs runs jobs (a PlanVenues plan, perhaps with finished work
// removed by Searched.Skip) as DiscoverVenues does. When a search fails it
// returns the venues found so far along with the error, so the caller can
// save what was paid for.
func DiscoverJobs(ctx context.Context, s maps.PlaceSearcher, areas []Area, jobs []VenueJob, language string, budget int) ([]Venue, Searches, error) {
	if len(areas) == 0 {
		return nil, Searches{}, nil
	}
	return runVenueJobs(ctx, s, areas, jobs, language, budget)
}

// Searched is the set of area × category pairs already searched.
type Searched struct {
	pairs map[[2]string]bool // area id, category id
	cats  map[string]bool    // categories with a venue anywhere (for city-wide searches)
}

// LoadSearched reads the pairs that have venues discovered since the cut-off.
// A pair counts as searched when discovery found a venue in the area for the
// category: the database keeps no record of a search itself, and a search
// that found nothing is cheap to repeat. A neighbour's search can spill into
// the area, so this can skip an area whose own search never ran; rerun with
// a zero window to search everything.
func LoadSearched(ctx context.Context, q *dbgen.Queries, cityID string, since time.Time) (Searched, error) {
	rows, err := q.ListDiscoveredVenuePairs(ctx, dbgen.ListDiscoveredVenuePairsParams{CityID: cityID, Since: pgtype.Timestamptz{Time: since, Valid: true}})
	if err != nil {
		return Searched{}, err
	}
	s := Searched{pairs: map[[2]string]bool{}, cats: map[string]bool{}}
	for _, r := range rows {
		s.pairs[[2]string{r.AreaID, r.CategoryID}] = true
		s.cats[r.CategoryID] = true
	}
	return s, nil
}

// Skip returns the jobs not yet done, and how many it dropped. A city-wide
// search (no area) is done once its category has a venue anywhere.
func (s Searched) Skip(jobs []VenueJob) (todo []VenueJob, skipped int) {
	for _, j := range jobs {
		done := s.cats[j.Category.ID]
		if j.From != "" {
			done = s.pairs[[2]string{j.From, j.Category.ID}]
		}
		if done {
			skipped++
			continue
		}
		todo = append(todo, j)
	}
	return todo, skipped
}

// IsQuotaError reports whether err is Google refusing a request for quota
// (HTTP 429), and whether the limit is per minute. Google names the limit in
// the error ("... and limit 'SearchNearbyRequest per minute'"; the details
// carry "...PerMinutePerProject"). A per-minute limit clears by itself; any
// other quota 429 (per day, per month, or one that doesn't say) is treated as
// one that won't, so the run stops instead of waiting for nothing.
func IsQuotaError(err error) (quota, perMinute bool) {
	se, ok := httpjson.Status(err)
	if !ok || se.StatusCode != 429 {
		return false, false
	}
	b := strings.ToLower(se.Body)
	return true, strings.Contains(b, "per minute") || strings.Contains(b, "perminute") || strings.Contains(b, "per-minute")
}

// Pacer wraps a PlaceSearcher for a long discovery run. It spaces requests
// to at most PerMinute a minute, and if Google still answers with a
// per-minute quota 429 it waits QuotaWait and tries again, up to MaxWaits
// times in a row. Any other error, including a daily quota, goes straight
// back to the caller.
//
// Pacing comes first because it is predictable: the run takes searches ÷
// PerMinute minutes and never trips the limit, where backing off from 429s
// alone would hit the wall over and over, a minute lost each time. The
// 429 wait is the net for a limit lower than assumed or shared with another
// run. Safe for concurrent use, though discovery is sequential.
type Pacer struct {
	S         maps.PlaceSearcher
	PerMinute int           // 0 = unpaced
	QuotaWait time.Duration // wait after a per-minute 429
	MaxWaits  int           // consecutive waits before giving up
	Log       *slog.Logger

	mu   sync.Mutex
	next time.Time
	// Overridable for tests.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

var _ maps.PlaceSearcher = (*Pacer)(nil)

// NewPacer paces s at perMinute requests a minute (0 = no pacing) and waits
// 65 s, five times at most, when Google reports a per-minute quota.
func NewPacer(s maps.PlaceSearcher, perMinute int) *Pacer {
	return &Pacer{S: s, PerMinute: perMinute, QuotaWait: 65 * time.Second, MaxWaits: 5, Log: slog.Default()}
}

func (p *Pacer) Nearby(ctx context.Context, req maps.NearbyRequest) ([]maps.Place, error) {
	for waits := 0; ; waits++ {
		if err := p.pace(ctx); err != nil {
			return nil, err
		}
		places, err := p.S.Nearby(ctx, req)
		if err == nil {
			return places, nil
		}
		if _, perMinute := IsQuotaError(err); !perMinute || waits >= p.MaxWaits {
			return nil, err
		}
		if p.Log != nil {
			p.Log.Warn("per-minute quota reached; waiting", "wait", p.QuotaWait, "wait_number", waits+1, "of", p.MaxWaits)
		}
		if err := p.doSleep(ctx, p.QuotaWait); err != nil {
			return nil, fmt.Errorf("%w (while waiting out the quota)", err)
		}
	}
}

// pace sleeps until the next request slot and claims it.
func (p *Pacer) pace(ctx context.Context) error {
	if p.PerMinute <= 0 {
		return nil
	}
	gap := time.Minute / time.Duration(p.PerMinute)
	p.mu.Lock()
	now := time.Now()
	if p.now != nil {
		now = p.now()
	}
	at := now
	if p.next.After(at) {
		at = p.next
	}
	p.next = at.Add(gap)
	p.mu.Unlock()
	if d := at.Sub(now); d > 0 {
		return p.doSleep(ctx, d)
	}
	return nil
}

func (p *Pacer) doSleep(ctx context.Context, d time.Duration) error {
	if p.sleep != nil {
		return p.sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
